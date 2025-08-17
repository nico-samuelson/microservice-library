package db

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

func Connect() (*mongo.Client, *mongo.Database, error) {
	godotenv.Load(".env")

	clientOptions := options.Client()
	clientOptions.ApplyURI(os.Getenv("MONGODB_URI"))
	clientOptions.SetMaxPoolSize(100)
	clientOptions.SetMinPoolSize(25)
	clientOptions.SetWriteConcern(writeconcern.W1())

	// Add connection timeouts
	clientOptions.SetMaxConnIdleTime(30 * time.Second)
	clientOptions.SetConnectTimeout(5 * time.Second)
	clientOptions.SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	return client, client.Database("library_management_system"), nil
}
