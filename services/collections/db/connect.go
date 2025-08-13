package db

import (
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func Connect() (*mongo.Client, *mongo.Database, error) {
	godotenv.Load(".env")

	clientOptions := options.Client()
	clientOptions.ApplyURI(os.Getenv("MONGODB_URI"))
	clientOptions.SetMaxPoolSize(10)
    clientOptions.SetMinPoolSize(3)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, nil, err
	}

	return client, client.Database("library_management_system"), nil
}
