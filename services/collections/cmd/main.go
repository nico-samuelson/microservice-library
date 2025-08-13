package main

import (
	"collection/db"
	"collection/internal"
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	pb "shared/proto/buffer"
	"syscall"

	"google.golang.org/grpc"
)

func main() {
	// Database Connection
	client, database, err := db.Connect()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	svc := internal.NewCollectionService(database, "collections")
	pb.RegisterCollectionServiceServer(s, svc)

	log.Printf("server listening at %v", lis.Addr())

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Collection service started. Waiting for messages...")

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down collection service...")

	s.GracefulStop()

	if err := client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from database: %v", err)
	}
	log.Println("Collection service shut down gracefully")
}
