package model

import (
	"log"
	pb "shared/proto/buffer"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Book struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CollectionId primitive.ObjectID `bson:"collection_id" json:"collection_id" validate:"required"`
	IsBorrowed   bool               `bson:"is_borrowed" json:"is_borrowed" validate:"required"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at" validate:"required"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at" validate:"required"`
}

type BookUpdateRequest struct {
	CollectionId *primitive.ObjectID `json:"collection_id,omitempty" validate:"omitempty,required"`
	IsBorrowed   *bool               `json:"is_borrowed,omitempty" validate:"omitempty,required"`
}

func NewBook() Book {
	book := Book{
		Id:           primitive.NewObjectID(),
		CollectionId: primitive.NewObjectID(),
		IsBorrowed:   false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return book
}

func ToPbBook(c *Book) *pb.Book {
	if c == nil {
		return nil
	}

	return &pb.Book{
		Id:           c.Id.Hex(),
		CollectionId: c.CollectionId.Hex(),
		IsBorrowed:   wrapperspb.Bool(c.IsBorrowed),
		CreatedAt:    c.CreatedAt.Format("2006-01-02T15:04:05.000000Z"),
		UpdatedAt:    c.UpdatedAt.Format("2006-01-02T15:04:05.000000Z"),
	}
}

func FromPbBook(p *pb.Book) *Book {
	if p == nil {
		return nil
	}

	objId, err := primitive.ObjectIDFromHex(p.Id)
	if err != nil {
		log.Printf("Failed to convert ID from hex: %v", err)
		return nil
	}

	collectionId, err := primitive.ObjectIDFromHex(p.CollectionId)
	if err != nil {
		log.Printf("Failed to convert ID from hex: %v", err)
		return nil
	}

	parsedCreatedTime, err := time.Parse("2006-01-02T15:04:05.000000Z", p.CreatedAt)
	if err != nil {
		log.Printf("Failed to parse time: %v", err)
		return nil
	}

	parsedUpdatedTime, err := time.Parse("2006-01-02T15:04:05.000000Z", p.UpdatedAt)
	if err != nil {
		log.Printf("Failed to parse time: %v", err)
		return nil
	}

	return &Book{
		Id:           objId,
		CollectionId: collectionId,
		IsBorrowed:   p.IsBorrowed.Value,
		CreatedAt:    parsedCreatedTime,
		UpdatedAt:    parsedUpdatedTime,
	}
}

func FromPbBooks(pBooks []*pb.Book) []*Book {
	if pBooks == nil {
		return nil
	}

	books := make([]*Book, len(pBooks))
	for i, p := range pBooks {
		books[i] = FromPbBook(p)
	}
	return books
}

func ToPbBooks(models []Book) []*pb.Book {
	result := make([]*pb.Book, len(models))
	for i, m := range models {
		result[i] = ToPbBook(&m)
	}
	return result
}
