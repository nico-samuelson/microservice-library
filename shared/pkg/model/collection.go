package model

import (
	"log"
	pb "shared/proto/buffer"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Collection struct {
	Id             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name           string             `bson:"name" json:"name" validate:"required"`
	Author         string             `bson:"author" json:"author" validate:"required"`
	Categories     []string           `bson:"categories" json:"categories" validate:"required,min=1,dive,min=1,max=50"`
	TotalBooks     int                `bson:"total_books" json:"total_books" validate:"gte=0"`
	AvailableBooks int                `bson:"available_books" json:"available_books" validate:"gte=0"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at" validate:"required"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at" validate:"required"`
}

type CollectionUpdateRequest struct {
	Name           *string   `json:"name" validate:"omitempty,min=1,max=200"`
	Author         *string   `json:"author" validate:"omitempty,min=1,max=100"`
	Categories     *[]string `json:"categories" validate:"omitempty,min=1,dive,min=1,max=50"`
	TotalBooks     *int      `json:"total_books" validate:"omitempty,gte=0"`
	AvailableBooks *int      `json:"available_books" validate:"omitempty,gte=0"`
}

func NewCollection() Collection {
	collection := Collection{
		Id:             primitive.NewObjectID(),
		Name:           "",
		Author:         "",
		Categories:     []string{},
		TotalBooks:     0,
		AvailableBooks: 0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	return collection
}

func ToPbCollection(c *Collection) *pb.Collection {
	if c == nil {
		return nil
	}

	return &pb.Collection{
		Id:             c.Id.Hex(),
		Name:           c.Name,
		Author:         c.Author,
		Categories:     c.Categories,
		TotalBooks:     int32(c.TotalBooks),
		AvailableBooks: int32(c.AvailableBooks),
		CreatedAt:      c.CreatedAt.UTC().String(),
		UpdatedAt:      c.UpdatedAt.UTC().String(),
	}
}

func FromPbCollection(p *pb.Collection) *Collection {
	if p == nil {
		return nil
	}

	objId, err := primitive.ObjectIDFromHex(p.Id)
	if err != nil {
		log.Printf("Failed to convert ID from hex: %v", err)
		return nil
	}

	parsedCreatedTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", p.CreatedAt)
	if err != nil {
		log.Printf("Failed to parse time: %v", err)
		return nil
	}

	parsedUpdatedTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", p.UpdatedAt)
	if err != nil {
		log.Printf("Failed to parse time: %v", err)
		return nil
	}

	return &Collection{
		Id:             objId,
		Name:           p.Name,
		Author:         p.Author,
		Categories:     p.Categories,
		TotalBooks:     int(p.TotalBooks),
		AvailableBooks: int(p.AvailableBooks),
		CreatedAt:      parsedCreatedTime,
		UpdatedAt:      parsedUpdatedTime,
	}
}

func FromPbCollections(pCollections []*pb.Collection) []*Collection {
	if pCollections == nil {
		return nil
	}

	collections := make([]*Collection, len(pCollections))
	for i, p := range pCollections {
		collections[i] = FromPbCollection(p)
	}
	return collections
}

func ToPbCollections(models []Collection) []*pb.Collection {
	result := make([]*pb.Collection, len(models))
	for i, m := range models {
		result[i] = ToPbCollection(&m)
	}
	return result
}
