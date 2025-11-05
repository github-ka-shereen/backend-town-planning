package controllers

import (
	"context"
	"time"
	"town-planning-backend/config"
	"town-planning-backend/db/models"
	"town-planning-backend/token"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"
	"town-planning-backend/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type EnhancedLoginController struct {
	userRepo         repositories.UserRepository
	pasetoMaker      token.Maker
	ctx              context.Context
	redisClient      *redis.Client
	magicLinkService *services.MagicLinkService
	otpService       services.OtpService // Changed from pointer to interface
	authPrefService  *services.AuthPreferencesService
	deviceService    *services.DeviceService
	baseFrontendURL  string
}

func NewEnhancedLoginController(
	userRepo repositories.UserRepository,
	pasetoMaker token.Maker,
	ctx context.Context,
	redisClient *redis.Client,
	magicLinkService *services.MagicLinkService,
	otpService services.OtpService,
	authPrefService *services.AuthPreferencesService,
	deviceService *services.DeviceService,
	baseFrontendURL string,
) *EnhancedLoginController {
	return &EnhancedLoginController{
		userRepo:         userRepo,
		pasetoMaker:      pasetoMaker,
		ctx:              ctx,
		redisClient:      redisClient,
		magicLinkService: magicLinkService,
		otpService:       otpService,
		authPrefService:  authPrefService,
		deviceService:    deviceService,
		baseFrontendURL:  baseFrontendURL,
	}
}

// InitiateLogin handles the initial login request
func (elc *EnhancedLoginController) InitiateLogin(c *fiber.Ctx) error {
	type LoginRequest struct {
		Email             string                     `json:"email"`
		Password          string                     `json:"password,omitempty"`
		DeviceFingerprint services.DeviceFingerprint `json:"device_fingerprint"`
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Invalid request format", err)
	}

	user, err := elc.userRepo.GetUserByEmail(req.Email)
	if err != nil {
		// Don't reveal if user exists for security
		return elc.sendSuccessResponse(c, "If an account exists, login instructions have been sent")
	}

	// Check user's preferred auth method
	authMethod, err := elc.authPrefService.GetAuthMethod(user.ID.String())
	if err != nil {
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to get auth preferences", err)
	}

	switch authMethod {
	case string(models.AuthMethodMagicLink):
		return elc.handleMagicLinkLogin(c, user, req.DeviceFingerprint)
	case string(models.AuthMethodPassword):
		return elc.handlePasswordLogin(c, user, req.Password, req.DeviceFingerprint)
	case string(models.AuthMethodAuthenticator):
		return elc.handleAuthenticatorLogin(c, user, req.DeviceFingerprint)
	default:
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Invalid authentication method configured", nil)
	}
}

// handleMagicLinkLogin processes magic link authentication
func (elc *EnhancedLoginController) handleMagicLinkLogin(c *fiber.Ctx, user *models.User, deviceFingerprint services.DeviceFingerprint) error {
	magicLink, err := elc.magicLinkService.GenerateMagicLink(user.ID.String(), user.Email, deviceFingerprint)
	if err != nil {
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate magic link", err)
	}

	// Send styled magic link email
	if err := utils.SendMagicLinkEmail(user.Email, magicLink.URL, "15 minutes"); err != nil {
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to send magic link email", err)
	}

	return elc.sendSuccessResponse(c, "Magic link sent to your email", fiber.Map{
		"auth_method": models.AuthMethodMagicLink,
		"expires_at":  magicLink.ExpiresAt,
	})
}

// handlePasswordLogin processes password authentication
func (elc *EnhancedLoginController) handlePasswordLogin(c *fiber.Ctx, user *models.User, password string, deviceFingerprint services.DeviceFingerprint) error {
	if password == "" {
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Password is required", nil)
	}

	if !services.CheckPasswordHash(password, user.Password) {
		return elc.sendErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials", nil)
	}

	// Check if device is trusted
	isTrusted, _, _ := elc.deviceService.IsDeviceTrusted(user.ID.String(), deviceFingerprint)

	// If untrusted device or TOTP enabled, require additional verification
	if !isTrusted || elc.otpService.IsTOTPEnabled(user.ID.String()) {
		return elc.initiateTwoFactor(c, user, deviceFingerprint, isTrusted)
	}

	return elc.completeLogin(c, user, deviceFingerprint, true)
}

