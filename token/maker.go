package token

import "time"

// Defines a contract for anything that can create and verify tokens.
//Allows you to swap out token implementations (e.g., switch from PASETO
// to something else) without changing the rest of your application logic.
// This promotes flexibility and testability.

type Maker interface {
	CreateToken(email string, duration time.Duration) (string, error)

	VerifyToken(token string) (*Payload, error)
}
