package internal

import (
	"context"
	"time"

	"shared/pkg/model"
	"shared/pkg/service"
	pb "shared/proto/buffer"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BookServiceServer struct {
	pb.UnimplementedBookServiceServer
	service *service.BaseService[model.Book, model.BookUpdateRequest]
}

func NewBookService(database *mongo.Database, collection_name string) *BookServiceServer {
	repository := NewBookRepository(database, collection_name)
	return &BookServiceServer{
		service: service.NewBaseService[model.Book, model.BookUpdateRequest](&repository.Repository),
	}
}

func (s *BookServiceServer) GetBook(ctx context.Context, in *pb.GetBookRequest) (*pb.BookResponse, error) {
	data, err := s.service.List(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	books := model.ToPbBooks(data)
	return s.buildResponse(true, "Books retrieved successfully", books), nil
}

func (s *BookServiceServer) FindBookById(ctx context.Context, in *pb.FindBookRequest) (*pb.BookResponse, error) {
	data, err := s.service.Find(ctx, bson.M{"_id": in.Id})

	if err == mongo.ErrNoDocuments {
		return s.buildResponse(false, "Book not found", nil), nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	newBook := model.ToPbBook(data)
	return s.buildResponse(true, "Book found", []*pb.Book{newBook}), nil
}

func (s *BookServiceServer) AddBook(ctx context.Context, in *pb.AddBookRequest) (*pb.BookResponse, error) {
	currTime := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	in.Book.Id = primitive.NewObjectID().Hex()
	in.Book.CreatedAt = currTime
	in.Book.UpdatedAt = currTime

	Book := model.FromPbBook(in.Book)
	err := s.service.Create(ctx, *Book)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.buildResponse(true, "Book added!", []*pb.Book{in.Book}), nil
}

func (s *BookServiceServer) UpdateBook(ctx context.Context, in *pb.UpdateBookRequest) (*pb.BookResponse, error) {
	update := in.Payload.AsMap()
	update["updated_at"] = time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	if collectionId, ok := update["collection_id"]; ok {
		collectionId, err := primitive.ObjectIDFromHex(collectionId.(string))
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		update["collection_id"] = collectionId
	}
	delete(update, "id")

	data, err := s.service.Update(ctx, update, in.Id)

	if err == mongo.ErrNoDocuments {
		reply := s.buildResponse(false, "Book not found", nil)
		return reply, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	dataPb := model.ToPbBook(&data)
	if dataPb == nil {
		return nil, status.Error(codes.Internal, "Failed to convert Book to protobuf")
	}
	return s.buildResponse(true, "Book updated!", []*pb.Book{dataPb}), nil
}

func (s *BookServiceServer) DeleteBook(ctx context.Context, in *pb.DeleteBookRequest) (*pb.BookResponse, error) {
	data, err := s.service.Delete(ctx, in.Id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Book not found", nil), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	newBook := model.ToPbBook(&data)
	return s.buildResponse(true, "Book deleted!", []*pb.Book{newBook}), nil
}

func (s *BookServiceServer) GetAvailableBook(ctx context.Context, in *pb.GetAvailableBookRequest) (*pb.BookResponse, error) {
	data, err := s.service.Find(ctx, bson.M{
		"collection_id": in.CollectionId,
		"is_borrowed":   false,
	})

	if err == mongo.ErrNoDocuments {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	book := model.ToPbBook(data)
	return s.buildResponse(true, "Books retrieved successfully", []*pb.Book{book}), nil
}

func (s *BookServiceServer) buildResponse(success bool, message string, collections []*pb.Book) *pb.BookResponse {
	return &pb.BookResponse{
		Success: success,
		Book:    collections,
		Message: message,
	}
}
