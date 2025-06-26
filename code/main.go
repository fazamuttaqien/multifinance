package main

import (
	"log"
	"time"

	"github.com/fazamuttaqien/xyz-multifinance/database"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database connection
	db, err := database.InitializeDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Enable debug mode for development (optional)
	db.EnableDebugMode()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("Database connection successful!")

	// Auto-migrate models
	// if err := models.MigrateDB(db.DB); err != nil {
	// 	log.Fatalf("Failed to migrate database: %v", err)
	// }
	// log.Println("Database migration completed!")

	// Print connection stats
	stats := db.GetStats()
	log.Printf("Database stats: %+v", stats)

	// Example: Create a customer
	customer := models.Customer{
		NIK:                "0123456778901234",
		FullName:           "Jane Doe",
		LegalName:          "Jane Doe",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Now().AddDate(-30, 0, 0), // 30 years ago
		Salary:             5000000.00,
		KTPPhotoURL:        "https://example.com/ktp.jpg",
		SelfiePhotoURL:     "https://example.com/selfie.jpg",
		VerificationStatus: models.VerificationPending,
	}

	// Insert customer
	result := db.DB.Create(&customer)
	if result.Error != nil {
		log.Printf("Failed to create customer: %v", result.Error)
	} else {
		log.Printf("Customer created with ID: %d", customer.ID)
	}

	// Example: Find customer by NIK
	var foundCustomer models.Customer
	if err := db.DB.Where("nik = ?", "1234567890123456").First(&foundCustomer).Error; err != nil {
		log.Printf("Customer not found: %v", err)
	} else {
		log.Printf("Found customer: %s", foundCustomer.FullName)
	}
}

// Alternative initialization methods

func initWithCustomConfig() {
	// Initialize with custom configuration
	db, err := database.InitializeDatabaseWithConfig(
		"localhost",
		"root",
		"password",
		"loan_db",
		3306,
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("Connected with custom config")
}

func initWithManualConfig() {
	// Manual configuration
	config := database.CreateConfig("localhost", "root", "password", "loan_db", 3306)

	db, err := database.Connect(config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	log.Println("Connected with manual config!")
}
