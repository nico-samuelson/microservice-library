package main

import (
	"book/db"
	"book/internal"
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
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	svc := internal.NewBookService(database, "book")
	pb.RegisterBookServiceServer(s, svc)

	log.Printf("server listening at %v", lis.Addr())

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Book service started. Waiting for messages...")

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down book service...")

	s.GracefulStop()

	if err := client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from database: %v", err)
	}
	log.Println("Book service shut down gracefully")
}
