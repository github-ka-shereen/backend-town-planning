package utils

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StringToUUIDPtr converts a string to UUID pointer
func StringToUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &u
}

// StringPtr returns a pointer to the string value
func StringPtr(s string) *string {
	return &s
}

// FormatWaitingListNumber formats a sequence number into a waiting list number
func FormatWaitingListNumber(sequence int) string {
	year := time.Now().Year()
	return fmt.Sprintf("WL-%d-%05d", year, sequence)
}
