package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type MockService[T any, U any] struct{ mock.Mock }

func (m *MockService[T, U]) List(ctx context.Context, filter bson.M, sort bson.D, skip int, limit int) ([]T, error) {
	args := m.Called(ctx)
	if v, ok := args.Get(0).([]T); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockService[T, U]) FindById(ctx context.Context, id string) (*T, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*T); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockService[T, U]) Find(ctx context.Context, filter bson.M) (*T, error) {
	args := m.Called(ctx, filter)
	if v, ok := args.Get(0).(*T); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockService[T, U]) Create(ctx context.Context, doc T) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}
func (m *MockService[T, U]) Update(ctx context.Context, update map[string]interface{}, id string) (T, error) {
	args := m.Called(ctx, update, id)
	var zero T
	if v, ok := args.Get(0).(T); ok {
		return v, args.Error(1)
	}
	return zero, args.Error(1)
}
func (m *MockService[T, U]) Delete(ctx context.Context, id string) (T, error) {
	args := m.Called(ctx, id)
	var zero T
	if v, ok := args.Get(0).(T); ok {
		return v, args.Error(1)
	}
	return zero, args.Error(1)
}
func (m *MockService[T, U]) Exists(ctx context.Context, filter bson.M) (bool, error) {
	args := m.Called(ctx, filter)
	return args.Bool(0), args.Error(1)
}

func (m *MockService[T, U]) BulkInsert(ctx context.Context, entities []T) error {
	args := m.Called(ctx, entities)
	return args.Error(0)
}

func (m *MockService[T, U]) Count(ctx context.Context, filter bson.M) (int64, error) {
	args := m.Called(ctx, filter)
	if v, ok := args.Get(0).(int64); ok {
		return v, args.Error(1)
	}
	return 0, args.Error(1)
}
