package utils

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// Initialize Redis client
var rdb = redis.NewClient(&redis.Options{
    Addr:     os.Getenv("REDIS_ADDRESS"), 
    Password: "",                         
    DB:       0,                          
})

// InvalidateCache will invalidate all cached keys for the given resource type
func InvalidateCache(resourceType string) error {
	// Use SCAN instead of KEYS for better performance in production
	pattern := fmt.Sprintf("%s:*", resourceType)
	iter := rdb.Scan(context.Background(), 0, pattern, 0).Iterator()
	
	// Iterate over matching keys
	for iter.Next(context.Background()) {
		key := iter.Val()
		err := rdb.Del(context.Background(), key).Err()
		if err != nil {
			return fmt.Errorf("failed to delete key %s: %v", key, err)
		}
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("error during SCAN iteration: %v", err)
	}

	return nil
}

// InvalidateCacheAsync invalidates the cache for a given resource type asynchronously
func InvalidateCacheAsync(resourceType string) {
	go func() {
		err := InvalidateCache(resourceType) // Call the original InvalidateCache
		if err != nil {
			// Log the error, but don't block the process
			log.Printf("Cache invalidation failed for resource type '%s': %v", resourceType, err)
		}
	}()
}
