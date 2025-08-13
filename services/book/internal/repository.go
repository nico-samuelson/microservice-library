package internal

import (
	"shared/pkg/model"
	"shared/pkg/repository"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type BookRepository struct {
	Repository repository.BaseRepository[model.Book]
}

func NewBookRepository(database *mongo.Database, collection_name string) *BookRepository {
	return &BookRepository{
		Repository: *repository.NewRepository[model.Book](database, "book"),
	}
}
