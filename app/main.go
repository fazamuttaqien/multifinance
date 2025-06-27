package main

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/fazamuttaqien/multifinance/helper/cloudinary"
	"github.com/fazamuttaqien/multifinance/infra"
	"github.com/fazamuttaqien/multifinance/model"
	"github.com/fazamuttaqien/multifinance/presenter"
	"github.com/fazamuttaqien/multifinance/router"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize db connection
	db, err := infra.InitializeDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer infra.Close(db)

	seedAdmin(db)

	// Enable debug mode for development (optional)
	infra.EnableDebugMode(db)

	// Test connection
	if err := infra.Ping(db); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("Database connection successful!")

	// Auto-migrate models
	// if err := models.MigrateDB(db.DB); err != nil {
	// 	log.Fatalf("Failed to migrate database: %v", err)
	// }
	// log.Println("Database migration completed!")

	// Print connection stats
	stats := infra.GetStats(db)
	log.Printf("Database stats: %+v", stats)

	// Initialize Cloudinary service
	cloudinaryService, err := cloudinary.NewCloudinaryService(cloudinary.CloudinaryConfig{
		CloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		APIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		APISecret: os.Getenv("CLOUDINARY_API_SECRET"),
	})
	if err != nil {
		log.Fatal("Failed to initialize Cloudinary service:", err)
	}

	presenter := presenter.NewPresenter(db, cloudinaryService)
	router := router.NewRouter(presenter, db)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(router.Listen(":" + port))
}

const (
	AdminID  uint64 = 1
	AdminNIK string = "1010010110100101"
)

func seedAdmin(db *gorm.DB) {
	log.Println("Checking for admin user...")

	var adminUser model.Customer
	err := db.First(&adminUser, AdminID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Println("Admin user not found, creating one...")

		// Buat data admin baru
		newAdmin := model.Customer{
			ID:                 AdminID,
			NIK:                AdminNIK,
			FullName:           "Administrator",
			LegalName:          "System Administrator",
			BirthPlace:         "System",
			BirthDate:          time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             99999999,
			KTPPhotoURL:        "https://via.placeholder.com/150",
			SelfiePhotoURL:     "https://via.placeholder.com/150",
			VerificationStatus: model.VerificationVerified,
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
