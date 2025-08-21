package test

import (
	"borrow/internal"
	"borrow/test/mocks"
	"context"
	"testing"
	"time"

	"shared/pkg/model"
	pb "shared/proto/buffer"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func newRedis(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newServer(cache *redis.Client) (*mocks.MockService[model.Borrow, model.BorrowUpdateRequest], *internal.BorrowServiceServer) {
	mockService := &mocks.MockService[model.Borrow, model.BorrowUpdateRequest]{}

	svc := &internal.BorrowServiceServer{
		Service:          mockService,
		Cache:            cache,
		CollectionClient: mocks.NewMockCollectionService(cache),
		BookClient:       mocks.NewMockBookService(cache),
	}

	return mockService, svc
}

func ArrangeBorrowData() (primitive.ObjectID, primitive.ObjectID, *pb.Collection, *pb.Book, time.Time) {
	collectionId := primitive.NewObjectID()
	bookId := primitive.NewObjectID()
	now := time.Now().UTC()

	collection := pb.Collection{
		Id:        collectionId.Hex(),
		Name:      "Harry Potter",
		Author:    "J. K. Rowling",
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	}
	book := pb.Book{
		Id:           bookId.Hex(),
		CollectionId: collectionId.Hex(),
		IsBorrowed:   &wrapperspb.BoolValue{Value: false},
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
	}

	return collectionId, bookId, &collection, &book, now
}

func ArrangeReturnData() (primitive.ObjectID, primitive.ObjectID, primitive.ObjectID, *pb.Book, *model.Borrow, time.Time) {
	collectionId := primitive.NewObjectID()
	bookId := primitive.NewObjectID()
	borrowId := primitive.NewObjectID()
	now := time.Now().UTC()

	book := pb.Book{
		Id:           bookId.Hex(),
		CollectionId: collectionId.Hex(),
		IsBorrowed:   &wrapperspb.BoolValue{Value: false},
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
	}
	borrowRecord := model.Borrow{
		Id:           borrowId,
		CollectionId: collectionId,
		BookId:       bookId,
		BorrowDate:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
		ReturnDate:   nil,
	}

	return collectionId, bookId, borrowId, &book, &borrowRecord, now
}

func TestBorrow_Success(t *testing.T) {
	// Arrange
	cache := newRedis(t)
	_, mockService := newServer(cache)
	collectionId, bookId, collection, book, _ := ArrangeBorrowData()
	ctx := context.Background()

	mockService.CollectionClient.(*mocks.MockCollectionService).On("FindCollectionById", ctx, &pb.FindCollectionRequest{Id: collectionId.Hex()}).Return(&pb.Response{Collection: []*pb.Collection{collection}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("GetAvailableBook", ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId.Hex()}).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id &&
			req.Payload.Fields["is_borrowed"].GetBoolValue() == true &&
			req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("Create", ctx, mock.MatchedBy(func(req model.Borrow) bool {
		return req.BookId.Hex() == book.Id && req.CollectionId.Hex() == collection.Id
	})).Return(nil)

	// Act
	cache.SAdd(ctx, "available_books:"+collectionId.Hex(), bookId.Hex(), time.Hour)
	resp, err := mockService.BorrowBook(ctx, &pb.BorrowRequest{
		CollectionId: collectionId.Hex(),
		UserId:       primitive.NewObjectID().Hex(),
	})
	exist, err2 := cache.SIsMember(ctx, "available_books:"+collectionId.Hex(), book.Id).Result()

	// Assert
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, book.Id, resp.BookId)

	require.NoError(t, err2)
	assert.False(t, exist)
}

func TestBorrow_FailedCollectionFetch(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)
	collectionId := primitive.NewObjectID()
	ctx := context.Background()

	mockService.CollectionClient.(*mocks.MockCollectionService).On("FindCollectionById", ctx, &pb.FindCollectionRequest{Id: collectionId.Hex()}).Return(nil, status.Error(codes.NotFound, "Book not found"))

	mockService.BookClient.(*mocks.MockBookServiceClient).On("GetAvailableBook", ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId.Hex()}).Return(nil, status.Error(codes.Aborted, "Error getting books"))

	_, err := mockService.BorrowBook(ctx, &pb.BorrowRequest{
		CollectionId: collectionId.Hex(),
		UserId:       primitive.NewObjectID().Hex(),
	})
	require.Error(t, err)
}

func TestBorrow_FailedBookFetch(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)
	collectionId, _, collection, _, _ := ArrangeBorrowData()
	ctx := context.Background()

	mockService.CollectionClient.(*mocks.MockCollectionService).On("FindCollectionById", ctx, &pb.FindCollectionRequest{Id: collectionId.Hex()}).Return(&pb.Response{Collection: []*pb.Collection{collection}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("GetAvailableBook", ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId.Hex()}).Return(nil, status.Error(codes.Aborted, "Error getting books"))

	_, err := mockService.BorrowBook(ctx, &pb.BorrowRequest{
		CollectionId: collectionId.Hex(),
		UserId:       primitive.NewObjectID().Hex(),
	})
	require.Error(t, err)
}

