package config

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func InitRedisServer(ctx context.Context) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     GetEnv("REDIS_ADDRESS"),
		Password: "",
		DB:       0,
	})

	_, err := client.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}

	return client
}
