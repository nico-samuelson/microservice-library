package utils

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

func GetCachedData[K any](ctx context.Context, cache *redis.Client, key string) (*K, bool) {
	data, err := cache.Get(ctx, key).Result()
	if err != nil {
		log.Printf("Error getting cache: %v", err)
		log.Println("Cache miss")
		return nil, false
	}

	var obj K
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		log.Printf("Error unpacking cache data: %v", err)
		return nil, false
	}

	return &obj, true
}
