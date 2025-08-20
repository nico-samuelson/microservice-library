package test

import (
	"book/internal"
	"book/test/mocks"
	"context"
	"encoding/json"
	"errors"
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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func newRedis(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newServer(cache *redis.Client) (*mocks.MockService[model.Book, model.BookUpdateRequest], *internal.BookServiceServer) {
	mockService := &mocks.MockService[model.Book, model.BookUpdateRequest]{}

	svc := &internal.BookServiceServer{
		Service:          mockService,
		Cache:            cache,
		CollectionClient: mocks.NewMockCollectionService(cache),
	}

	return mockService, svc
}

func TestGetBook_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	// Arrange
	ctx := context.Background()
	mockData := []model.Book{{Id: primitive.NewObjectID(), CollectionId: primitive.NewObjectID(), IsBorrowed: false}}
	mockBaseService.On("List", ctx).Return(mockData, nil)

	filterMap := map[string]interface{}{}
	filter, err := structpb.NewStruct(filterMap)
	require.NoError(t, err)

	sort := []*pb.Sort{}

	// Act
	resp, err := mockService.GetBook(ctx, &pb.GetBookRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   0,
		Limit:  10,
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, resp.Success)
	// assert.Equal(t, "Books retrieved successfully", resp.Message)
	assert.NotEmpty(t, resp.Book)
}

func TestGetBook_Error(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	ctx := context.Background()
	mockBaseService.On("List", ctx).Return(nil, errors.New("db error"))

	filterMap := map[string]interface{}{}
	filter, err := structpb.NewStruct(filterMap)
	require.NoError(t, err)

	sort := []*pb.Sort{}

	// Act
	resp, err := mockService.GetBook(ctx, &pb.GetBookRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   0,
		Limit:  10,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestFindBookById_CacheMissThenSet(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	id := primitive.NewObjectID()
	collectionId := primitive.NewObjectID()
	mc := &model.Book{Id: id, CollectionId: collectionId, IsBorrowed: false}
	mockBaseService.On("Find", mockAnyCtx(), bson.M{"_id": id.Hex()}).Return(mc, nil)

	resp, err := mockService.FindBookById(context.Background(), &pb.FindBookRequest{Id: id.Hex()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	require.Len(t, resp.Book, 1)
	assert.Equal(t, id.Hex(), resp.Book[0].Id)
	// assert.Equal(t, "Book found", resp.Message)

	// Verify cached value exists and matches
	raw, err := cache.Get(context.Background(), "book:"+id.Hex()).Bytes()
	require.NoError(t, err)
	var cached model.Book
	require.NoError(t, json.Unmarshal(raw, &cached))
	assert.Equal(t, mc.Id, cached.Id)
}

func TestAddBook_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	collectionId := primitive.NewObjectID()

	// seed cache with collection having AvailableBooks=5
	seed := &model.Collection{Id: mustOID(collectionId.Hex()), TotalBooks: 5}
	raw, _ := json.Marshal(seed)
	require.NoError(t, cache.Set(context.Background(), "collection:"+collectionId.Hex(), raw, time.Hour).Err())

	inpb := &pb.AddBookRequest{Book: &pb.Book{CollectionId: collectionId.Hex(), IsBorrowed: &wrapperspb.BoolValue{Value: false}}}

	mockBaseService.On("Create", mockAnyCtx(), mock.Anything).Return(nil)
	mockService.CollectionClient.(*mocks.MockCollectionService).On(
		"DecrementAvailableBooks",
		mock.AnythingOfType("*context.timerCtx"),
		&pb.DecrementAvailableBooksRequest{
			Id:     collectionId.Hex(),
			Amount: 1,
		},
	).Return(&pb.Response{Success: true}, nil)

	resp, err := mockService.AddBook(context.Background(), inpb)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	// assert.Equal(t, )
	// assert.Equal(t, "Book added!", resp.Message)

	// Wait a bit for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	out, err := cache.Get(context.Background(), "collection:"+collectionId.Hex()).Bytes()
	require.NoError(t, err)
	var cached model.Collection
	require.NoError(t, json.Unmarshal(out, &cached))
	assert.Equal(t, 6, cached.TotalBooks)

	// Verify that the mock was called as expected
	mockService.CollectionClient.(*mocks.MockCollectionService).AssertExpectations(t)
}

func TestUpdateBook_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	id := primitive.NewObjectID()
	collectionId := primitive.NewObjectID()

	updated := model.Book{Id: id, CollectionId: collectionId, IsBorrowed: true}
	mockBaseService.On("Update", mockAnyCtx(), mock.MatchedBy(func(m map[string]any) bool { return m["updated_at"] != nil }), id.Hex()).Return(updated, nil)

	resp, err := mockService.UpdateBook(context.Background(), &pb.UpdateBookRequest{Id: id.Hex(), Payload: &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"is_borrowed": structpb.NewBoolValue(true),
		},
	}})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.True(t, resp.Book[0].IsBorrowed.Value)
	assert.Equal(t, updated.IsBorrowed, resp.Book[0].IsBorrowed.Value)
	// assert.Equal(t, "Book updated!", resp.Message)
}

func TestDeleteBook_NotFound(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	mockBaseService.On("Delete", mockAnyCtx(), "missing").Return(model.Book{}, mongo.ErrNoDocuments)

	resp, err := mockService.DeleteBook(context.Background(), &pb.DeleteBookRequest{Id: "missing"})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Empty(t, resp.Book)
	// assert.Equal(t, "Book not found", resp.Message)
}

func TestDeleteBook_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	collectionId := primitive.NewObjectID()
	seed := &model.Collection{Id: mustOID(collectionId.Hex()), TotalBooks: 5}
	raw, _ := json.Marshal(seed)
	require.NoError(t, cache.Set(context.Background(), "collection:"+collectionId.Hex(), raw, time.Hour).Err())

	id := primitive.NewObjectID()
	deleted := model.Book{Id: id, CollectionId: collectionId}

	mockBaseService.On("Delete", mockAnyCtx(), mock.Anything).Return(deleted, nil)
	mockService.CollectionClient.(*mocks.MockCollectionService).On(
		"DecrementAvailableBooks",
		mock.AnythingOfType("*context.timerCtx"), // or mock.Anything for simplicity
		&pb.DecrementAvailableBooksRequest{
			Id:     collectionId.Hex(),
			Amount: -1,
		},
	).Return(&pb.Response{Success: true}, nil)

	resp, err := mockService.DeleteBook(context.Background(), &pb.DeleteBookRequest{Id: id.Hex()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, deleted.Id.Hex(), resp.Book[0].Id)

	// Wait a bit for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	out, err := cache.Get(context.Background(), "collection:"+collectionId.Hex()).Bytes()
	require.NoError(t, err)
	var cached model.Collection
	require.NoError(t, json.Unmarshal(out, &cached))
	assert.Equal(t, 4, cached.TotalBooks)
}

func TestGetAvailableBook_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService := newServer(cache)

	collectionId := primitive.NewObjectID()
	id1 := primitive.NewObjectID()

	mockBaseService.On("Find", mockAnyCtx(), bson.M{
		"collection_id": collectionId,
		"is_borrowed":   false,
	}).Return(&model.Book{Id: id1, CollectionId: collectionId}, nil)

	resp, err := mockService.GetAvailableBook(context.Background(), &pb.GetAvailableBookRequest{
		CollectionId: collectionId.Hex(),
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, id1.Hex(), resp.Book[0].Id)

	// Wait a bit for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	assert.True(t, cache.SIsMember(context.Background(), "available_books:"+collectionId.Hex(), id1.Hex()).Val())
}

func mockAnyCtx() interface{} { return mock.MatchedBy(func(ctx context.Context) bool { return true }) }
func mustOID(hex string) primitive.ObjectID {
	id, _ := primitive.ObjectIDFromHex(hex)
	return id
}
