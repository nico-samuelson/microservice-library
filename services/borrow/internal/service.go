package internal

import (
	"context"
	"log"
	interfaces "shared/pkg/interface"
	"shared/pkg/model"
	"shared/pkg/repository"
	"shared/pkg/service"
	pb "shared/proto/buffer"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type BorrowServiceServer struct {
	pb.UnimplementedBorrowServiceServer
	Service          interfaces.ServiceInterface[model.Borrow, model.BorrowUpdateRequest]
	Cache            *redis.Client
	CollectionClient pb.CollectionServiceClient
	BookClient       pb.BookServiceClient
}

func NewBorrowService(database *mongo.Database, collection_name string, connections map[string]*grpc.ClientConn, redis *redis.Client) *BorrowServiceServer {
	repository := repository.NewRepository[model.Borrow](database, collection_name)
	return &BorrowServiceServer{
		Service:          service.NewBaseService[model.Borrow, model.BorrowUpdateRequest](repository),
		Cache:            redis,
		CollectionClient: pb.NewCollectionServiceClient(connections["collection"]),
		BookClient:       pb.NewBookServiceClient(connections["book"]),
	}
}

func (s *BorrowServiceServer) BorrowBook(ctx context.Context, in *pb.BorrowRequest) (*pb.BorrowServiceResponse, error) {
	// Fetch book and collection info
	book, err := s.fetchBookAndCollection(ctx, in.CollectionId)
	if err != nil {
		return nil, err
	}

	// Create borrow record with compensation pattern
	borrow, err := s.createBorrowWithCompensation(ctx, book, in.CollectionId)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.updateCache(ctx, book.Id.Hex(), in.CollectionId, "remove")

	return s.buildResponse(true, "Book borrowed!", borrow.Id.Hex(), borrow.BookId.Hex()), nil
}

func (s *BorrowServiceServer) ReturnBook(ctx context.Context, in *pb.ReturnRequest) (*pb.BorrowServiceResponse, error) {
	now := time.Now().UTC()

	// Check if book already returned
	borrowRecord, err := s.Service.FindById(ctx, in.BorrowId)
	if err == mongo.ErrNoDocuments {
		log.Printf("error checking book status when returning: %v", err)
		return nil, status.Error(codes.NotFound, "Borrow record not found")
	} else if borrowRecord != nil {
		if borrowRecord.ReturnDate != nil && !borrowRecord.ReturnDate.IsZero() {
			log.Printf("Borrow already returned: %v", borrowRecord.Id.Hex())
			return nil, status.Error(codes.FailedPrecondition, "Book already returned")
		}
	}

	if err := s.markBookBorrowedStatus(ctx, borrowRecord.BookId.Hex(), false, now); err != nil {
		return nil, status.Errorf(codes.Aborted, "failed to mark book as returned: %v", err)
	}

	// Update borrow record
	_, err = s.Service.Update(ctx, map[string]interface{}{
		"return_date": now.Format(time.RFC3339),
		"updated_at":  now.Format(time.RFC3339),
	}, in.BorrowId)

	if err != nil {
		s.markBookBorrowedStatus(ctx, borrowRecord.BookId.Hex(), true, now)
		return nil, status.Errorf(codes.Internal, "failed to update borrow record: %v", err)
	}

	// Update cache
	s.updateCache(ctx, borrowRecord.BookId.Hex(), borrowRecord.CollectionId.Hex(), "put")

	return s.buildResponse(true, "Book returned successfully", borrowRecord.Id.Hex(), borrowRecord.BookId.Hex()), nil
}

func (s *BorrowServiceServer) fetchBookAndCollection(ctx context.Context, collectionId string) (*model.Book, error) {
	var wg sync.WaitGroup
	var book *model.Book
	var collectionErr, bookErr error

	wg.Add(2)
	go func() {
		defer wg.Done()

		_, err := s.getCollection(ctx, collectionId)
		if err != nil {
			collectionErr = err
		}
	}()

	go func() {
		defer wg.Done()

		book_resp, err := s.getBook(ctx, collectionId)
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
	if bookErr != nil {
		return nil, status.Error(status.Code(bookErr), bookErr.Error())
	}

	return book, nil
}

func (s *BorrowServiceServer) getCollection(ctx context.Context, collectionId string) (*model.Collection, error) {
	response, err := s.CollectionClient.FindCollectionById(ctx, &pb.FindCollectionRequest{Id: collectionId})
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

func (s *BorrowServiceServer) getBook(ctx context.Context, collectionId string) (*model.Book, error) {
	// Try to get an available book first
	bookResponse, err := s.BookClient.GetAvailableBook(ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId})
	if err != nil {
		return nil, err
	}

	books := model.FromPbBooks(bookResponse.Book)
	if len(books) > 0 {
		// Reserve book so it doesn't get picked up by another concurrent request
		s.updateCache(ctx, books[0].Id.Hex(), collectionId, "remove")
		return books[0], nil
	}

	return nil, status.Error(codes.Internal, "Unknown error")
}

func (s *BorrowServiceServer) createBorrowWithCompensation(ctx context.Context, book *model.Book, collectionId string) (*model.Borrow, error) {
	now := time.Now()
	due := now.AddDate(0, 0, 7)

	collection_id, err := primitive.ObjectIDFromHex(collectionId)
	if err != nil {
		return nil, err
	}

	needsBookUpdate := !book.IsBorrowed // If book wasn't already borrowed, we need to mark it

	if needsBookUpdate {
		if err := s.markBookBorrowedStatus(ctx, book.Id.Hex(), true, now); err != nil {
			return nil, err
		}
	}

	newBorrow := &model.Borrow{
		Id:           primitive.NewObjectID(),
		BookId:       book.Id,
		UserId:       primitive.NewObjectID(), // TODO: use real user ID
		CollectionId: collection_id,
		BorrowDate:   now,
		DueDate:      &due,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.Service.Create(ctx, *newBorrow); err != nil {
		// Mark book as not borrowed on failure
		s.markBookBorrowedStatus(ctx, book.Id.Hex(), false, now)
		s.updateCache(ctx, book.Id.Hex(), collectionId, "put")
		return nil, status.Errorf(codes.Internal, "failed to create borrow record: %v", err)
	}

	return newBorrow, nil
}

func (s *BorrowServiceServer) markBookBorrowedStatus(ctx context.Context, bookId string, borrowed bool, timestamp time.Time) error {
	_, err := s.BookClient.UpdateBook(ctx, &pb.UpdateBookRequest{
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

func (s *BorrowServiceServer) buildResponse(success bool, message string, borrowId string, bookId string) *pb.BorrowServiceResponse {
	return &pb.BorrowServiceResponse{
		Id:      borrowId,
		BookId:  bookId,
		Success: success,
		Message: message,
	}
}

func (s *BorrowServiceServer) updateCache(ctx context.Context, bookId string, collectionId string, action string) {
	cacheKey := "available_books:" + collectionId

	// Check key existence
	existInCache, err := s.Cache.Exists(ctx, cacheKey).Result()
	if err != nil {
		log.Printf("Error checking key existence: %v", err)
		s.Cache.Del(ctx, cacheKey)
	}

	if existInCache > 0 {
		switch action {
		case "put":
			err = s.Cache.SAdd(ctx, cacheKey, bookId, time.Hour).Err()
			if err != nil {
				s.Cache.Del(ctx, cacheKey)
			}
		case "remove":
			err := s.Cache.SRem(ctx, cacheKey, bookId).Err()
			if err != nil {
				s.Cache.Del(ctx, cacheKey)
			}
		}
	} else if action == "put" {
		err = s.Cache.SAdd(ctx, cacheKey, bookId, time.Hour).Err()
		if err != nil {
			s.Cache.Del(ctx, cacheKey)
		}
	}
}
