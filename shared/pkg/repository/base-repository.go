package repository

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type BaseRepository[K any] struct {
	Database       *mongo.Database
	CollectionName string
}

func NewRepository[K any](database *mongo.Database, collection_name string) *BaseRepository[K] {
	return &BaseRepository[K]{Database: database, CollectionName: collection_name}
}

func (r *BaseRepository[K]) GetAll(ctx context.Context) ([]K, error) {
	coll := r.Database.Collection(r.CollectionName)
	cursor, err := coll.Find(ctx, bson.D{})
	if err != nil {
		log.Printf("Error fetching data: %s", err)
		return []K{}, err
	}
	defer cursor.Close(ctx)

	var results []K
	if err = cursor.All(ctx, &results); err != nil {
		log.Printf("Error decoding data: %s", err)
		return []K{}, err
	}

	return results, err
}

func (r *BaseRepository[K]) Find(ctx context.Context, filter bson.M) (*K, error) {
	var result K

	idStr, ok := filter["_id"].(string)
	if ok {
		objectID, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			log.Printf("Error converting string to object ID: %s", err)
			return &result, err
		}
		filter["_id"] = objectID
	}

	coll := r.Database.Collection(r.CollectionName)
	err := coll.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		log.Printf("Error finding data: %s", err)
	}

	return &result, err
}

func (r *BaseRepository[K]) Insert(ctx context.Context, obj K) (*mongo.InsertOneResult, error) {
	coll := r.Database.Collection(r.CollectionName)
	result, err := coll.InsertOne(ctx, obj)

	if err != nil {
		log.Printf("Error deleting data: %s", err)
	}

	return result, err
}

func (r *BaseRepository[K]) UpdateOne(ctx context.Context, obj map[string]interface{}, id string) (K, error) {
	coll := r.Database.Collection(r.CollectionName)
	obj["updated_at"] = time.Now()

	// Convert id into Object ID
	var result K
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Error converting string to object ID: %s", err)
		return result, err
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err = coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": objectId}, // Assuming obj has an _id field
		bson.M{"$set": obj},
		opts,
	).Decode(&result)

	if err != nil {
		log.Printf("Error updating data: %s", err)
	}

	return result, err
}

func (r *BaseRepository[K]) DeleteOne(ctx context.Context, id string) (K, error) {
	coll := r.Database.Collection(r.CollectionName)
	var result K

	// Convert id into Object ID
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Printf("Error converting string to object ID: %s", err)
		return result, err
	}

	err = coll.FindOneAndDelete(ctx, bson.M{"_id": objectId}).Decode(&result)
	if err != nil {
		log.Printf("Error deleting data: %s", err)
	}
	return result, err
}

func (r *BaseRepository[K]) DataExists(ctx context.Context, filter bson.M) (bool, error) {
	coll := r.Database.Collection(r.CollectionName)

	opts := options.FindOne().SetProjection(bson.M{"_id": 1})
	var result bson.M
	err := coll.FindOne(ctx, filter, opts).Decode(&result)

	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check data existence: %w", err)
	}

	return true, nil
}

func (r *BaseRepository[K]) Upsert(ctx context.Context, data K, filter bson.M) (*mongo.UpdateResult, error) {
	coll := r.Database.Collection(r.CollectionName)
	update := r.buildUpdateDocument(data)
	update["created_at"] = time.Now()
	update["updated_at"] = time.Now()

	opts := options.UpdateOne().SetUpsert(true)
	result, err := coll.UpdateOne(
		ctx,
		filter,
		bson.M{"$setOnInsert": update},
		opts,
	)
	options.UpdateOne().SetUpsert(false)

	return result, err
}

func (r *BaseRepository[K]) buildUpdateDocument(data K) bson.M {
	update := bson.M{}
	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)

	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		valueField := v.Field(i)

		// Get the bson tag
		bsonTag := structField.Tag.Get("bson")

		// Skip if explicitly ignored or tag is missing
		if bsonTag == "-" || bsonTag == "" {
			continue
		}

		// Extract field name before comma
		fieldName := bsonTag
		if commaIdx := indexComma(fieldName); commaIdx != -1 {
			fieldName = fieldName[:commaIdx]
		}
		if fieldName == "" {
			fieldName = structField.Name
		}

		update[fieldName] = valueField.Interface()
	}

	return update
}

func indexComma(tag string) int {
	for i, r := range tag {
		if r == ',' {
			return i
		}
	}
	return -1
}
