package requests

import (
	"town-planning-backend/db/models"

	"github.com/google/uuid"
)

// Request types
type ParticipantRequest struct {
	UserID uuid.UUID              `json:"user_id"`
	Role   models.ParticipantRole `json:"role,omitempty"`
}

type UnifiedParticipantRequest struct {
	// For single operations
	UserID uuid.UUID              `json:"user_id,omitempty"`
	Role   models.ParticipantRole `json:"role,omitempty"`

	// For bulk operations
	Participants []ParticipantRequest `json:"participants,omitempty"`
	UserIDs      []uuid.UUID          `json:"user_ids,omitempty"`

	// Operation type
	Operation string `json:"operation,omitempty"` // "add_single", "add_bulk", "remove_single", "remove_bulk"
}
