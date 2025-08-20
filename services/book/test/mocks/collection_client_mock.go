package mocks

import (
	"context"
	"encoding/json"
	"shared/pkg/model"
	pb "shared/proto/buffer"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type MockCollectionService struct {
	mock.Mock
	cache *redis.Client
}

func NewMockCollectionService(cache *redis.Client) *MockCollectionService {
	return &MockCollectionService{
		cache: cache,
	}
}

func (m *MockCollectionService) GetCollection(ctx context.Context, in *pb.GetCollectionRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	return nil, nil
}

func (m *MockCollectionService) FindCollectionById(ctx context.Context, in *pb.FindCollectionRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	return nil, nil
}

func (m *MockCollectionService) AddCollection(ctx context.Context, in *pb.AddCollectionRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	return nil, nil
}

func (m *MockCollectionService) UpdateCollection(ctx context.Context, in *pb.UpdateCollectionRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	return nil, nil
}

func (m *MockCollectionService) DeleteCollection(ctx context.Context, in *pb.DeleteCollectionRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	return nil, nil
}

func (m *MockCollectionService) DecrementAvailableBooks(ctx context.Context, in *pb.DecrementAvailableBooksRequest, opts ...grpc.CallOption) (*pb.Response, error) {
	args := m.Called(ctx, in)

	out, err := m.cache.Get(ctx, "collection:"+in.Id).Bytes()
	if err != nil {
		return nil, err
	}
	var cached model.Collection
	err = json.Unmarshal(out, &cached)
	if err != nil {
		return nil, err
	}
	cached.TotalBooks += int(in.Amount)

	bytes, err := json.Marshal(cached)
	if err != nil {
		return nil, err
	}

	m.cache.Set(ctx, "collection:"+in.Id, bytes, time.Hour)

	return args.Get(0).(*pb.Response), args.Error(1)
}
