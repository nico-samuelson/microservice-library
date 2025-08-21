package internal

import (
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"
	"time"

	interfaces "shared/pkg/interface"
	"shared/pkg/model"
	"shared/pkg/repository"
	"shared/pkg/service"
	"shared/pkg/utils"
	pb "shared/proto/buffer"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BookServiceServer struct {
	pb.UnimplementedBookServiceServer
	Service          interfaces.ServiceInterface[model.Book, model.BookUpdateRequest]
	Cache            *redis.Client
	CollectionClient pb.CollectionServiceClient
}

func NewBookService(database *mongo.Database, collection_name string, connections map[string]*grpc.ClientConn, cache *redis.Client) *BookServiceServer {
	repository := repository.NewRepository[model.Book](database, collection_name)
	return &BookServiceServer{
		Service:          service.NewBaseService[model.Book, model.BookUpdateRequest](repository),
		Cache:            cache,
		CollectionClient: pb.NewCollectionServiceClient(connections["collection"]),
	}
}

func (s *BookServiceServer) GetBook(ctx context.Context, in *pb.GetBookRequest) (*pb.BookResponse, error) {
	// Parse filter and sort from protobuf
	var filter bson.M
	var sort bson.D

	if len(in.Filter.Fields) > 0 {
		filterMap := in.Filter.AsMap()
		filter = bson.M{}
		for k, v := range filterMap {
			filter[k] = v
		}
	} else {
		filter = bson.M{}
	}

	if len(in.Sort) > 0 {
		sort = bson.D{}
		for _, sortItem := range in.Sort {
			sort = append(sort, bson.E{Key: sortItem.Key, Value: sortItem.Direction})
		}
	} else {
		sort = bson.D{}
	}

	data, err := s.Service.List(ctx, filter, sort, int(in.Skip), int(in.Limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	books := model.ToPbBooks(data)
	return s.buildResponse(true, "Books retrieved successfully", books), nil
}

func (s *BookServiceServer) FindBookById(ctx context.Context, in *pb.FindBookRequest) (*pb.BookResponse, error) {
	book, success := s.getCachedBook(ctx, in.Id)

	if !success {
		data, err := s.Service.Find(ctx, bson.M{"_id": in.Id})

		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Book not found", nil), nil
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		book = data

		// Set cache
		bytes, err := json.Marshal(book)
		if err != nil {
			log.Printf("Error packing JSON: %s", err)
		} else {
			err = s.Cache.Set(ctx, "book:"+in.Id, bytes, time.Hour).Err()
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
	}

	pbBook := model.ToPbBook(book)
	return s.buildResponse(true, "Book found", []*pb.Book{pbBook}), nil
}

func (s *BookServiceServer) AddBook(ctx context.Context, in *pb.AddBookRequest) (*pb.BookResponse, error) {
	currTime := time.Now().UTC().Format(time.RFC3339)
	in.Book.Id = primitive.NewObjectID().Hex()
	in.Book.CreatedAt = currTime
	in.Book.UpdatedAt = currTime

	Book := model.FromPbBook(in.Book)
	err := s.Service.Create(ctx, *Book)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	backgroundCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	go func() {
		defer cancel()

		retries := 0
		for retries < 3 {
			if _, err := s.CollectionClient.DecrementAvailableBooks(backgroundCtx, &pb.DecrementAvailableBooksRequest{
				Id:     in.Book.CollectionId,
				Amount: 1,
			}); err != nil {
				log.Printf("Failed to update collection stock: %v", err)
				retries += 1
			} else {
				break
			}
		}
	}()

	return s.buildResponse(true, "Book added!", []*pb.Book{in.Book}), nil
}

func (s *BookServiceServer) UpdateBook(ctx context.Context, in *pb.UpdateBookRequest) (*pb.BookResponse, error) {
	update := in.Payload.AsMap()
	update["updated_at"] = time.Now().UTC().Format(time.RFC3339)

	if collectionId, ok := update["collection_id"]; ok {
		collectionId, err := primitive.ObjectIDFromHex(collectionId.(string))
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		update["collection_id"] = collectionId
	}
	delete(update, "id")

	data, err := s.Service.Update(ctx, update, in.Id)

	if err == mongo.ErrNoDocuments {
		reply := s.buildResponse(false, "Book not found", nil)
		return reply, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.invalidateCache(ctx, in.Id)

	dataPb := model.ToPbBook(&data)
	if dataPb == nil {
		return nil, status.Error(codes.Internal, "Failed to convert Book to protobuf")
	}
	return s.buildResponse(true, "Book updated!", []*pb.Book{dataPb}), nil
}

func (s *BookServiceServer) DeleteBook(ctx context.Context, in *pb.DeleteBookRequest) (*pb.BookResponse, error) {
	data, err := s.Service.Delete(ctx, in.Id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Book not found", nil), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.invalidateCache(ctx, in.Id)

	backgroundCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	go func() {
		defer cancel()

		retries := 0
		for retries < 3 {
			if _, err := s.CollectionClient.DecrementAvailableBooks(backgroundCtx, &pb.DecrementAvailableBooksRequest{
				Id:     data.CollectionId.Hex(),
				Amount: -1,
			}); err != nil {
				log.Printf("Failed to update collection stock: %v", err)
				retries += 1
			} else {
				break
			}
		}
	}()

	newBook := model.ToPbBook(&data)
	return s.buildResponse(true, "Book deleted!", []*pb.Book{newBook}), nil
}

func (s *BookServiceServer) GetAvailableBook(ctx context.Context, in *pb.GetAvailableBookRequest) (*pb.BookResponse, error) {
	// First check for cache
	book, success := s.getCachedAvailableBook(ctx, in.CollectionId)

	if !success {
		collectionId, err := primitive.ObjectIDFromHex(in.CollectionId)
		if err != nil {
			log.Printf("Error converting collection ID: %v", in.CollectionId)
			return nil, status.Error(codes.Internal, err.Error())
		}

		data, err := s.Service.Find(ctx, bson.M{
			"collection_id": collectionId,
			"is_borrowed":   false,
		})

		if err == mongo.ErrNoDocuments {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		book = data

		// Set cache
		err = s.Cache.SAdd(ctx, "available_books:"+in.CollectionId, book.Id.Hex(), time.Hour).Err()
		if err != nil {
			log.Printf("Error setting cache: %v", err)
		}
	}

	pbBook := model.ToPbBook(book)
	return s.buildResponse(true, "Books retrieved successfully", []*pb.Book{pbBook}), nil
}

func (s *BookServiceServer) CountBook(ctx context.Context, in *pb.CountBookRequest) (*pb.BookCountResponse, error) {
	// Check cache first
	if count, found := utils.GetCachedData[int64](ctx, s.Cache, in.CollectionId); found {
		return &pb.BookCountResponse{
			Count:   *count,
			Success: true,
			Message: "Book counted successfully!",
		}, nil
	}

	// Compute from books
	collectionObjId, _ := primitive.ObjectIDFromHex(in.CollectionId)
	count, err := s.Service.Count(ctx, bson.M{
		"collection_id": collectionObjId,
	})

	if err != nil {
		return nil, err
	}

	// Cache result
	s.Cache.Set(ctx, "available_count:"+in.CollectionId, int(count), time.Hour)
	return &pb.BookCountResponse{
		Count:   count,
		Success: true,
		Message: "Book counted successfully!",
	}, nil
}

func (s *BookServiceServer) BulkInsert(ctx context.Context, in *pb.BulkInsertBookRequest) (*pb.BookResponse, error) {
	// log.Println(in.Books[0])
	// for _, book := range in.Books {
	// 	currTime := time.Now().UTC().Format(time.RFC3339)
	// 	book.Id = primitive.NewObjectID().Hex()
	// 	book.CreatedAt = currTime
	// 	book.UpdatedAt = currTime
	// }

	// log.Println(in.Books[0].CollectionId, in.Books[0].IsBorrowed)

	booksPtr := model.FromPbBooks(in.Books)
	books := make([]model.Book, len(booksPtr))
	for i, b := range booksPtr {
		books[i] = *b
	}

	err := s.Service.BulkInsert(ctx, books)
	if err != nil {
		log.Printf("error bulk insert: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.buildResponse(true, "Book added!", in.Books), nil
}

func (s *BookServiceServer) buildResponse(success bool, message string, collections []*pb.Book) *pb.BookResponse {
	return &pb.BookResponse{
		Success: success,
		Book:    collections,
		Message: message,
	}
}

func (s *BookServiceServer) getCachedAvailableBook(ctx context.Context, collectionId string) (*model.Book, bool) {
	books, err := s.Cache.SMembers(ctx, "available_books:"+collectionId).Result()

	if err != nil {
		return nil, false
	}

	if len(books) == 0 {
		return nil, false
	}

	bookId, err := primitive.ObjectIDFromHex(books[rand.IntN(len(books))])
	if err != nil {
		log.Printf("Error converting book id to object id: %v", err)
		return nil, false
	}

	collectionIdObj, err := primitive.ObjectIDFromHex(collectionId)
	if err != nil {
		log.Printf("Error converting collection id to object id: %v", err)
		return nil, false
	}

	return &model.Book{
		Id:           bookId,
		CollectionId: collectionIdObj,
		IsBorrowed:   false,
	}, true
}

func (s *BookServiceServer) getCachedBook(ctx context.Context, id string) (*model.Book, bool) {
	cachedBook, success := utils.GetCachedData[model.Book](ctx, s.Cache, "book:"+id)

	if !success {
		return nil, false
	}

	return cachedBook, true
}

func (s *BookServiceServer) invalidateCache(ctx context.Context, id string) {
	// Invalidate cache
	err := s.Cache.Del(ctx, "book:"+id).Err()
	if err != nil {
		log.Printf("Error deleting cache: %v", err)
	}
}
