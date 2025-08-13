package internal

import (
	"context"
	"log"
	"shared/pkg/model"
	"shared/pkg/service"
	pb "shared/proto/buffer"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type BorrowServiceServer struct {
	pb.UnimplementedBorrowServiceServer
	service          *service.BaseService[model.Borrow, model.BorrowUpdateRequest]
	collectionClient pb.CollectionServiceClient
	bookClient       pb.BookServiceClient
}

func NewBorrowService(database *mongo.Database, collection_name string, connections map[string]*grpc.ClientConn) *BorrowServiceServer {
	repository := NewBorrowRepository(database, collection_name)
	return &BorrowServiceServer{
		service:          service.NewBaseService[model.Borrow, model.BorrowUpdateRequest](&repository.Repository),
		collectionClient: pb.NewCollectionServiceClient(connections["collection"]),
		bookClient:       pb.NewBookServiceClient(connections["book"]),
	}
}

func (s *BorrowServiceServer) BorrowBook(ctx context.Context, in *pb.BorrowRequest) (*pb.BorrowServiceResponse, error) {
	var wg sync.WaitGroup
	var bookErr, collectionErr error
	var collection *model.Collection
	var book *model.Book

	wg.Add(2)

	// Fetch collection and book info concurrently
	go func() {
		defer wg.Done()

		collection_resp, err := s.getCollection(ctx, in.CollectionId)
		if err != nil {
			collectionErr = err
		} else {
			if collection_resp.AvailableBooks < 1 {
				collectionErr = status.Error(codes.NotFound, "No available books in this collection")
			} else {
				collection = collection_resp
			}
		}
	}()

	go func() {
		defer wg.Done()

		book_resp, err := s.getOrCreateBook(ctx, in.CollectionId)
		if err != nil {
			bookErr = err
		} else {
			book = book_resp
		}
	}()

	wg.Wait()

	// Check for any error
	if collectionErr != nil {
		return nil, status.Error(status.Code(collectionErr), collectionErr.Error())
	}
	if bookErr != nil && status.Code(bookErr) != codes.NotFound {
		return nil, status.Error(status.Code(bookErr), bookErr.Error())
	}

	// Create borrow record with compensation pattern
	borrow, err := s.createBorrowWithCompensation(ctx, book, collection)
	if err != nil {
		return nil, err
	}

	return s.buildResponse(true, "Book borrowed!", []*pb.Borrow{model.ToPbBorrow(borrow)}), nil
}

func (s *BorrowServiceServer) ReturnBook(ctx context.Context, in *pb.ReturnRequest) (*pb.BorrowServiceResponse, error) {
	now := time.Now().UTC()
	borrow, err := s.service.Update(ctx, map[string]interface{}{
		"return_date": now.Format("2006-01-02T15:04:05.000000Z"),
		"updated_at":  now.Format("2006-01-02T15:04:05.000000Z"),
	}, in.BorrowId)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update borrow record: %v", err)
	}

	// Update Book
	if err := s.markBookBorrowedStatus(ctx, borrow.BookId.Hex(), false, now); err != nil {
		s.compensateBook(ctx, borrow.BookId.Hex(), false, true, true, now)
		return nil, status.Errorf(codes.Aborted, "failed to mark book as returned: %v", err)
	}

	// Update collection stock
	if _, err := s.collectionClient.DecrementAvailableBooks(ctx, &pb.DecrementAvailableBooksRequest{
		Id:     borrow.CollectionId.Hex(),
		Amount: 1,
	}); err != nil {
		// Compensate book state
		s.compensateBook(ctx, borrow.BookId.Hex(), false, true, true, now)
		return nil, status.Errorf(codes.Aborted, "failed to increment stock atomically: %v", err)
	}

	return s.buildResponse(true, "Book returned successfully", []*pb.Borrow{model.ToPbBorrow(&borrow)}), nil
}

func (s *BorrowServiceServer) getCollection(ctx context.Context, collectionId string) (*model.Collection, error) {
	response, err := s.collectionClient.FindCollectionById(ctx, &pb.FindCollectionRequest{Id: collectionId})
	if status.Code(err) == codes.NotFound {
		return nil, status.Error(codes.NotFound, "Collection not found")
	}
	if err != nil {
		log.Printf("Error retrieving collection: %v", err)
		return nil, status.Error(codes.Internal, "Error retrieving collection info")
	}

	collections := model.FromPbCollections(response.Collection)
	if len(collections) == 0 {
		return nil, status.Error(codes.Internal, "Invalid collection response")
	}

	return collections[0], nil
}

func (s *BorrowServiceServer) getOrCreateBook(ctx context.Context, collectionId string) (*model.Book, error) {
	// Try to get an available book first
	bookResponse, err := s.bookClient.GetAvailableBook(ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId})
	if err == nil {
		books := model.FromPbBooks(bookResponse.Book)
		if len(books) > 0 {
			return books[0], nil
		}
	}

	// If no available book found and not a "not found" error, return error
	if status.Code(err) != codes.NotFound {
		log.Printf("Error retrieving books: %v", err)
		return nil, status.Error(codes.Internal, "Error retrieving books")
	}

	// Create new book if none available
	return s.createNewBook(ctx, collectionId)
}