// VerifyOtp handles OTP verification for email-based 2FA
func (elc *EnhancedLoginController) VerifyOtp(c *fiber.Ctx) error {
	type VerifyOtpRequest struct {
		UserID      string `json:"user_id"`
		Otp         string `json:"otp"`
		PreToken    string `json:"pre_token"`
		TrustDevice bool   `json:"trust_device,omitempty"`
	}

	var req VerifyOtpRequest
	if err := c.BodyParser(&req); err != nil {
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Invalid request format", err)
	}

	// Validate OTP
	valid := elc.otpService.ValidateOtp(req.Otp, req.PreToken, "login_otp:"+req.UserID)
	if !valid {
		return elc.sendErrorResponse(c, fiber.StatusUnauthorized, "Invalid OTP", nil)
	}

	// Get user
	user, err := elc.userRepo.GetUserByID(req.UserID)
	if err != nil {
		return elc.sendErrorResponse(c, fiber.StatusNotFound, "User not found", err)
	}

	// Complete login
	return elc.completeLogin(c, user, services.DeviceFingerprint{}, req.TrustDevice)
}

// VerifyTotp handles TOTP verification for authenticator-based 2FA
func (elc *EnhancedLoginController) VerifyTotp(c *fiber.Ctx) error {
	type VerifyTotpRequest struct {
		UserID      string `json:"user_id"`
		TotpCode    string `json:"totp_code"`
		TrustDevice bool   `json:"trust_device,omitempty"`
	}

	var req VerifyTotpRequest
	if err := c.BodyParser(&req); err != nil {
		return elc.sendErrorResponse(c, fiber.StatusBadRequest, "Invalid request format", err)
	}

	// Validate TOTP code
	valid := elc.otpService.ValidateTOTPCode(req.UserID, req.TotpCode)
	if !valid {
		return elc.sendErrorResponse(c, fiber.StatusUnauthorized, "Invalid TOTP code", nil)
	}

	// Get user
	user, err := elc.userRepo.GetUserByID(req.UserID)
	if err != nil {
		return elc.sendErrorResponse(c, fiber.StatusNotFound, "User not found", err)
	}

	// Complete login
	return elc.completeLogin(c, user, services.DeviceFingerprint{}, req.TrustDevice)
}

// handleAuthenticatorLogin processes authenticator-based login
func (elc *EnhancedLoginController) handleAuthenticatorLogin(c *fiber.Ctx, user *models.User, deviceFingerprint services.DeviceFingerprint) error {
	isTrusted, _, _ := elc.deviceService.IsDeviceTrusted(user.ID.String(), deviceFingerprint)

	if isTrusted {
		// Skip immediate verification for trusted devices
		return elc.sendSuccessResponse(c, "Authenticator verification required", fiber.Map{
			"requires_verification": true,
			"trusted_device":        true,
			"user_id":               user.ID,
		})
	}

	// For untrusted devices, require TOTP verification
	otp, preToken := elc.otpService.GenerateOtp("login:" + user.ID.String())

	return elc.sendSuccessResponse(c, "Authenticator verification required", fiber.Map{
		"requires_verification": true,
		"trusted_device":        false,
		"user_id":               user.ID,
		"pre_token":             preToken,
		"otp":                   otp,
	})
}

func (elc *EnhancedLoginController) initiateTwoFactor(c *fiber.Ctx, user *models.User, deviceFingerprint services.DeviceFingerprint, isTrustedDevice bool) error {
	// Check if TOTP is enabled for this user
	totpEnabled := elc.otpService.IsTOTPEnabled(user.ID.String())

	if totpEnabled {
		// If TOTP is enabled, require TOTP verification
		otp, preToken := elc.otpService.GenerateOtp("login_totp:" + user.ID.String())

		return elc.sendSuccessResponse(c, "TOTP verification required", fiber.Map{
			"requires_totp":  true,
			"user_id":        user.ID.String(),
			"pre_token":      preToken,
			"otp":            otp,
			"trusted_device": isTrustedDevice,
		})
	}

	// If TOTP not enabled but device is untrusted, send email OTP
	otp, preToken := elc.otpService.GenerateOtp("login_otp:" + user.ID.String())

	message := "Here is your OTP: " + otp + "\n\nIf this wasn't you, please secure your account immediately."
	title := "Authentication OTP - New Device"
	utils.SendEmail(user.Email, message, title, otp, "")

	return elc.sendSuccessResponse(c, "OTP sent successfully", fiber.Map{
		"requires_otp":   true,
		"pre_token":      preToken,
		"user_id":        user.ID.String(),
		"trusted_device": false,
	})
}

