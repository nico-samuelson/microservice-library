package internal

import (
	"book/internal/db"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"shared/config"
	pb "shared/proto/buffer"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Setup() {
	godotenv.Load(".env")

	// Setup database connection
	client, database, err := db.Connect()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	// Dial other services
	connections := DialClients()
	defer CloseClientConnections(connections)

	// Setup Redis client
	rdb, err := StartRedisClient(config.LoadRedisConfig())
	if err != nil {
		log.Fatalf("failed to start Redis client: %v", err)
	}

	// Setup gRPC server
	server, err := StartServer(database, connections, rdb)
	if err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}

	// Setup signal handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Book service started. Waiting for messages...")

	// Wait for shutdown signal
	<-quit
	log.Println("Shutting down book service...")

	// Stop services
	server.GracefulStop()
	if err := rdb.Close(); err != nil {
		log.Printf("Error closing Redis client: %v", err)
	}
	if err := client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from database: %v", err)
	}

	log.Println("Book service shut down gracefully")
}

func DialClients() map[string]*grpc.ClientConn {
	services := map[string]string{
		"collection": os.Getenv("COLLECTION_SERVICE_PORT"),
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

func CloseClientConnections(connections map[string]*grpc.ClientConn) {
	for _, conn := range connections {
		conn.Close()
	}
}

func StartServer(database *mongo.Database, connections map[string]*grpc.ClientConn, redis *redis.Client) (*grpc.Server, error) {
	godotenv.Load(".env")
	log.Println(os.Getenv("BOOK_SERVICE_PORT"))
	lis, err := net.Listen("tcp", ":"+os.Getenv("BOOK_SERVICE_PORT"))
	if err != nil {
		log.Printf("Error listening on port %s: %v", os.Getenv("BOOK_SERVICE_PORT"), err)
	}

	s := grpc.NewServer()
	svc := NewBookService(database, "book", connections, redis)
	pb.RegisterBookServiceServer(s, svc)

	log.Printf("server listening at %v", lis.Addr())

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	if err != nil {
		return nil, err
	}
	return s, nil
}

func StartRedisClient(cfg *config.RedisConfig) (*redis.Client, error) {
	options := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	}
	rdb := redis.NewClient(options)

	// Test connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	// Setup cache configuration
	if err := SetupRedisCache(rdb, config.CacheConfig{
		MaxMemory: "256mb",
		Policy:    "allkeys-lru",
	}); err != nil {
		return nil, err
	}

	return rdb, nil
}

func SetupRedisCache(client *redis.Client, config config.CacheConfig) error {
	ctx := context.Background()

	// Set maximum memory
	if config.MaxMemory != "" {
		err := client.ConfigSet(ctx, "maxmemory", config.MaxMemory).Err()
		if err != nil {
			return fmt.Errorf("failed to set maxmemory: %w", err)
		}
		log.Printf("Set Redis max memory to: %s", config.MaxMemory)
	}

	// Set eviction policy
	if config.Policy != "" {
		err := client.ConfigSet(ctx, "maxmemory-policy", config.Policy).Err()
		if err != nil {
			return fmt.Errorf("failed to set maxmemory-policy: %w", err)
		}
		log.Printf("Set Redis eviction policy to: %s", config.Policy)
	}

	// Verify configuration
	return VerifyConfig(client)
}

func VerifyConfig(client *redis.Client) error {
	ctx := context.Background()

	// Get current memory settings
	maxMem, err := client.ConfigGet(ctx, "maxmemory").Result()
	if err != nil {
		return fmt.Errorf("failed to get maxmemory config: %w", err)
	}

	policy, err := client.ConfigGet(ctx, "maxmemory-policy").Result()
	if err != nil {
		return fmt.Errorf("failed to get maxmemory-policy config: %w", err)
	}

	log.Printf("Current Redis configuration:")
	log.Printf("  Max Memory: %v", maxMem)
	log.Printf("  Eviction Policy: %v", policy)

	return nil
}

// Cache warming function
func WarmCache(ctx context.Context, client *redis.Client) error {
	pipe := client.Pipeline()

	warmData := map[string]interface{}{
		"config:app": `{"theme":"dark","lang":"en"}`,
	}

	for key, value := range warmData {
		pipe.Set(ctx, key, value, time.Hour) // 1 hour default TTL
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to warm cache: %w", err)
	}

	log.Printf("Warmed cache with %d keys", len(warmData))
	return nil
}
