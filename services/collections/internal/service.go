package internal

import (
	"context"
	"encoding/json"
	"log"
	"time"

	interfaces "shared/pkg/interface"
	"shared/pkg/model"
	"shared/pkg/service"
	"shared/pkg/utils"
	pb "shared/proto/buffer"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type CollectionServiceServer struct {
	pb.UnimplementedCollectionServiceServer
	Service    interfaces.ServiceInterface[model.Collection, model.CollectionUpdateRequest]
	Repository CollectionRepositoryInterface
	Cache      *redis.Client
	BookClient pb.BookServiceClient
}

func NewCollectionService(database *mongo.Database, collection_name string, connections map[string]*grpc.ClientConn, cache *redis.Client) *CollectionServiceServer {
	repository := NewCollectionRepository(database, collection_name)

	return &CollectionServiceServer{
		Service:    service.NewBaseService[model.Collection, model.CollectionUpdateRequest](repository.Repository),
		Repository: repository,
		Cache:      cache,
		BookClient: pb.NewBookServiceClient(connections["book"]),
	}
}

func (s *CollectionServiceServer) GetCollection(ctx context.Context, in *pb.GetCollectionRequest) (*pb.Response, error) {
	// Parse filter and sort from protobuf
	var filter bson.M
	var sort bson.D

	if len(in.Filter.Fields) > 0 {
		filterMap := in.Filter.AsMap()
		filter = bson.M{}
		for k, v := range filterMap {
			filter[k] = v
		}
	} else {
		filter = bson.M{}
	}

	if len(in.Sort) > 0 {
		sort = bson.D{}
		for _, sortItem := range in.Sort {
			sort = append(sort, bson.E{Key: sortItem.Key, Value: sortItem.Direction})
		}
	} else {
		sort = bson.D{}
	}

	data, err := s.Service.List(ctx, filter, sort, int(in.Skip), int(in.Limit))

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	collections := model.ToPbCollections(data)
	return s.buildResponse(true, "Collections retrieved successfully", collections), nil
}

func (s *CollectionServiceServer) FindCollectionById(ctx context.Context, in *pb.FindCollectionRequest) (*pb.Response, error) {
	collection, success := s.getCachedCollection(ctx, in.Id)

	if !success {
		data, err := s.Service.Find(ctx, bson.M{"_id": in.Id})

		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Collection not found", nil), nil
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		collection = data

		// Set cache
		bytes, err := json.Marshal(collection)
		if err != nil {
			log.Printf("Error packing JSON: %s", err)
		} else {
			err = s.Cache.Set(ctx, "collection:"+in.Id, bytes, time.Hour).Err()
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
	}

	pbCollection := model.ToPbCollection(collection)
	return s.buildResponse(true, "Collection found", []*pb.Collection{pbCollection}), nil
}

func (s *CollectionServiceServer) AddCollection(ctx context.Context, in *pb.AddCollectionRequest) (*pb.Response, error) {
	currTime := time.Now().UTC().String()
	in.Collection.Id = primitive.NewObjectID().Hex()
	in.Collection.CreatedAt = currTime
	in.Collection.UpdatedAt = currTime

	// Check if collection already exists
	exists, err := s.checkIfExists(ctx, in.Collection.Name, in.Collection.Author)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if exists {
		return s.buildResponse(false, "Collection already exists", nil), nil
	}

	collection := model.FromPbCollection(in.Collection)
	err = s.Service.Create(ctx, *collection)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if in.Collection.TotalBooks > 0 {
		backgroundCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		go func() {
			defer cancel()

			var books []*pb.Book
			for range collection.TotalBooks {
				book := pb.Book{
					Id:           primitive.NewObjectID().Hex(),
					CollectionId: collection.Id.Hex(),
					IsBorrowed:   &wrapperspb.BoolValue{Value: false},
					CreatedAt:    time.Now().UTC().Format(time.RFC3339),
					UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
				}
				books = append(books, &book)
			}

			retries := 0
			for retries < 3 {
				if _, err := s.BookClient.BulkInsert(backgroundCtx, &pb.BulkInsertBookRequest{
					Books: books,
				}); err != nil {
					// Log error but don't fail the main operation
					log.Printf("Failed to bulk insert books for collection %s: %v", collection.Id, err)

					// Optional: implement retry logic or send to dead letter queue
					retries += 1
				} else {
					break
				}
			}
		}()
	}

	return s.buildResponse(true, "Collection added!", []*pb.Collection{in.Collection}), nil
}

func (s *CollectionServiceServer) UpdateCollection(ctx context.Context, in *pb.UpdateCollectionRequest) (*pb.Response, error) {
	update := in.Payload.AsMap()
	update["updated_at"] = time.Now().UTC().Format(time.RFC3339)

	filter := bson.M{}
	if name, ok := update["name"]; ok {
		filter["name"] = name.(string)
	}
	if author, ok := update["author"]; ok {
		filter["author"] = author.(string)
	}

	if len(filter) > 0 {
		// Check if updated title already exists
		found, err := s.Service.Find(ctx, filter)
		if !found.Id.IsZero() && found.Id.Hex() != in.Id {
			return nil, status.Error(codes.AlreadyExists, "Collection already exists!")
		} else if err != nil && err != mongo.ErrNoDocuments {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Update collection
	data, err := s.Service.Update(ctx, update, in.Id)
	if err == mongo.ErrNoDocuments {
		reply := s.buildResponse(false, "Collection not found", nil)
		return reply, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.invalidateCache(ctx, in.Id)

	dataPb := model.ToPbCollection(&data)
	if dataPb == nil {
		return nil, status.Error(codes.Internal, "Failed to convert collection to protobuf")
	}
	return s.buildResponse(true, "Collection updated!", []*pb.Collection{dataPb}), nil
}

func (s *CollectionServiceServer) DeleteCollection(ctx context.Context, in *pb.DeleteCollectionRequest) (*pb.Response, error) {
	data, err := s.Service.Delete(ctx, in.Id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Collection not found", nil), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.invalidateCache(ctx, in.Id)

	newCollection := model.ToPbCollection(&data)
	return s.buildResponse(true, "Collection deleted!", []*pb.Collection{newCollection}), nil
}

func (s *CollectionServiceServer) DecrementAvailableBooks(ctx context.Context, in *pb.DecrementAvailableBooksRequest) (*pb.Response, error) {
	result, err := s.Repository.UpdateBookStock(ctx, map[string]interface{}{"total_books": in.Amount}, in.Id)

	if err != nil {
		return s.buildResponse(false, err.Error(), []*pb.Collection{}), err
	}
	if result.(*mongo.UpdateResult).ModifiedCount == 0 {
		return s.buildResponse(false, "No book updated", []*pb.Collection{}), err
	}

	// Update cache
	cachedCollection, success := s.getCachedCollection(ctx, in.Id)
	if !success {
		log.Printf("Error getting cache")
	} else {
		cachedCollection.TotalBooks += int(in.Amount)

		bytes, err := json.Marshal(cachedCollection)
		if err != nil {
			log.Printf("Error packing JSON: %s", err)
			s.Cache.Del(ctx, "collection:"+in.Id)
		}

		err = s.Cache.Set(ctx, "collection:"+in.Id, bytes, time.Hour).Err()
		if err != nil {
			log.Printf("Error updating cache: %s", err)
			s.Cache.Del(ctx, "collection:"+in.Id)
		}
	}

	return s.buildResponse(true, "Stock updated successfully!", []*pb.Collection{}), nil
}

func (s *CollectionServiceServer) getCachedCollection(ctx context.Context, id string) (*model.Collection, bool) {
	collection, success := utils.GetCachedData[model.Collection](ctx, s.Cache, "collection:"+id)

	if !success {
		return nil, false
	}

	return collection, true
}

func (s *CollectionServiceServer) invalidateCache(ctx context.Context, id string) {
	// Invalidate cache
	err := s.Cache.Del(ctx, "collection:"+id).Err()
	if err != nil {
		log.Printf("Error deleting cache: %v", err)
	}
}

func (s *CollectionServiceServer) buildResponse(success bool, message string, collections []*pb.Collection) *pb.Response {
	return &pb.Response{
		Success:    success,
		Collection: collections,
		Message:    message,
	}
}

func (s *CollectionServiceServer) checkIfExists(ctx context.Context, name string, author string) (bool, error) {
	filter := map[string]interface{}{
		"name":   name,
		"author": author,
	}
	exists, err := s.Service.Exists(ctx, filter)
	if err != nil {
		return false, err
	}
	return exists, nil
}
