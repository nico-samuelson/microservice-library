package mocks

import (
	"context"
	pb "shared/proto/buffer"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type MockBookServiceClient struct {
	mock.Mock
	cache *redis.Client
}

func NewMockBookService(cache *redis.Client) *MockBookServiceClient {
	return &MockBookServiceClient{
		cache: cache,
	}
}

func (m *MockBookServiceClient) BulkInsert(ctx context.Context, in *pb.BulkInsertBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*pb.BookResponse); ok {
		return v, args.Error(1)
	}
	return &pb.BookResponse{}, args.Error(1)
}

func (m *MockBookServiceClient) GetBook(ctx context.Context, in *pb.GetBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	return nil, nil
}

func (m *MockBookServiceClient) FindBookById(ctx context.Context, in *pb.FindBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	return nil, nil
}

func (m *MockBookServiceClient) AddBook(ctx context.Context, in *pb.AddBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	return nil, nil
}

func (m *MockBookServiceClient) UpdateBook(ctx context.Context, in *pb.UpdateBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*pb.BookResponse); ok {
		return v, args.Error(1)
	}
	return &pb.BookResponse{}, args.Error(1)
}

func (m *MockBookServiceClient) DeleteBook(ctx context.Context, in *pb.DeleteBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	return nil, nil
}

func (m *MockBookServiceClient) GetAvailableBook(ctx context.Context, in *pb.GetAvailableBookRequest, opts ...grpc.CallOption) (*pb.BookResponse, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*pb.BookResponse); ok {
		return v, args.Error(1)
	}
	return &pb.BookResponse{}, args.Error(1)
}

func (m *MockBookServiceClient) CountBook(ctx context.Context, in *pb.CountBookRequest, opts ...grpc.CallOption) (*pb.BookCountResponse, error) {
	return nil, nil
}
