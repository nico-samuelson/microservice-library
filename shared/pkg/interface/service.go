package interfaces

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ServiceInterface[K any, V any] interface {
	List(ctx context.Context, filter bson.M, sort bson.D, skip int, limit int) ([]K, error)
	FindById(ctx context.Context, id string) (*K, error)
	Find(ctx context.Context, filter bson.M) (*K, error)
	Create(ctx context.Context, entity K) error
	Update(ctx context.Context, update map[string]interface{}, id string) (K, error)
	Delete(ctx context.Context, id string) (K, error)
	Exists(ctx context.Context, filter bson.M) (bool, error)
	Count(ctx context.Context, filter bson.M) (int64, error)
	BulkInsert(ctx context.Context, entities []K) error
}
