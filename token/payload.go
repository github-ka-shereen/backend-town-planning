package token

import (
	"errors"
	"fmt"
	"time"
	"town-planning-backend/utils"

	"github.com/google/uuid"
)

var ErrExpired = errors.New("token has expired")

type Payload struct {
	ID        uuid.UUID `json:"id"`         // Token ID
	UserID    uuid.UUID `json:"user_id"`    // User identifier
	IssuedAt  time.Time `json:"issued_at"`
	ExpiredAt time.Time `json:"expired_at"`
}

func NewPayload(userID uuid.UUID, duration time.Duration) (*Payload, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user ID cannot be empty")
	}
	if duration <= 0 {
		return nil, errors.New("duration must be positive")
	}

	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	// Use utils.DateLocation to convert to the app's timezone
	issuedAt := time.Now().In(utils.DateLocation)
	expiredAt := issuedAt.Add(duration)

	payload := &Payload{
		ID:        tokenID,
		UserID:    userID,
		IssuedAt:  issuedAt,
		ExpiredAt: expiredAt,
	}
	return payload, nil
}

func (payload *Payload) Valid() error {
	// Compare against current time in the app's timezone
	if time.Now().In(utils.DateLocation).After(payload.ExpiredAt) {
		return ErrExpired
	}
	return nil
}

func (p *Payload) String() string {
	return fmt.Sprintf("ID: %s, UserID: %s, IssuedAt: %s, ExpiredAt: %s", 
		p.ID, p.UserID, p.IssuedAt, p.ExpiredAt)
}