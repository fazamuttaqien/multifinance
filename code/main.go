package main

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/fazamuttaqien/xyz-multifinance/controllers"
	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"github.com/fazamuttaqien/xyz-multifinance/repositories"
	"github.com/fazamuttaqien/xyz-multifinance/usecases"
	"gorm.io/gorm"

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

	seedAdmin(db)

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
		// customersAPI.Get("/:customerId/limits/:tenorMonths", controller.GetLimit)
	}

	adminAPI := api.Group("/admin")
	{
		adminAPI.Post("/:customerId/limits", controller.SetLimits)
	}

	partnersAPI := api.Group("/partners")
	{
		partnersAPI.Post("/transactions", controller.CreateTransaction)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

const (
	AdminID  uint64 = 1
	AdminNIK string = "1010010110100101"
)

func seedAdmin(db *gorm.DB) {
	log.Println("Checking for admin user...")

	var adminUser models.Customer
	err := db.First(&adminUser, AdminID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Println("Admin user not found, creating one...")

		// Buat data admin baru
		newAdmin := models.Customer{
			ID:                 AdminID,
			NIK:                AdminNIK,
			FullName:           "Administrator",
			LegalName:          "System Administrator",
			BirthPlace:         "System",
			BirthDate:          time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             99999999,
			KTPPhotoURL:        "https://via.placeholder.com/150",
			SelfiePhotoURL:     "https://via.placeholder.com/150",
			VerificationStatus: models.VerificationVerified,
		}

		if err := db.Create(&newAdmin).Error; err != nil {
			log.Fatalf("Failed to seed admin user: %v", err)
		}
		log.Println("Admin user created successfully.")
	} else if err != nil {
		log.Fatalf("Error checking for admin user: %v", err)
	} else {
		log.Println("Admin user already exists.")
	}
}
