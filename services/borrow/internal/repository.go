package internal

import (
	"shared/pkg/model"
	"shared/pkg/repository"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type BorrowRepository struct {
	Repository repository.BaseRepository[model.Borrow]
}

func NewBorrowRepository(database *mongo.Database, collection_name string) *BorrowRepository {
	return &BorrowRepository{
		Repository: *repository.NewRepository[model.Borrow](database, "borrow_history"),
	}
}
