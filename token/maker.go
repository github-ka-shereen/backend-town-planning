package token

import (
	"time"
	
	"github.com/google/uuid"
)

// Maker defines a contract for anything that can create and verify tokens.
// Allows you to swap out token implementations (e.g., switch from PASETO
// to something else) without changing the rest of your application logic.
// This promotes flexibility and testability.

type Maker interface {
	CreateToken(userID uuid.UUID, duration time.Duration) (string, error)
	VerifyToken(token string) (*Payload, error)
}