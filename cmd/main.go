package main

import (
	"context"
	"log"

	config "town-planning-backend/config"
	"town-planning-backend/token"
	"town-planning-backend/utils"

	// "town-planning-backend/seeds"

	"town-planning-backend/middleware"

	// Repositories

	applicants_repositories "town-planning-backend/applicants/repositories"
	applications_repositories "town-planning-backend/applications/repositories"
	document_repositories "town-planning-backend/documents/repositories"
	stands_repositories "town-planning-backend/stands/repositories"
	users_repositories "town-planning-backend/users/repositories"
	applications_services "town-planning-backend/applications/services"

	// Routes

	applicant_routes "town-planning-backend/applicants/routes"
	application_routes "town-planning-backend/applications/routes"
	stand_routes "town-planning-backend/stands/routes"
	user_routes "town-planning-backend/users/routes"

	// bleve
	bleveControllers "town-planning-backend/bleve/controllers"
	bleveRepositories "town-planning-backend/bleve/repositories"
	bleveRoutes "town-planning-backend/bleve/routes"
	bleveServices "town-planning-backend/bleve/services"

	// documents
	document_services "town-planning-backend/documents/services"
	// services

	// "town-planning-backend/internal/bootstrap"

	// WebSocket
	"town-planning-backend/websocket"

	// WhatsApp

	// Other imports
	"encoding/gob"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	// "github.com/gofiber/fiber/v2/middleware/session"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Initialize Zap logger
	config.InitLogger()

	// Load environment variables
	err := godotenv.Load(".env")
	if err != nil {
		config.Logger.Fatal("Error loading .env file", zap.Error(err))
	}
	gob.Register(uuid.UUID{})

	app := fiber.New()

	// Apply CORS middleware from middleware package
	middleware.InitCors(app)

	// Initialize database and configs
	db := config.ConfigureDatabase()
	port := config.GetEnv("PORT")
	ctx := context.Background()

	// Redis client for Asynq and other uses
	redisAddr := config.GetEnv("REDIS_ADDRESS")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default for development
		config.Logger.Warn("REDIS_ADDRESS not set, using default: localhost:6379")
	}

	redisClient := config.InitRedisServer(ctx) // Assuming this gives you a *redis.Client or similar
	// Note: asynq.RedisClientOpt uses its own Redis client internally.

	asynqRedisOpt := asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: "", // Or config.GetEnv("REDIS_PASSWORD") if needed
		DB:       0,
	}

	asynqClient := asynq.NewClient(asynqRedisOpt)
	defer asynqClient.Close()

	tokenKey := config.GetEnv("TOKEN_SYMMETRIC_KEY")
	tokenMaker, err := token.NewPasetoMaker(tokenKey)
	if err != nil {
		config.Logger.Fatal("Cannot create token maker", zap.Error(err))

	}
	// TODO: Update bleve index path for Docker volume in production
	indexPath := config.GetEnv("BLEVE_INDEX_PATH")
	if indexPath == "" {
		indexPath = "./bleve_data" // Default for local development
		config.Logger.Warn("BLEVE_INDEX_PATH not set, using default: ./bleve_data")
	}

	// Get base URLs from environment or use defaults
	baseURL := config.GetEnv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Default backend URL
		config.Logger.Warn("BASE_URL not set, using default", zap.String("url", baseURL))
	}

	baseFrontendURL := config.GetEnv("BASE_FRONTEND_URL")
	if baseFrontendURL == "" {
		baseFrontendURL = "http://localhost:5173" // Default frontend URL
		config.Logger.Warn("BASE_FRONTEND_URL not set, using default", zap.String("url", baseFrontendURL))
	}

	// // Use Cloudflare Workers AI (Recommended)
	// cloudFlareService, err := internal_services.NewCloudflareWorkerAIService(
	// 	config.GetEnv("CLOUDFLARE_ACCOUNT_ID"),
	// 	config.GetEnv("CLOUDFLARE_API_TOKEN"),
	// )
	// if err != nil {
	// 	log.Fatal("Failed to create Cloudflare service:", err)
	// }

	// Initialize the mailer
	utils.InitializeMailer()

	// Now you can use the mailer globally
	mailer := utils.GetMailer()
	if mailer == nil {
		config.Logger.Fatal("Mailer not initialized")
		log.Fatalf("Mailer not initialized")
	}

	// ------ WebSocket Hub Initialization for Real-time Chat ------
	config.Logger.Info("Initializing WebSocket hub for real-time chat features...")
	wsHub := websocket.NewHub()
	go wsHub.Run()

	

	// Serve static files
	app.Static("/public", "./public")
	app.Static("/uploads", "./uploads")

	// Repositories
	bleveIndexingService := bleveServices.NewIndexingService(config.Logger, indexPath)
	standRepo := stands_repositories.NewStandRepository(db)
	userRepo := users_repositories.NewUserRepository(db)
	applicantRepo := applicants_repositories.NewApplicantRepository(db)
	bleveServiceRepo, bleveInterfaceRepo := bleveRepositories.NewBleveRepository(bleveIndexingService)
	documentRepo := document_repositories.NewDocumentRepository(db, standRepo)
	readReceiptService := applications_services.NewReadReceiptService(db)

	// Services
	fileStorage := utils.NewLocalFileStorage("./uploads")
	documentService := document_services.NewDocumentService(documentRepo, fileStorage)

	applicationRepo := applications_repositories.NewApplicationRepository(db, documentService)

	// Routes
	user_routes.InitRoutes(app, userRepo, ctx, redisClient, tokenMaker, bleveInterfaceRepo, db, baseURL, baseFrontendURL)
	applicant_routes.ApplicantInitRoutes(app, applicantRepo, bleveInterfaceRepo, db)
	application_routes.ApplicationRouterInit(app, db, applicationRepo, bleveInterfaceRepo, userRepo, documentService, applicantRepo, wsHub) // Added wsHub
	stand_routes.StandRouterInit(app, db, standRepo, bleveInterfaceRepo)

	// Create WebSocket handler with token validation
	wsHandler := websocket.NewWsHandler(wsHub, tokenMaker, *readReceiptService)

	// ------ WebSocket Route for Real-time Communication ------
	app.Get("/ws", wsHandler.HandleWebSocket)
	config.Logger.Info("WebSocket endpoint registered at /ws")

	// Bleve Routes
	bleveController := bleveControllers.NewSearchController(bleveServiceRepo)
	bleveRoutes.InitBleveRoutes(app, bleveController, db)

	// Date location
	if err := utils.InitializeDateLocation(); err != nil {
		config.Logger.Fatal("Failed to initialize date location", zap.Error(err))
	}

	// Background cleanup tasks
	go utils.RunScheduledCleanup(redisClient)

	// // Re-Index all data
	// bootstrap.IndexBleveData(ctx, userRepo, applicantRepo, standRepo, bleveInterfaceRepo)

	//------ Run seeders for initial data with proper error handling and logging ------ //
	// config.Logger.Info("Starting database seeding...")

	// // Run comprehensive database seeding
	// config.Logger.Info("Starting comprehensive database seeding process...")

	// if err := seeds.SeedTownPlanningAll(db); err != nil {
	// 	config.Logger.Error("Database seeding failed", zap.Error(err))
	// } else {
	// 	config.Logger.Info("All database seeding completed successfully")
	// }

	// config.Logger.Info("Database seeding process finished")

	//------ Run seeders for initial data with proper error handling and logging ------ //

	// Start the application
	config.Logger.Info("Server starting with WebSocket support", zap.String("port", port))
	config.Logger.Fatal("Server failed", zap.String("port", port), zap.Error(app.Listen(":"+port)))
}

