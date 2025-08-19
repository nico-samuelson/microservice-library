package mocks

import (
	"context"
	"shared/pkg/model"

	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MockCollectionRepository struct {
	mock.Mock
	Repository MockRepository[model.Collection]
}

func (m *MockCollectionRepository) UpdateBookStock(ctx context.Context, update map[string]interface{}, id string) (interface{}, error) {
	args := m.Called(ctx, update, id)
	if res := args.Get(0); res != nil {
		return res, args.Error(1)
	}
	return mongo.UpdateResult{}, args.Error(1)
}
