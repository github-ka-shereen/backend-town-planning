package controllers

import (
	"town-planning-backend/db/models"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
)

type AuthPreferencesController struct {
	authPrefService *services.AuthPreferencesService
	userRepo        repositories.UserRepository
}

func NewAuthPreferencesController(
	authPrefService *services.AuthPreferencesService,
	userRepo repositories.UserRepository,
) *AuthPreferencesController {
	return &AuthPreferencesController{
		authPrefService: authPrefService,
		userRepo:        userRepo,
	}
}

// SetAuthMethod updates the user's preferred authentication method
func (apc *AuthPreferencesController) SetAuthMethod(c *fiber.Ctx) error {
	type Request struct {
		UserID string `json:"user_id"`
		Method string `json:"method"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	// Validate the requested method
	switch req.Method {
	case string(models.AuthMethodMagicLink), string(models.AuthMethodPassword), string(models.AuthMethodAuthenticator):
		// Valid method
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid authentication method",
			"error":   "method must be one of: magic_link, password, authenticator",
		})
	}

	// Check if user can use the requested method
	user, err := apc.userRepo.GetUserByID(req.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
			"error":   err.Error(),
		})
	}

	if req.Method == string(models.AuthMethodPassword) && user.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Password not set",
			"error":   "user must set a password before enabling password authentication",
		})
	}

	if req.Method == string(models.AuthMethodAuthenticator) && user.TOTPSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Authenticator not set up",
			"error":   "user must set up authenticator before enabling this method",
		})
	}

	// Update auth method
	err = apc.authPrefService.SetAuthMethod(req.UserID, req.Method)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to update authentication method",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Authentication method updated successfully",
		"data": fiber.Map{
			"auth_method": req.Method,
		},
		"error": nil,
	})
}

// GetAuthMethods returns the available and current auth methods for a user
func (apc *AuthPreferencesController) GetAuthMethods(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	currentMethod, err := apc.authPrefService.GetAuthMethod(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get current auth method",
			"error":   err.Error(),
		})
	}

	canUsePassword, err := apc.authPrefService.CanUsePassword(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to check password availability",
			"error":   err.Error(),
		})
	}

	canUseAuthenticator, err := apc.authPrefService.CanUseAuthenticator(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to check authenticator availability",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Auth methods retrieved",
		"data": fiber.Map{
			"current_method": currentMethod,
			"available_methods": fiber.Map{
				"magic_link":    true, // Always available
				"password":      canUsePassword,
				"authenticator": canUseAuthenticator,
			},
		},
		"error": nil,
	})
}
