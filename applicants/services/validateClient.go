package services

import (
	"town-planning-backend/db/models"
	"errors"
	"regexp"
	"strings"
)

func ValidateApplicant(applicant *models.Applicant) string {
	// Validate client type
	if applicant.ApplicantType == "" {
		return "Client type is required"
	}

	if applicant.ApplicantType == models.OrganisationApplicant {
		if applicant.OrganisationName == nil || *applicant.OrganisationName == "" {
			return "Company name is required for company clients."
		}
		if len(applicant.OrganisationRepresentatives) == 0 {
			return "At least one representative is required for a company client."
		}
	}

	if applicant.ApplicantType == models.IndividualApplicant {
		if applicant.FirstName == nil || applicant.LastName == nil {
			return "First and last name are required for individual clients."
		}
	}

	// Validate first name (only for individuals)
	if applicant.ApplicantType == models.IndividualApplicant {
		if applicant.FirstName == nil || strings.TrimSpace(*applicant.FirstName) == "" {
			return "First name is required for individual clients"
		}
		if applicant.LastName == nil || strings.TrimSpace(*applicant.LastName) == "" {
			return "Last name is required for individual clients"
		}
	}

	// Validate company name (only for companies)
	if applicant.ApplicantType == models.OrganisationApplicant {
		if applicant.OrganisationName == nil || strings.TrimSpace(*applicant.OrganisationName) == "" {
			return "Company name is required for company clients"
		}
	}

	phoneRegex := regexp.MustCompile(`^\+\d{9,15}$`)
	if !phoneRegex.MatchString(applicant.PhoneNumber) {
		return "Phone number must start with '+' followed by 9 to 15 digits"
	}

	return ""
}

// IsValidStatus validates if the given status is a valid client status.
func IsValidStatus(status string) error {
	validStatuses := []models.ApplicantStatus{
		models.ProspectiveApplicant,
		models.ActiveApplicant,
		models.InactiveApplicant,
	}

	// Check if the status exists in the list of valid statuses
	for _, validStatus := range validStatuses {
		if status == string(validStatus) {
			return nil // Return nil if status is valid
		}
	}

	// If the status is not valid, return an error
	return errors.New("invalid status")
}
