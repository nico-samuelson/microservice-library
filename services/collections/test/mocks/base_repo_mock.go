package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Mock repository for testing
type MockRepository[K any] struct {
	mock.Mock
}

func (m *MockRepository[K]) GetAll(ctx context.Context) ([]K, error) {
	args := m.Called(ctx)
	return args.Get(0).([]K), args.Error(1)
}

func (m *MockRepository[K]) Find(ctx context.Context, filter bson.M) (*K, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*K), args.Error(1)
}

func (m *MockRepository[K]) Insert(ctx context.Context, entity K) (interface{}, error) {
	args := m.Called(ctx, entity)
	return args.Get(0), args.Error(1)
}

func (m *MockRepository[K]) UpdateOne(ctx context.Context, update map[string]interface{}, id string) (K, error) {
	args := m.Called(ctx, update, id)
	return args.Get(0).(K), args.Error(1)
}

func (m *MockRepository[K]) DeleteOne(ctx context.Context, id string) (K, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(K), args.Error(1)
}

func (m *MockRepository[K]) DataExists(ctx context.Context, filter bson.M) (bool, error) {
	args := m.Called(ctx, filter)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository[K]) Count(ctx context.Context, filter bson.M) (int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository[K]) BulkInsert(ctx context.Context, entities []K) (interface{}, error) {
	args := m.Called(ctx, entities)
	return args.Get(0), args.Error(1)
}
