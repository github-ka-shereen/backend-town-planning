package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	// "log"

	// "sort"

	"github.com/redis/go-redis/v9"
)

// GenerateHash generates two hashes: one for searching (without timestamp) and one for storage (with timestamp)
func GenerateHash(resourceType string, filters map[string]string, page, pageSize int) (string, string) {
	// Get the current timestamp in seconds for uniqueness in storage key
	timestamp := Today().Unix()

	// Create a sorted list of query parameters for both search and storage
	query := fmt.Sprintf("resource=%s&page=%d&page_size=%d", resourceType, page, pageSize)
	for key, value := range filters {
		query += fmt.Sprintf("&%s=%s", key, value)
	}

	// Generate search hash (without timestamp)
	searchHash := sha256.New()
	searchHash.Write([]byte(query))
	searchHashStr := hex.EncodeToString(searchHash.Sum(nil))

	// Generate storage hash (with timestamp)
	storageQuery := fmt.Sprintf("%s&timestamp=%d", query, timestamp)
	storageHash := sha256.New()
	storageHash.Write([]byte(storageQuery))
	storageHashStr := hex.EncodeToString(storageHash.Sum(nil))

	// Add resourceType prefix with a colon separator for both search and storage keys
	searchKey := fmt.Sprintf("%s:%s", resourceType, searchHashStr)
	storageKey := fmt.Sprintf("%s:%s", resourceType, storageHashStr)

	return searchKey, storageKey
}

func FindMatchingFile(rdb *redis.Client, searchHash string) (string, error) {
	// Use SCAN instead of KEYS for better performance in production
	iter := rdb.Scan(context.Background(), 0, fmt.Sprintf("*%s*", searchHash), 1).Iterator()
	for iter.Next(context.Background()) {
		// Get the first matching key's value
		filePath, err := rdb.Get(context.Background(), iter.Val()).Result()
		if err == nil {
			// log.Printf("Found cached file in Redis: %s", filePath)
			return filePath, nil
		}
	}
	if err := iter.Err(); err != nil {
		return "", err
	}

	// log.Printf("No file found in Redis for searchHash: %s", searchHash)
	return "", redis.Nil
}

// func GenerateHash(resourceType string, filters map[string]string) string {
// 	query := fmt.Sprintf("resource=%s", resourceType)
// 	for key, value := range filters {
// 		query += fmt.Sprintf("&%s=%s", key, value)
// 	}

// 	hash := sha256.New()
// 	hash.Write([]byte(query))
// 	hashStr := hex.EncodeToString(hash.Sum(nil))

// 	return fmt.Sprintf("%s:%s", resourceType, hashStr)
// }

// func FindMatchingFile(rdb *redis.Client, searchHash string) (string, error) {
//     filePath, err := rdb.Get(context.Background(), searchHash).Result()
//     if err == redis.Nil {
//         return "", nil
//     }
//     if err != nil {
//         return "", err
//     }
//     return filePath, nil
// }
