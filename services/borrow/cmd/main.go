package main

import (
	"borrow/db"
	"borrow/internal"
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	pb "shared/proto/buffer"
	"syscall"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupGRPC() map[string]*grpc.ClientConn {
	godotenv.Load(".env")
	services := map[string]string{
		"collection": os.Getenv("COLLECTION_SERVICE_PORT"),
		"book":       os.Getenv("BOOK_SERVICE_PORT"),
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
	// Database Connection
	client, database, err := db.Connect()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	// Setup gRPC Client
	connections := setupGRPC()
	defer closeConnections(connections)

	// Setup gRPC Server
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	svc := internal.NewBorrowService(database, "borrow_history", connections)
	pb.RegisterBorrowServiceServer(s, svc)

	log.Printf("server listening at %v", lis.Addr())

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Borrow service started. Waiting for messages...")

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down borrow service...")

	s.GracefulStop()

	if err := client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from database: %v", err)
	}
	log.Println("Borrow service shut down gracefully")
}