func TestBorrow_UpdateBookFailure(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	collectionId, _, collection, book, _ := ArrangeBorrowData()
	ctx := context.Background()

	mockService.CollectionClient.(*mocks.MockCollectionService).On("FindCollectionById", ctx, &pb.FindCollectionRequest{Id: collectionId.Hex()}).Return(&pb.Response{Collection: []*pb.Collection{collection}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("GetAvailableBook", ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId.Hex()}).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id &&
			req.Payload.Fields["is_borrowed"].GetBoolValue() == true &&
			req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(nil, status.Error(codes.Internal, "Error updating book status"))

	_, err := mockService.BorrowBook(ctx, &pb.BorrowRequest{
		CollectionId: collection.Id,
	})
	require.Error(t, err)
}

func TestBorrow_CreateBorrowFailure(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	collectionId, bookId, collection, book, _ := ArrangeBorrowData()
	ctx := context.Background()

	mockService.CollectionClient.(*mocks.MockCollectionService).On("FindCollectionById", ctx, &pb.FindCollectionRequest{Id: collectionId.Hex()}).Return(&pb.Response{Collection: []*pb.Collection{collection}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("GetAvailableBook", ctx, &pb.GetAvailableBookRequest{CollectionId: collectionId.Hex()}).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id && req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("Create", ctx, mock.MatchedBy(func(req model.Borrow) bool {
		return req.BookId.Hex() == book.Id && req.CollectionId.Hex() == collection.Id
	})).Return(status.Error(codes.Internal, "Error creating borrow record"))

	cache.SAdd(ctx, "available_books:"+collectionId.Hex(), bookId.Hex(), time.Hour)
	_, err := mockService.BorrowBook(ctx, &pb.BorrowRequest{
		CollectionId: collectionId.Hex(),
		UserId:       primitive.NewObjectID().Hex(),
	})
	require.Error(t, err)

	exist, err := cache.SIsMember(ctx, "available_books:"+collectionId.Hex(), book.Id).Result()
	require.NoError(t, err)
	assert.True(t, exist)
}

func TestReturn_Success(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	collectionId, bookId, borrowId, book, borrowRecord, _ := ArrangeReturnData()
	ctx := context.Background()

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id && req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("FindById", ctx, borrowId.Hex()).Return(borrowRecord, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("Update", ctx, mock.MatchedBy(func(req map[string]interface{}) bool {
		_, ok1 := req["return_date"]
		_, ok2 := req["updated_at"]
		return ok1 && ok2
	}), borrowId.Hex()).Return(borrowRecord, nil)

	resp, err := mockService.ReturnBook(ctx, &pb.ReturnRequest{
		BorrowId: borrowId.Hex(),
	})
	exist, err2 := cache.SIsMember(ctx, "available_books:"+collectionId.Hex(), bookId.Hex()).Result()

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, borrowId.Hex(), resp.Id)

	require.NoError(t, err2)
	assert.True(t, exist)
}

func TestReturn_NotFound(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	borrowId := primitive.NewObjectID()
	ctx := context.Background()

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("FindById", ctx, borrowId.Hex()).Return(nil, mongo.ErrNoDocuments)

	_, err := mockService.ReturnBook(ctx, &pb.ReturnRequest{
		BorrowId: borrowId.Hex(),
	})
	require.Error(t, err)
}

func TestReturn_AlreadyReturned(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	_, _, borrowId, _, borrowRecord, now := ArrangeReturnData()
	borrowRecord.ReturnDate = &now
	ctx := context.Background()

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("FindById", ctx, borrowId.Hex()).Return(borrowRecord, nil)

	_, err := mockService.ReturnBook(ctx, &pb.ReturnRequest{
		BorrowId: borrowId.Hex(),
	})
	require.Error(t, err)
}

func TestReturn_BookUpdateFailure(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	_, _, borrowId, book, borrowRecord, _ := ArrangeReturnData()
	ctx := context.Background()

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id && req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(nil, status.Error(codes.Aborted, "failed to mark book as returned"))

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("FindById", ctx, borrowId.Hex()).Return(borrowRecord, nil)

	_, err := mockService.ReturnBook(ctx, &pb.ReturnRequest{
		BorrowId: borrowId.Hex(),
	})
	require.Error(t, err)
}

func TestReturn_BorrowUpdateFailure(t *testing.T) {
	cache := newRedis(t)
	_, mockService := newServer(cache)

	_, _, borrowId, book, borrowRecord, _ := ArrangeReturnData()
	ctx := context.Background()

	mockService.BookClient.(*mocks.MockBookServiceClient).On("UpdateBook", ctx, mock.MatchedBy(func(req *pb.UpdateBookRequest) bool {
		return req.Id == book.Id && req.Payload.Fields["updated_at"].GetStringValue() != ""
	})).Return(&pb.BookResponse{Book: []*pb.Book{book}}, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("FindById", ctx, borrowId.Hex()).Return(borrowRecord, nil)

	mockService.Service.(*mocks.MockService[model.Borrow, model.BorrowUpdateRequest]).On("Update", ctx, mock.MatchedBy(func(req map[string]interface{}) bool {
		_, ok1 := req["return_date"]
		_, ok2 := req["updated_at"]
		return ok1 && ok2
	}), borrowId.Hex()).Return(nil, status.Error(codes.Internal, "failed to update borrow record"))

	_, err := mockService.ReturnBook(ctx, &pb.ReturnRequest{
		BorrowId: borrowId.Hex(),
	})
	require.Error(t, err)
}
