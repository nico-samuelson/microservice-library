package test

import (
	"context"
	"errors"
	"shared/pkg/repository"
	"shared/pkg/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Mock repository for testing
type MockRepository[K any] struct {
	mock.Mock
}

func (m *MockRepository[K]) GetAll(ctx context.Context, filter bson.M, sort bson.D, skip int, limit int) ([]K, error) {
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

// Mock validation service for testing
type MockValidationService[K any, V any] struct {
	mock.Mock
}

func (m *MockValidationService[K, V]) Validate(entity K) error {
	args := m.Called(entity)
	return args.Error(0)
}

func (m *MockValidationService[K, V]) ValidateUpdateRequest(payload map[string]interface{}) (map[string]interface{}, error) {
	args := m.Called(payload)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// Test entity types
type User struct {
	ID    string `bson:"_id" json:"id" validate:"required"`
	Name  string `bson:"name" json:"name" validate:"required"`
	Email string `bson:"email" json:"email" validate:"required,email"`
}

type UserUpdateSchema struct {
	Name  string `json:"name,omitempty" validate:"omitempty"`
	Email string `json:"email,omitempty" validate:"omitempty,email"`
}

// Helper function to create a test service with mocked dependencies
func setupTestService() (*service.BaseService[User, UserUpdateSchema], *MockRepository[User], *MockValidationService[User, UserUpdateSchema]) {
	mockRepo := &MockRepository[User]{}
	mockValidator := &MockValidationService[User, UserUpdateSchema]{}

	service := &service.BaseService[User, UserUpdateSchema]{
		Repo:      mockRepo,
		Validator: mockValidator,
	}

	return service, mockRepo, mockValidator
}

func TestNewBaseService(t *testing.T) {
	repo := &repository.BaseRepository[User]{}
	service := service.NewBaseService[User, UserUpdateSchema](repo)

	assert.NotNil(t, service)
	assert.NotNil(t, service.Repo)
	assert.NotNil(t, service.Validator)
}

func TestBaseService_List(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()

	expectedUsers := []User{
		{ID: "1", Name: "John", Email: "john@example.com"},
		{ID: "2", Name: "Jane", Email: "jane@example.com"},
	}

	t.Run("successful list", func(t *testing.T) {
		mockRepo.On("GetAll", ctx).Return(expectedUsers, nil).Once()

		result, err := service.List(ctx, bson.M{}, bson.D{}, 0, 10)

		assert.NoError(t, err)
		assert.Equal(t, expectedUsers, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.On("GetAll", ctx).Return([]User{}, errors.New("database error")).Once()

		result, err := service.List(ctx, bson.M{}, bson.D{}, 0, 10)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_FindById(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()
	userID := "123"
	expectedUser := &User{ID: userID, Name: "John", Email: "john@example.com"}
	expectedFilter := bson.M{"_id": userID}

	t.Run("successful find", func(t *testing.T) {
		mockRepo.On("Find", ctx, expectedFilter).Return(expectedUser, nil).Once()

		result, err := service.FindById(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockRepo.On("Find", ctx, expectedFilter).Return((*User)(nil), errors.New("not found")).Once()

		result, err := service.FindById(ctx, userID)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Find(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()
	filter := bson.M{"name": "John"}
	expectedUser := &User{ID: "123", Name: "John", Email: "john@example.com"}

	t.Run("successful find", func(t *testing.T) {
		mockRepo.On("Find", ctx, filter).Return(expectedUser, nil).Once()

		result, err := service.Find(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo.On("Find", ctx, filter).Return((*User)(nil), errors.New("not found")).Once()

		result, err := service.Find(ctx, filter)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Create(t *testing.T) {
	service, mockRepo, mockValidator := setupTestService()
	ctx := context.Background()
	user := User{ID: "123", Name: "John", Email: "john@example.com"}

	t.Run("successful create", func(t *testing.T) {
		mockValidator.On("Validate", user).Return(nil).Once()
		mockRepo.On("Insert", ctx, user).Return("123", nil).Once()

		err := service.Create(ctx, user)

		assert.NoError(t, err)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		validationErr := errors.New("validation failed")
		mockValidator.On("Validate", user).Return(validationErr).Once()

		err := service.Create(ctx, user)

		assert.Error(t, err)
		assert.Equal(t, validationErr, err)
		mockValidator.AssertExpectations(t)
		// Repository should not be called if validation fails
		mockRepo.AssertNotCalled(t, "Insert")
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("database error")
		mockValidator.On("Validate", user).Return(nil).Once()
		mockRepo.On("Insert", ctx, user).Return(nil, repoErr).Once()

		err := service.Create(ctx, user)

		assert.Error(t, err)
		assert.Equal(t, repoErr, err)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Update(t *testing.T) {
	service, mockRepo, mockValidator := setupTestService()
	ctx := context.Background()
	userID := "123"
	updateData := map[string]interface{}{
		"name":  "John Updated",
		"email": "john.updated@example.com",
	}
	updatedUser := User{ID: userID, Name: "John Updated", Email: "john.updated@example.com"}

	t.Run("successful update", func(t *testing.T) {
		mockValidator.On("ValidateUpdateRequest", updateData).Return(updateData, nil).Once()
		mockRepo.On("UpdateOne", ctx, updateData, userID).Return(updatedUser, nil).Once()

		result, err := service.Update(ctx, updateData, userID)

		assert.NoError(t, err)
		assert.Equal(t, updatedUser, result)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		validationErr := errors.New("validation failed")
		mockValidator.On("ValidateUpdateRequest", updateData).Return(map[string]interface{}{}, validationErr).Once()

		result, err := service.Update(ctx, updateData, userID)

		assert.Error(t, err)
		assert.Equal(t, validationErr, err)
		assert.Equal(t, User{}, result) // Should return zero value
		mockValidator.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "UpdateOne")
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("database error")
		mockValidator.On("ValidateUpdateRequest", updateData).Return(updateData, nil).Once()
		mockRepo.On("UpdateOne", ctx, updateData, userID).Return(User{}, repoErr).Once()

		result, err := service.Update(ctx, updateData, userID)

		assert.Error(t, err)
		assert.Equal(t, repoErr, err)
		assert.Equal(t, User{}, result)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Delete(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()
	userID := "123"
	deletedUser := User{ID: userID, Name: "John", Email: "john@example.com"}

	t.Run("successful delete", func(t *testing.T) {
		mockRepo.On("DeleteOne", ctx, userID).Return(deletedUser, nil).Once()

		result, err := service.Delete(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, deletedUser, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("delete failed")
		mockRepo.On("DeleteOne", ctx, userID).Return(User{}, repoErr).Once()

		result, err := service.Delete(ctx, userID)

		assert.Error(t, err)
		assert.Equal(t, repoErr, err)
		assert.Equal(t, User{}, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Exists(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()
	filter := bson.M{"email": "john@example.com"}

	t.Run("entity exists", func(t *testing.T) {
		mockRepo.On("DataExists", ctx, filter).Return(true, nil).Once()

		result, err := service.Exists(ctx, filter)

		assert.NoError(t, err)
		assert.True(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("entity does not exist", func(t *testing.T) {
		mockRepo.On("DataExists", ctx, filter).Return(false, nil).Once()

		result, err := service.Exists(ctx, filter)

		assert.NoError(t, err)
		assert.False(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("database error")
		mockRepo.On("DataExists", ctx, filter).Return(false, repoErr).Once()

		result, err := service.Exists(ctx, filter)

		assert.Error(t, err)
		assert.False(t, result)
		assert.Equal(t, repoErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_Count(t *testing.T) {
	service, mockRepo, _ := setupTestService()
	ctx := context.Background()
	filter := bson.M{"active": true}

	t.Run("successful count", func(t *testing.T) {
		expectedCount := int64(42)
		mockRepo.On("Count", ctx, filter).Return(expectedCount, nil).Once()

		result, err := service.Count(ctx, filter)

		assert.NoError(t, err)
		assert.Equal(t, expectedCount, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("count failed")
		mockRepo.On("Count", ctx, filter).Return(int64(0), repoErr).Once()

		result, err := service.Count(ctx, filter)

		assert.Error(t, err)
		assert.Equal(t, int64(0), result)
		assert.Equal(t, repoErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestBaseService_BulkInsert(t *testing.T) {
	service, mockRepo, mockValidator := setupTestService()
	ctx := context.Background()
	users := []User{
		{ID: "1", Name: "John", Email: "john@example.com"},
		{ID: "2", Name: "Jane", Email: "jane@example.com"},
	}

	t.Run("successful bulk insert", func(t *testing.T) {
		// Expect validation for each entity
		for _, user := range users {
			mockValidator.On("Validate", user).Return(nil).Once()
		}
		mockRepo.On("BulkInsert", ctx, users).Return([]string{"1", "2"}, nil).Once()

		err := service.BulkInsert(ctx, users)

		assert.NoError(t, err)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("validation error on first entity", func(t *testing.T) {
		validationErr := errors.New("validation failed")
		mockValidator.On("Validate", users[0]).Return(validationErr).Once()

		err := service.BulkInsert(ctx, users)

		assert.Error(t, err)
		assert.Equal(t, validationErr, err)
		mockValidator.AssertExpectations(t)
		// Repository should not be called if validation fails
		mockRepo.AssertNotCalled(t, "BulkInsert")
	})

	t.Run("validation error on second entity", func(t *testing.T) {
		validationErr := errors.New("validation failed on second entity")
		mockValidator.On("Validate", users[0]).Return(nil).Once()
		mockValidator.On("Validate", users[1]).Return(validationErr).Once()

		err := service.BulkInsert(ctx, users)

		assert.Error(t, err)
		assert.Equal(t, validationErr, err)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "BulkInsert")
	})

	t.Run("repository error", func(t *testing.T) {
		repoErr := errors.New("bulk insert failed")
		// Validation should pass for all entities
		for _, user := range users {
			mockValidator.On("Validate", user).Return(nil).Once()
		}
		mockRepo.On("BulkInsert", ctx, users).Return(nil, repoErr).Once()

		err := service.BulkInsert(ctx, users)

		assert.Error(t, err)
		assert.Equal(t, repoErr, err)
		mockValidator.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty slice", func(t *testing.T) {
		emptyUsers := []User{}
		mockRepo.On("BulkInsert", ctx, emptyUsers).Return([]string{}, nil).Once()

		err := service.BulkInsert(ctx, emptyUsers)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		// Validator should not be called for empty slice
		mockValidator.AssertNotCalled(t, "Validate")
	})
}
