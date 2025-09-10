package router

import (
	"context"
	"time"
	indexing_repository "town-planning-backend/bleve/repositories"
	"town-planning-backend/middleware"
	"town-planning-backend/token"
	"town-planning-backend/users/controllers"
	"town-planning-backend/users/repositories"
	"town-planning-backend/users/services"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func InitRoutes(
	app *fiber.App,
	userRepo repositories.UserRepository,
	ctx context.Context,
	redisClient *redis.Client,
	tokenMaker token.Maker,
	bleveRepo indexing_repository.BleveRepositoryInterface,
	db *gorm.DB,
	baseURL string,
	baseFrontendURL string,
) {
	// Initialize services
	magicLinkService := services.NewMagicLinkService(redisClient, ctx, baseURL, baseFrontendURL)
	otpService := services.NewOtpService(redisClient, ctx)
	deviceService := services.NewDeviceService(
		redisClient,
		ctx,
		baseFrontendURL,
		30*24*time.Hour, // deviceTTL of 30 days
	)
	authPrefService := services.NewAuthPreferencesService(userRepo, db, redisClient, ctx)

	// Initialize controllers
	userController := &controllers.UserController{
		UserRepo:  userRepo,
		DB:        db,
		Ctx:       ctx,
		BleveRepo: bleveRepo,
	}

	enhancedLoginController := controllers.NewEnhancedLoginController(
		userRepo,
		tokenMaker,
		ctx,
		redisClient,
		magicLinkService,
		otpService,
		authPrefService,
		deviceService,
		baseFrontendURL,
	)

	authPrefController := controllers.NewAuthPreferencesController(
		authPrefService,
		userRepo,
	)

	// Create an instance of AppContext
	appContext := &middleware.AppContext{
		PasetoMaker: tokenMaker,
		Ctx:         ctx,
		RedisClient: redisClient,
	}

	// Public routes (no authentication required)
	publicRoutes := app.Group("/api/v1")
	{
		// Authentication routes
		publicRoutes.Post("/auth/login", enhancedLoginController.InitiateLogin)
		publicRoutes.Post("/auth/magiclink/verify", enhancedLoginController.VerifyMagicLink)
		publicRoutes.Post("/auth/verify-otp", enhancedLoginController.VerifyOtp)
		publicRoutes.Post("/auth/verify-totp", enhancedLoginController.VerifyTotp)

		// Password recovery
		publicRoutes.Post("/auth/forgot-password-request", enhancedLoginController.ForgotPasswordRequest)
		publicRoutes.Post("/auth/forgot-password-reset", enhancedLoginController.ForgotPasswordReset)

		// TOTP setup
		publicRoutes.Post("/auth/totp/setup", enhancedLoginController.SetupTOTP)
		publicRoutes.Post("/auth/totp/enable", enhancedLoginController.EnableTOTP)
		publicRoutes.Post("/auth/totp/disable", enhancedLoginController.DisableTOTP)
		publicRoutes.Get("/auth/totp/status/:user_id", enhancedLoginController.GetTOTPStatus)
	}

	// Protected routes (require authentication)
	protectedRoutes := app.Group("/api/v1")
	protectedRoutes.Use(middleware.ProtectedRoute(appContext))
	{
		// User management routes group
		userRoutes := protectedRoutes.Group("/users")
		{
			// Specific routes first
			userRoutes.Get("/filtered", userController.GetFilteredUsersController)

			// General routes
			userRoutes.Get("/", userController.GetAllUsersController)
			userRoutes.Post("/", userController.CreateUser)

			// ID-based routes with validation
			userRoutes.Get("/:id", userController.RetrieveSingleUserController)
			userRoutes.Patch("/:id", userController.UpdateUserController)
			userRoutes.Delete("/:id", userController.DeleteUserController)
		}

		// Authentication preferences
		authRoutes := protectedRoutes.Group("/auth")
		{
			authRoutes.Post("/preferences/method", authPrefController.SetAuthMethod)
			authRoutes.Get("/preferences/methods/:user_id", authPrefController.GetAuthMethods)

			// Device management
			authRoutes.Get("/devices", enhancedLoginController.GetTrustedDevices)
			authRoutes.Get("/trusted-devices/:userID", enhancedLoginController.GetTrustedDeviceByUserID)
			authRoutes.Delete("/devices", enhancedLoginController.RemoveTrustedDevice)

			// Session management
			authRoutes.Post("/logout", enhancedLoginController.LogoutUser)
		}

		// Public user route (moved here to ensure proper ordering)
		publicRoutes.Get("/users/:id", userController.RetrieveSingleUserController)
	}
}
