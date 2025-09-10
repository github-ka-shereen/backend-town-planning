package middleware

import (
	"context"
	"town-planning-backend/token"

	"github.com/redis/go-redis/v9"
)

// AppContext bundles all dependencies
type AppContext struct {
	PasetoMaker token.Maker
	Ctx         context.Context
	RedisClient *redis.Client
}
