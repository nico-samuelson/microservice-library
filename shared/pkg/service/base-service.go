package service

import (
	"context"
	"shared/pkg/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BaseService[K any, V any] struct {
	Repo      *repository.BaseRepository[K]
	Validator *ValidationService[K, V]
}

func NewBaseService[K any, V any](repository *repository.BaseRepository[K]) *BaseService[K, V] {
	return &BaseService[K, V]{
		Repo:      repository,
		Validator: NewValidationService[K, V](),
	}
}

func (s *BaseService[K, V]) List(ctx context.Context) ([]K, error) {
	return s.Repo.GetAll(ctx)
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

// func (s *BaseService[K]) GetData(ctx context.Context, message model.RabbitMQMessage) (model.RabbitMQMessage, error) {
// 	results, err := s.Repo.GetAll(ctx)

// 	if err != nil {
// 		return utils.CreateReply(message.CorrelationID, message.Action, false, 500, "Internal Server Error", []interface{}{}), err
// 	}
// 	return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data found!", []interface{}{results}), nil
// }

// func (s *BaseService[K]) FindData(ctx context.Context, message model.RabbitMQMessage, filter bson.M) (model.RabbitMQMessage, error) {
// 	results, err := s.Repo.Find(ctx, filter)

// 	if err == mongo.ErrNoDocuments {
// 		return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data not found", []interface{}{}), err
// 	}
// 	if err != nil {
// 		return utils.CreateReply(message.CorrelationID, message.Action, false, 500, "Internal Server Error", []interface{}{}), err
// 	}
// 	return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data found!", []interface{}{results}), nil
// }

// func (s *BaseService[K]) AddData(ctx context.Context, message model.RabbitMQMessage, data K) (model.RabbitMQMessage, error) {
// 	if _, err := s.Repo.Insert(ctx, data); err != nil {
// 		log.Printf("Error inserting new data: %v", err)
// 		return utils.CreateReply(message.CorrelationID, message.Action, false, 500, "Internal Server Error", []interface{}{}), err
// 	}

// 	return utils.CreateReply(message.CorrelationID, message.Action, true, 201, "Data added successfully!", []interface{}{data}), nil
// }

// func (s *BaseService[K]) UpdateData(ctx context.Context, message model.RabbitMQMessage, data map[string]interface{}, filter bson.M) (model.RabbitMQMessage, error) {
// 	result, _, err := s.Repo.UpdateOne(ctx, data, filter)

// 	if err == mongo.ErrNoDocuments {
// 		return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data not found", []interface{}{}), err
// 	}
// 	if err != nil {
// 		return utils.CreateReply(message.CorrelationID, message.Action, false, 500, "Internal Server Error", []interface{}{}), err
// 	}
// 	return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data updated successfully!", []interface{}{result}), err
// }

// func (s *BaseService[K]) DeleteData(ctx context.Context, message model.RabbitMQMessage) (model.RabbitMQMessage, error) {
// 	id := message.Payload["id"].(string)
// 	result, err := s.Repo.DeleteOne(ctx, id)

// 	if err == mongo.ErrNoDocuments {
// 		return utils.CreateReply(message.CorrelationID, message.Action, true, 200, "Data not found", []interface{}{}), err
// 	}
// 	if err != nil {
// 		return utils.CreateReply(message.CorrelationID, message.Action, false, 500, "Internal Server Error", []interface{}{}), err
// 	}
// 	return utils.CreateReply(message.CorrelationID, message.Action, true, 201, "Data deleted successfully!", []interface{}{result}), nil
// }

// func (s *BaseService[K]) SendReplyWithRetry(ctx context.Context, queue string, reply model.RabbitMQMessage, maxRetries int) error {
// 	var lastErr error
// 	for i := range maxRetries {
// 		if err := rabbitmq.RabbitMQClient.SendMessage(queue, "", reply); err != nil {
// 			lastErr = err
// 			if i < maxRetries-1 {
// 				select {
// 				case <-ctx.Done():
// 					return ctx.Err()
// 				case <-time.After(time.Duration(i+1) * time.Second):
// 					continue
// 				}
// 			}
// 		} else {
// 			return nil
// 		}
// 	}
// 	return fmt.Errorf("failed to send reply after %d retries: %w", maxRetries, lastErr)
// }

// func (s *BaseService[K]) ProcessMessage(ctx context.Context, d interface{}, replyQueue string) error {
// 	return nil
// }
