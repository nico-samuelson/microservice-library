package test

import (
	"collection/internal"
	"collection/test/mocks"
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
)

func newRedis(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func newServer(cache *redis.Client) (*mocks.MockService[model.Collection, model.CollectionUpdateRequest], *internal.CollectionServiceServer, *mocks.MockCollectionRepository) {
	mockService := &mocks.MockService[model.Collection, model.CollectionUpdateRequest]{}
	repository := &mocks.MockCollectionRepository{}
	svc := &internal.CollectionServiceServer{
		Service:    mockService,
		Repository: repository,
		Cache:      cache,
		BookClient: &mocks.MockBookServiceClient{},
	}

	return mockService, svc, repository
}

func TestGetCollection_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	// Arrange
	ctx := context.Background()
	mockData := []model.Collection{{Id: primitive.NewObjectID(), Name: "Test", Author: "Author"}}
	mockBaseService.On("List", ctx).Return(mockData, nil)

	filterMap := map[string]interface{}{}
	filter, err := structpb.NewStruct(filterMap)
	require.NoError(t, err)

	sort := []*pb.Sort{}

	// Act
	resp, err := mockService.GetCollection(ctx, &pb.GetCollectionRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   0,
		Limit:  10,
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.Collection)
}

func TestGetCollection_Error(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	ctx := context.Background()
	mockBaseService.On("List", ctx).Return(nil, errors.New("db error"))

	filterMap := map[string]interface{}{}
	filter, err := structpb.NewStruct(filterMap)
	require.NoError(t, err)

	sort := []*pb.Sort{}

	// Act
	resp, err := mockService.GetCollection(ctx, &pb.GetCollectionRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   0,
		Limit:  10,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestFindCollectionById_CacheMissThenSet(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	id := primitive.NewObjectID()
	mc := &model.Collection{Id: id, Name: "Dune", Author: "Frank Herbert"}
	mockBaseService.On("Find", mockAnyCtx(), bson.M{"_id": id.Hex()}).Return(mc, nil)

	resp, err := mockService.FindCollectionById(context.Background(), &pb.FindCollectionRequest{Id: id.Hex()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	require.Len(t, resp.Collection, 1)
	assert.Equal(t, id.Hex(), resp.Collection[0].Id)

	// Verify cached value exists and matches
	raw, err := cache.Get(context.Background(), "collection:"+id.Hex()).Bytes()
	require.NoError(t, err)
	var cached model.Collection
	require.NoError(t, json.Unmarshal(raw, &cached))
	assert.Equal(t, mc.Name, cached.Name)
	assert.Equal(t, mc.Author, cached.Author)
}

func TestAddCollection_AlreadyExists(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	inpb := &pb.AddCollectionRequest{Collection: &pb.Collection{Name: "Name", Author: "Author"}}
	mockBaseService.On("Exists", mockAnyCtx(), bson.M{"name": "Name", "author": "Author"}).Return(true, nil)

	resp, err := mockService.AddCollection(context.Background(), inpb)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Collection already exists", resp.Message)
}

func TestAddCollection_CreateSuccess_NoBooks(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	inpb := &pb.AddCollectionRequest{Collection: &pb.Collection{Name: "C", Author: "A", TotalBooks: 0}}
	mockBaseService.On("Exists", mockAnyCtx(), bson.M{"name": "C", "author": "A"}).Return(false, nil)
	mockBaseService.On("Create", mockAnyCtx(), mock.Anything).Return(nil)

	resp, err := mockService.AddCollection(context.Background(), inpb)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, inpb.Collection.Name, resp.Collection[0].Name)
}

func TestUpdateCollection_NameAuthorConflict(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	otherID := primitive.NewObjectID() // existing conflicting id
	found := &model.Collection{Id: otherID}
	mockBaseService.On("Find", mockAnyCtx(), bson.M{"name": "New", "author": "A"}).Return(found, nil)

	_, err := mockService.UpdateCollection(context.Background(), &pb.UpdateCollectionRequest{Id: primitive.NewObjectID().Hex(), Payload: &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name":   structpb.NewStringValue("New"),
			"author": structpb.NewStringValue("A"),
		},
	}})
	require.Error(t, err)
}

func TestUpdateCollection_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	id := primitive.NewObjectID()
	// Find returns no documents
	mockBaseService.On("Find", mockAnyCtx(), mock.Anything).Return(&model.Collection{}, mongo.ErrNoDocuments)

	updated := model.Collection{Id: id, Name: "New", Author: "Auth"}
	mockBaseService.On("Update", mockAnyCtx(), mock.MatchedBy(func(m map[string]any) bool { return m["updated_at"] != nil }), id.Hex()).Return(updated, nil)

	resp, err := mockService.UpdateCollection(context.Background(), &pb.UpdateCollectionRequest{Id: id.Hex(), Payload: &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name": structpb.NewStringValue("New"),
		},
	}})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, updated.Name, resp.Collection[0].Name)
	assert.Equal(t, updated.Id.Hex(), resp.Collection[0].Id)
}

func TestDeleteCollection_NotFound(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	mockBaseService.On("Delete", mockAnyCtx(), "missing").Return(model.Collection{}, mongo.ErrNoDocuments)

	resp, err := mockService.DeleteCollection(context.Background(), &pb.DeleteCollectionRequest{Id: "missing"})
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Collection not found", resp.Message)
}

func TestDeleteCollection_Success(t *testing.T) {
	cache := newRedis(t)
	mockBaseService, mockService, _ := newServer(cache)

	id := primitive.NewObjectID()
	deleted := model.Collection{Id: id}
	mockBaseService.On("Delete", mockAnyCtx(), id.Hex()).Return(deleted, nil)

	resp, err := mockService.DeleteCollection(context.Background(), &pb.DeleteCollectionRequest{Id: id.Hex()})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, id.Hex(), resp.Collection[0].Id)
}

func TestDecrementAvailableBooks_UpdatesStockAndCache(t *testing.T) {
	cache := newRedis(t)
	// repo := new(mocks.MockCollectionRepository)
	_, mockService, repo := newServer(cache)

	id := primitive.NewObjectID().Hex()
	// seed cache with collection having AvailableBooks=5
	seed := &model.Collection{Id: mustOID(id), TotalBooks: 5}
	raw, _ := json.Marshal(seed)
	require.NoError(t, cache.Set(context.Background(), "collection:"+id, raw, time.Hour).Err())

	// use a matcher to allow flexible map matching (int vs int32)
	repo.On("UpdateBookStock", mockAnyCtx(), mock.MatchedBy(func(m map[string]interface{}) bool {
		v, ok := m["total_books"]
		if !ok {
			return false
		}
		switch x := v.(type) {
		case int:
			return x == 1
		case int32:
			return x == 1
		case int64:
			return x == 1
		case float64:
			return int(x) == 1
		default:
			return false
		}
	}), id).Return(mongo.UpdateResult{ModifiedCount: 1}, nil)

	resp, err := mockService.DecrementAvailableBooks(context.Background(), &pb.DecrementAvailableBooksRequest{Id: id, Amount: 1})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	out, err := cache.Get(context.Background(), "collection:"+id).Bytes()
	require.NoError(t, err)
	var cached model.Collection
	require.NoError(t, json.Unmarshal(out, &cached))
	assert.Equal(t, 6, cached.TotalBooks)
}

func mockAnyCtx() interface{} { return mock.MatchedBy(func(ctx context.Context) bool { return true }) }
func mustOID(hex string) primitive.ObjectID {
	id, _ := primitive.ObjectIDFromHex(hex)
	return id
}
