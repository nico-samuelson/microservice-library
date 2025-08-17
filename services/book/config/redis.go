package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type RedisConfig struct {
	Addr         string        `json:"addr"`
	Password     string        `json:"password"`
	DB           int           `json:"db"`
	PoolSize     int           `json:"pool_size"`
	MinIdleConns int           `json:"min_idle_conns"`
	MaxRetries   int           `json:"max_retries"`
	DialTimeout  time.Duration `json:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	PoolTimeout  time.Duration `json:"pool_timeout"`
}

type CacheConfig struct {
	MaxMemory string // e.g., "256mb", "1gb"
	Policy    string // eviction policy
}

// Default configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}
}

// Load configuration from environment or file
func LoadRedisConfig() *RedisConfig {
	godotenv.Load(".env")
	config := DefaultRedisConfig()

	// Override with environment variables if present
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		config.Addr = addr
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Password = password
	}

	return config
}
