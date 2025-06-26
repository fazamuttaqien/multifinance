package main

import (
	"log"
	"os"

	"github.com/fazamuttaqien/xyz-multifinance/controllers"
	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/repositories"
	"github.com/fazamuttaqien/xyz-multifinance/usecases"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize db connection
	db, err := databases.InitializeDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer databases.Close(db)

	// Enable debug mode for development (optional)
	databases.EnableDebugMode(db)

	// Test connection
	if err := databases.Ping(db); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("Database connection successful!")

	// Auto-migrate models
	// if err := models.MigrateDB(db.DB); err != nil {
	// 	log.Fatalf("Failed to migrate database: %v", err)
	// }
	// log.Println("Database migration completed!")

	// Print connection stats
	stats := databases.GetStats(db)
	log.Printf("Database stats: %+v", stats)

	app := fiber.New()
	app.Use(logger.New())

	uploader, err := helper.NewCloudinaryUploader()
	if err != nil {
		log.Fatalf("Failed to initialize Cloudinary: %v", err)
	}

	customerRepository := repositories.NewCustomerRepository(db)
	limitRepository := repositories.NewLimitRepository(db)
	tenorRepository := repositories.NewTenorRepository(db)
	transactionRepository := repositories.NewTransactionRepository(db)

	usecase := usecases.NewUsecase(
		db,
		customerRepository,
		limitRepository,
		tenorRepository,
		transactionRepository,
		uploader)
	controller := controllers.NewController(usecase)

	api := app.Group("/api/v1")
	customersAPI := api.Group("/customers")
	{
		customersAPI.Post("/register", controller.RegisterCustomer)
		customersAPI.Get("/:customerId/limits/:tenorMonths", controller.GetLimit)
	}
	adminAPI := api.Group("/admin")
	{
		adminAPI.Post("/:customerId/limits", controller.SetLimits)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

// Alternative initialization methods

func initWithCustomConfig() {
	// Initialize with custom configuration
	db, err := databases.InitializeDatabaseWithConfig(
		"localhost",
		"root",
		"password",
		"loan_db",
		3306,
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer databases.Close(db)

	log.Println("Connected with custom config")
}

func initWithManualConfig() {
	// Manual configuration
	config := databases.CreateConfig("localhost", "root", "password", "loan_db", 3306)

	db, err := databases.Connect(config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer databases.Close(db)

	log.Println("Connected with manual config!")
}
