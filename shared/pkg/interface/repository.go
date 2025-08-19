package interfaces

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RepositoryInterface[K any] interface {
	GetAll(ctx context.Context, filter bson.M, sort bson.D, skip int, limit int) ([]K, error)
	Find(ctx context.Context, filter bson.M) (*K, error)
	Insert(ctx context.Context, entity K) (interface{}, error)
	UpdateOne(ctx context.Context, update map[string]interface{}, id string) (K, error)
	DeleteOne(ctx context.Context, id string) (K, error)
	DataExists(ctx context.Context, filter bson.M) (bool, error)
	Count(ctx context.Context, filter bson.M) (int64, error)
	BulkInsert(ctx context.Context, entities []K) (interface{}, error)
}
