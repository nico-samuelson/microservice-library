package main

import (
	"apigateway/internal/routes"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupGRPC() map[string]*grpc.ClientConn {
	godotenv.Load(".env")
	services := map[string]string{
		"collection": os.Getenv("COLLECTION_SERVICE_PORT"),
		"book":       os.Getenv("BOOK_SERVICE_PORT"),
		"borrow":     os.Getenv("BORROW_SERVICE_PORT"),
	}

	connections := make(map[string]*grpc.ClientConn)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	for service, port := range services {
		conn, err := grpc.NewClient("localhost:"+port, opts...)
		if err != nil {
			log.Fatalf("%s grpc server connection failed: %s", service, err)
		}
		connections[service] = conn
	}

	return connections
}

func closeConnections(connections map[string]*grpc.ClientConn) {
	for _, conn := range connections {
		conn.Close()
	}
}

func main() {
	// Create a channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Setup gRPC
	connections := setupGRPC()
	defer closeConnections(connections)

	// Setup Gin routes
	router := routes.SetupRoutes(connections, routes.DefaultBatchingConfig())

	// Start server in a goroutine
	srv := &http.Server{
		Addr:    "localhost:8080",
		Handler: router,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Println("Server started on localhost:8080")

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
