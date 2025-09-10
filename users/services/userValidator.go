package services

import (
	"regexp"
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"
)

func ValidateUser(user *models.User) string {
	if user.FirstName == "" {
		return "FirstName is required"
	}
	if user.LastName == "" {
		return "LastName is required"
	}
	if user.Email == "" {
		return "Email is required"
	}
	if user.Password == "" {
		return "Password is required"
	}
	if user.Phone == "" {
		return "Phone is required"
	}

	if user.Role != models.AdminRole && user.Role != models.SuperUserRole && user.Role != models.PropertyManagerRole {
		return "Invalid role"
	}
	return ""
}

func ValidatePassword(password string) string {
	if len(password) < 8 {
		return "Password must be at least 8 characters long"
	}

	var uppercase = regexp.MustCompile(`[A-Z]`)
	if !uppercase.MatchString(password) {
		return "Password must contain at least one uppercase letter"
	}

	var lowercase = regexp.MustCompile(`[a-z]`)
	if !lowercase.MatchString(password) {
		return "Password must contain at least one lowercase letter"
	}

	var digit = regexp.MustCompile(`[0-9]`)
	if !digit.MatchString(password) {
		return "Password must contain at least one digit"
	}

	var specialChar = regexp.MustCompile(`[!@#\$%\^&\*\(\)_\+\-=\[\]\{\};':"\\|,.<>\/?]+`)
	if !specialChar.MatchString(password) {
		return "Password must contain at least one special character"
	}

	return ""
}

func ValidateEmailFormat(email string) bool {
	var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return emailRegex.MatchString(email)
}

func IsEmailInDB(email string, repo repositories.UserRepository) bool {
	user, err := repo.GetUserByEmail(email)
	return err == nil && user != nil
}

func ValidateEmail(email string, repo repositories.UserRepository) string {
	if !ValidateEmailFormat(email) {
		return "Invalid email format"
	}
	if IsEmailInDB(email, repo) {
		return "Email already exists"
	}
	return ""
}
