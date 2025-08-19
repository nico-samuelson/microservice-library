package internal

import (
	"context"
	"log"
	"shared/pkg/model"
	"shared/pkg/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type CollectionRepositoryInterface interface {
	UpdateBookStock(ctx context.Context, obj map[string]interface{}, id string) (interface{}, error)
}

type CollectionRepository struct {
	Repository repository.BaseRepository[model.Collection]
}

func NewCollectionRepository(database *mongo.Database, collection_name string) *CollectionRepository {
	return &CollectionRepository{
		Repository: *repository.NewRepository[model.Collection](database, collection_name),
	}
}

func (r *CollectionRepository) UpdateBookStock(ctx context.Context, obj map[string]interface{}, id string) (interface{}, error) {
	coll := r.Repository.Database.Collection(r.Repository.CollectionName)

	// Convert id into Object ID
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Error converting string to object ID: %s", err)
		return nil, err
	}

	result, err := coll.UpdateOne(
		ctx,
		bson.M{"_id": objectId},
		bson.M{"$inc": obj},
	)

	if err != nil {
		log.Printf("Error updating data: %s", err)
	}

	return result, err
}
