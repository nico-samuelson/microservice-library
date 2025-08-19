package service

import (
	"context"
	"log"
	interfaces "shared/pkg/interface"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BaseService[K any, V any] struct {
	Repo      interfaces.RepositoryInterface[K]
	Validator interfaces.ValidatorInterface[K, V]
}

func NewBaseService[K any, V any](repository interfaces.RepositoryInterface[K]) *BaseService[K, V] {
	return &BaseService[K, V]{
		Repo:      repository,
		Validator: NewValidationService[K, V](),
	}
}

func (s *BaseService[K, V]) List(ctx context.Context, filter bson.M, sort bson.D, skip int, limit int) ([]K, error) {
	return s.Repo.GetAll(ctx, filter, sort, skip, limit)
}

func (s *BaseService[K, V]) FindById(ctx context.Context, id string) (*K, error) {
	return s.Repo.Find(ctx, bson.M{"_id": id})
}

func (s *BaseService[K, V]) Find(ctx context.Context, filter bson.M) (*K, error) {
	return s.Repo.Find(ctx, filter)
}

func (s *BaseService[K, V]) Create(ctx context.Context, entity K) error {
	// Validate the entity
	err := s.Validator.Validate(entity)
	if err != nil {
		log.Printf("Error validating data: %v", err)
		return err
	}

	_, err = s.Repo.Insert(ctx, entity)
	return err
}

func (s *BaseService[K, V]) Update(ctx context.Context, update map[string]interface{}, id string) (K, error) {
	// Validate the update data
	var entity K
	_, err := s.Validator.ValidateUpdateRequest(update)
	if err != nil {
		return entity, err
	}

	return s.Repo.UpdateOne(ctx, update, id)
}

func (s *BaseService[K, V]) Delete(ctx context.Context, id string) (K, error) {
	return s.Repo.DeleteOne(ctx, id)
}

func (s *BaseService[K, V]) Exists(ctx context.Context, filter bson.M) (bool, error) {
	return s.Repo.DataExists(ctx, filter)
}

func (s *BaseService[K, V]) Count(ctx context.Context, filter bson.M) (int64, error) {
	return s.Repo.Count(ctx, filter)
}

func (s *BaseService[K, V]) BulkInsert(ctx context.Context, entities []K) error {
	// Validate the entity
	for _, entity := range entities {
		err := s.Validator.Validate(entity)
		if err != nil {
			log.Printf("Error validating data: %v", err)
			return err
		}
	}

	_, err := s.Repo.BulkInsert(ctx, entities)
	return err
}
