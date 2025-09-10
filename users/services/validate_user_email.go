package services

import (
	"town-planning-backend/users/repositories"
)

// ValidateUpdatedEmail checks if the provided email is valid and unique
// for updating a user, excluding the user with the given ID.
func ValidateUpdatedEmail(email string, userRepo repositories.UserRepository, userID string) string {
	if email == "" {
		return "Email cannot be empty"
	}
	if len(email) > 255 {
		return "Email is too long"
	}
	// Basic email format validation (you might want to use a more robust regex)
	if !isValidEmailFormat(email) {
		return "Invalid email format"
	}

	existingUser, err := userRepo.GetUserByEmail(email)
	if err == nil && existingUser != nil && existingUser.ID.String() != userID {
		return "Email already exists"
	}
	return "" // Email is valid and unique (or belongs to the same user)
}

// isValidEmailFormat performs a basic check for email format.
// You might want to use a more comprehensive regular expression for better validation.
func isValidEmailFormat(email string) bool {
	if len(email) < 3 || len(email) > 255 {
		return false
	}
	atCount := 0
	dotCount := 0
	lastAt := -1
	lastDot := -1

	for i, char := range email {
		switch char {
		case '@':
			atCount++
			lastAt = i
		case '.':
			dotCount++
			lastDot = i
		}
	}

	if atCount != 1 || lastAt < 1 || lastAt > len(email)-5 || lastDot <= lastAt || lastDot == len(email)-1 {
		return false
	}

	return true
}
