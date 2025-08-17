package routes

import (
	"apigateway/internal/handler"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type BatchingConfig struct {
	CollectionBatchWindow time.Duration
	BookBatchWindow       time.Duration
	BorrowBatchWindow     time.Duration
	RateLimit             int
	RateLimitWindow       time.Duration
}

func DefaultBatchingConfig() *BatchingConfig {
	return &BatchingConfig{
		CollectionBatchWindow: 20 * time.Millisecond,
		BookBatchWindow:       20 * time.Millisecond,
		RateLimit:             100,
		RateLimitWindow:       1 * time.Minute,
	}
}

func SetupRoutes(
	connections map[string]*grpc.ClientConn,
	config *BatchingConfig,
) *gin.Engine {
	if config == nil {
		config = DefaultBatchingConfig()
	}

	collectionHandler := handler.NewCollectionHandlerWithBatching(
		connections["collection"],
		config.CollectionBatchWindow,
	)

	bookHandler := handler.NewBookHandlerWithBatching(
		connections["book"],
		config.BookBatchWindow,
	)

	borrowHandler := handler.NewBorrowHandler(
		connections["borrow"],
	)

	router := gin.Default()

	// Global middleware
	router.Use(RateLimitingMiddleware(config.RateLimit, config.RateLimitWindow))
	router.Use(CorsMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	v1 := router.Group("/api/v1")
	{
		collections := v1.Group("/collections")
		collections.Use(collectionHandler.BatchingMiddleware())
		{
			collections.GET("", collectionHandler.GetCollectionBatch)
			collections.GET("/:id", collectionHandler.GetCollectionById)
			collections.POST("", collectionHandler.CreateCollection)
			collections.PUT("/:id", collectionHandler.UpdateCollection)
			collections.DELETE("/:id", collectionHandler.DeleteCollection)
		}

		books := v1.Group("/books")
		books.Use(bookHandler.BatchingMiddleware())
		{
			books.GET("", bookHandler.GetBookBatch)
			books.GET("/:id", bookHandler.GetBookById)
			books.POST("", bookHandler.CreateBook)
			books.PUT("/:id", bookHandler.UpdateBook)
			books.DELETE("/:id", bookHandler.DeleteBook)
		}

		borrows := v1.Group("/borrow")
		{
			borrows.POST("", borrowHandler.BorrowBook)
			borrows.POST("/return", borrowHandler.ReturnBook)
		}
	}

	// Authentication routes (typically don't need batching)
	// auth := router.Group("/auth")
	// {
	// 	// auth.POST("/login", authHandler.Login)
	// 	// auth.POST("/register", authHandler.Register)
	// 	// auth.POST("/refresh", authHandler.RefreshToken)
	// }

	return router
}

// Additional middleware functions
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Rate limiting middleware (from your original code)
func RateLimitingMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	var (
		requestCounts = make(map[string]int)
		lastReset     = time.Now()
		mu            sync.RWMutex
	)

	return func(c *gin.Context) {
		now := time.Now()

		mu.Lock()
		if now.Sub(lastReset) > window {
			requestCounts = make(map[string]int)
			lastReset = now
		}

		clientIP := c.ClientIP()
		if requestCounts[clientIP] >= maxRequests {
			mu.Unlock()
			c.JSON(429, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}

		requestCounts[clientIP]++
		mu.Unlock()

		c.Next()
	}
}