// completeLogin finalizes the authentication process
func (elc *EnhancedLoginController) completeLogin(c *fiber.Ctx, user *models.User, deviceFingerprint services.DeviceFingerprint, isTrustedDevice bool) error {
	// Generate access token
	accessToken, err := elc.pasetoMaker.CreateToken(user.ID, 24*time.Hour)
	if err != nil {
		return elc.sendErrorResponse(c, fiber.StatusInternalServerError, "Failed to create session", err)
	}

	// Register device if not already trusted
	if !isTrustedDevice {
		_, err = elc.deviceService.RegisterDevice(user.ID.String(), deviceFingerprint) // Keep as String() for device service if it expects string
		if err != nil {
			config.Logger.Error("Failed to register device", zap.Error(err))
		}
	}

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"data": fiber.Map{
			"access_token":   accessToken,
			"user":           user,
			"trusted_device": isTrustedDevice,
		},
		"error": nil,
	})
}

// Setup TOTP for a user
func (lc *EnhancedLoginController) SetupTOTP(c *fiber.Ctx) error {
	type SetupRequest struct {
		UserID string `json:"user_id"`
	}

	var req SetupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	// Get user to validate they exist and get their email
	user, err := lc.userRepo.GetUserByID(req.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
			"data":    nil,
			"error":   "User does not exist.",
		})
	}

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)

	// Check if TOTP is already enabled
	if otpService.IsTOTPEnabled(user.ID.String()) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "TOTP already enabled",
			"data":    nil,
			"error":   "TOTP is already set up for this user.",
		})
	}

	setup, err := otpService.GenerateTOTPSecret(user.ID.String(), user.Email)
	if err != nil {
		config.Logger.Error("Failed to generate TOTP secret", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Setup failed",
			"data":    nil,
			"error":   "Failed to generate TOTP secret.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP setup initiated",
		"data": fiber.Map{
			"qr_code_url":  setup.QRCodeURL,
			"manual_key":   setup.ManualKey,
			"instructions": "Scan the QR code with your authenticator app or manually enter the key. Then verify with a code to complete setup.",
		},
		"error": nil,
	})
}

// Enable TOTP after user verifies they can generate correct codes
func (lc *EnhancedLoginController) EnableTOTP(c *fiber.Ctx) error {
	type EnableRequest struct {
		UserID   string `json:"user_id"`
		TOTPCode string `json:"totp_code"`
	}

	var req EnableRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)

	err := otpService.EnableTOTP(req.UserID, req.TOTPCode)
	if err != nil {
		config.Logger.Error("Failed to enable TOTP", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Enable failed",
			"data":    nil,
			"error":   "Invalid code or setup not found.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP enabled successfully",
		"data": fiber.Map{
			"enabled": true,
		},
		"error": nil,
	})
}

// Disable TOTP for a user
func (lc *EnhancedLoginController) DisableTOTP(c *fiber.Ctx) error {
	type DisableRequest struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"` // Require password for security
	}

	var req DisableRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
			"data":    nil,
			"error":   "Invalid request format.",
		})
	}

	// Verify user's password before disabling TOTP
	user, err := lc.userRepo.GetUserByID(req.UserID)
	if err != nil || !services.CheckPasswordHash(req.Password, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Authentication failed",
			"data":    nil,
			"error":   "Invalid password.",
		})
	}

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)

	err = otpService.DisableTOTP(req.UserID)
	if err != nil {
		config.Logger.Error("Failed to disable TOTP", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Disable failed",
			"data":    nil,
			"error":   "Failed to disable TOTP.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "TOTP disabled successfully",
		"data": fiber.Map{
			"enabled": false,
		},
		"error": nil,
	})
}

// Check TOTP status for a user
func (lc *EnhancedLoginController) GetTOTPStatus(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	otpService := services.NewOtpService(lc.redisClient, lc.ctx)
	enabled := otpService.IsTOTPEnabled(userID)

	return c.JSON(fiber.Map{
		"message": "TOTP status retrieved",
		"data": fiber.Map{
			"totp_enabled": enabled,
		},
		"error": nil,
	})
}

// Utility methods
func (elc *EnhancedLoginController) sendErrorResponse(c *fiber.Ctx, status int, message string, err error) error {
	if err != nil {
		config.Logger.Error(message, zap.Error(err))
	}
	return c.Status(status).JSON(fiber.Map{
		"message": message,
		"error":   err.Error(),
	})
}

func (elc *EnhancedLoginController) sendSuccessResponse(c *fiber.Ctx, message string, data ...fiber.Map) error {
	response := fiber.Map{
		"message": message,
		"error":   nil,
	}

	if len(data) > 0 {
		response["data"] = data[0]
	}

	return c.JSON(response)
}