// // Initialize and start the PaymentCalculationService
// paymentCalculationService := payment_services.NewPaymentCalculationService(db, paymentRepo)
// paymentCalculationService.StartDailyCalculations()

// // Generate a TOKEN_SYMMETRIC_KEY
// key := make([]byte, 32)
// _, err = rand.Read(key)
// if err != nil {
// 	log.Fatalf("Error generating random key: %v", err)
// }

// // Encode the key to base64 so it can be easily stored in an environment variable or .env file
// encodedKey := base64.URLEncoding.EncodeToString(key)

// // Trim the encoded key to the first 32 characters
// trimmedKey := encodedKey[:32]

// fmt.Println("Your secure PASETO symmetric key (trimmed to 32 characters):")
// fmt.Println(trimmedKey)
// fmt.Println("\nIMPORTANT: Add this to your .env file as TOKEN_SYMMETRIC_KEY and ensure .env is in .gitignore.")
// fmt.Println("Example line for your .env file:")
// fmt.Printf(`TOKEN_SYMMETRIC_KEY=%s`, trimmedKey)
// fmt.Println()

// err = config.BackupDatabase()
// if err != nil {
// 	config.Logger.Error("Error backing up database", zap.Error(err))
// 	log.Fatalf("Error backing up database: %v", err)
// }

// err = config.RestoreDatabase()
// if err != nil {
// 	config.Logger.Error("Error restoring database", zap.Error(err))
// 	log.Fatalf("Error restoring database: %v", err)
// }