func (s *BorrowServiceServer) createNewBook(ctx context.Context, collectionId string) (*model.Book, error) {
	now := time.Now()
	addResp, err := s.bookClient.AddBook(ctx, &pb.AddBookRequest{
		Book: &pb.Book{
			Id:           primitive.NewObjectID().Hex(),
			CollectionId: collectionId,
			IsBorrowed:   &wrapperspb.BoolValue{Value: true},
			CreatedAt:    now.UTC().Format(time.RFC3339Nano),
			UpdatedAt:    now.UTC().Format(time.RFC3339Nano),
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create new book: %v", err)
	}

	books := model.FromPbBooks(addResp.Book)
	if len(books) == 0 {
		return nil, status.Error(codes.Internal, "created book but response was empty")
	}

	return books[0], nil
}

func (s *BorrowServiceServer) createBorrowWithCompensation(ctx context.Context, book *model.Book, collection *model.Collection) (*model.Borrow, error) {
	now := time.Now()
	due := now.AddDate(0, 0, 7)

	// Track if we need to update existing book vs created new one
	needsBookUpdate := !book.IsBorrowed                       // If book wasn't already borrowed, we need to mark it
	wasNewBook := book.CreatedAt.After(now.Add(-time.Minute)) // Heuristic: if created recently, it's new

	// Mark existing book as borrowed if needed
	if needsBookUpdate {
		if err := s.markBookBorrowedStatus(ctx, book.Id.Hex(), true, now); err != nil {
			return nil, err
		}
	}

	// Atomic stock decrement
	if _, err := s.collectionClient.DecrementAvailableBooks(ctx, &pb.DecrementAvailableBooksRequest{
		Id:     collection.Id.Hex(),
		Amount: -1,
	}); err != nil {
		// Compensate book state
		s.compensateBook(ctx, book.Id.Hex(), wasNewBook, needsBookUpdate, false, now)
		return nil, status.Errorf(codes.Aborted, "failed to decrement stock atomically: %v", err)
	}

	// Create borrow record
	newBorrow := &model.Borrow{
		Id:           primitive.NewObjectID(),
		BookId:       book.Id,
		UserId:       primitive.NewObjectID(), // TODO: use real user ID
		CollectionId: collection.Id,
		BorrowDate:   now,
		DueDate:      &due,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.service.Create(ctx, *newBorrow); err != nil {
		// Compensate both stock and book state
		s.compensateStock(ctx, collection.Id.Hex())
		s.compensateBook(ctx, book.Id.Hex(), wasNewBook, needsBookUpdate, false, now)
		return nil, status.Errorf(codes.Internal, "failed to create borrow record: %v", err)
	}

	log.Printf("Book borrowed successfully: %v", newBorrow.Id.Hex())
	return newBorrow, nil
}

func (s *BorrowServiceServer) markBookBorrowedStatus(ctx context.Context, bookId string, borrowed bool, timestamp time.Time) error {
	_, err := s.bookClient.UpdateBook(ctx, &pb.UpdateBookRequest{
		Id: bookId,
		Payload: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"is_borrowed": structpb.NewBoolValue(borrowed),
				"updated_at":  structpb.NewStringValue(timestamp.UTC().Format(time.RFC3339Nano)),
			},
		},
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to mark book as borrowed: %v", err)
	}
	return nil
}

func (s *BorrowServiceServer) compensateBook(ctx context.Context, bookId string, wasNewBook, needsUnmark bool, borrowed bool, timestamp time.Time) {
	if wasNewBook {
		// Delete the newly created book
		_, _ = s.bookClient.DeleteBook(ctx, &pb.DeleteBookRequest{Id: bookId})
	} else if needsUnmark {
		// Unmark existing book as borrowed
		_, _ = s.bookClient.UpdateBook(ctx, &pb.UpdateBookRequest{
			Id: bookId,
			Payload: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"is_borrowed": structpb.NewBoolValue(borrowed),
					"updated_at":  structpb.NewStringValue(timestamp.UTC().Format(time.RFC3339Nano)),
				},
			},
		})
	}
}

func (s *BorrowServiceServer) compensateStock(ctx context.Context, collectionId string) {
	_, _ = s.collectionClient.DecrementAvailableBooks(ctx, &pb.DecrementAvailableBooksRequest{
		Id:     collectionId,
		Amount: 1, // increment back
	})
}

func (s *BorrowServiceServer) buildResponse(success bool, message string, borrow []*pb.Borrow) *pb.BorrowServiceResponse {
	return &pb.BorrowServiceResponse{
		Success: success,
		Borrow:  borrow,
		Message: message,
	}
}
