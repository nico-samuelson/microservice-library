package internal

import (
	"context"
	"log"
	"time"

	"shared/pkg/model"
	"shared/pkg/service"
	pb "shared/proto/buffer"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CollectionServiceServer struct {
	pb.UnimplementedCollectionServiceServer
	service    *service.BaseService[model.Collection, model.CollectionUpdateRequest]
	repository *CollectionRepository
}

func NewCollectionService(database *mongo.Database, collection_name string) *CollectionServiceServer {
	repository := NewCollectionRepository(database, collection_name)
	return &CollectionServiceServer{
		service:    service.NewBaseService[model.Collection, model.CollectionUpdateRequest](&repository.Repository),
		repository: repository,
	}
}

func (s *CollectionServiceServer) GetCollection(ctx context.Context, in *pb.GetCollectionRequest) (*pb.Response, error) {
	data, err := s.service.List(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	collections := model.ToPbCollections(data)
	return s.buildResponse(true, "Collections retrieved successfully", collections), nil
}

func (s *CollectionServiceServer) FindCollectionById(ctx context.Context, in *pb.FindCollectionRequest) (*pb.Response, error) {
	data, err := s.service.Find(ctx, bson.M{"_id": in.Id})

	if err == mongo.ErrNoDocuments {
		return s.buildResponse(false, "Collection not found", nil), nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	newCollection := model.ToPbCollection(data)
	return s.buildResponse(true, "Collection found", []*pb.Collection{newCollection}), nil
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
	err = s.service.Create(ctx, *collection)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.buildResponse(true, "Collection added!", []*pb.Collection{in.Collection}), nil
}

func (s *CollectionServiceServer) UpdateCollection(ctx context.Context, in *pb.UpdateCollectionRequest) (*pb.Response, error) {
	update := in.Payload.AsMap()
	update["updated_at"] = time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")

	filter := bson.M{}
	if name, ok := update["name"]; ok {
		filter["name"] = name.(string)
	}
	if author, ok := update["author"]; ok {
		filter["author"] = author.(string)
	}

	if len(filter) > 0 {
		// Check if updated title already exists
		found, err := s.service.Find(ctx, filter)
		if !found.Id.IsZero() && found.Id.Hex() != in.Id {
			return nil, status.Error(codes.AlreadyExists, "Collection already exists!")
		} else if err != nil && err != mongo.ErrNoDocuments {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Update collection
	data, err := s.service.Update(ctx, update, in.Id)
	if err == mongo.ErrNoDocuments {
		reply := s.buildResponse(false, "Collection not found", nil)
		return reply, nil
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	dataPb := model.ToPbCollection(&data)
	if dataPb == nil {
		return nil, status.Error(codes.Internal, "Failed to convert collection to protobuf")
	}
	return s.buildResponse(true, "Collection updated!", []*pb.Collection{dataPb}), nil

}

func (s *CollectionServiceServer) DeleteCollection(ctx context.Context, in *pb.DeleteCollectionRequest) (*pb.Response, error) {
	data, err := s.service.Delete(ctx, in.Id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return s.buildResponse(false, "Collection not found", nil), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	newCollection := model.ToPbCollection(&data)
	return s.buildResponse(true, "Collection deleted!", []*pb.Collection{newCollection}), nil
}

func (s *CollectionServiceServer) DecrementAvailableBooks(ctx context.Context, in *pb.DecrementAvailableBooksRequest) (*pb.Response, error) {
	log.Println(in)
	result, err := s.repository.UpdateBookStock(ctx, map[string]interface{}{"available_books": in.Amount}, in.Id)

	if err != nil {
		return s.buildResponse(false, err.Error(), []*pb.Collection{}), err
	}
	if result.ModifiedCount == 0 {
		return s.buildResponse(false, "No book updated", []*pb.Collection{}), err
	}

	return s.buildResponse(true, "Stock updated successfully!", []*pb.Collection{}), nil
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
	exists, err := s.service.Exists(ctx, filter)
	if err != nil {
		return false, err
	}
	return exists, nil
}
