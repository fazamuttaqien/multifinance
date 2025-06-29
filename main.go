package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fazamuttaqien/multifinance/config"
	"github.com/fazamuttaqien/multifinance/database"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/pkg/cloudinary"
	"github.com/fazamuttaqien/multifinance/pkg/telemetry"
	"github.com/fazamuttaqien/multifinance/presenter"
	"github.com/fazamuttaqien/multifinance/router"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	slog.Info("Starting application setup...")

	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		slog.Error("No .env file found, using system environment variables", "error", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}

	tel, err := telemetry.New(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize monitoring: %v", err))
	}

	db, err := database.InitializeDatabase()
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelShutdown()

		zap.L().Info("Closing MySQL Connection...")
		if err := database.Close(db, shutdownCtx); err != nil {
			zap.L().Error("Error disconnecting from MySQL", zap.Error(err))
		} else {
			zap.L().Info("Disconnected from MySQL.")
		}

		zap.L().Info("Shutting down monitoring...")
		if err := tel.Shutdown(shutdownCtx); err != nil {
			zap.L().Error("Error during monitoring shutdown", zap.Error(err))
		} else {
			zap.L().Info("Monitoring shutdown complete.")
		}
	}()

	if err := model.AutoMigrate(db); err != nil {
		slog.Error("Failed to migrate database", "error", err)
		os.Exit(1)
	}
	slog.Info("Database migration completed!")

	SeedTenors(db)
	SeedAdmin(db)

	database.EnableDebugMode(db)

	if err := database.Ping(db, ctx); err != nil {
		slog.Error("Database ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Database connection successful!")

	stats := database.GetStats(db)
	slog.Info("Database stats:", "stats", stats)

	cld, err := cloudinary.InitCloudinary(cfg)
	if err != nil {
		slog.Error("Failed to initialize Cloudinary service:", "error", err)
		os.Exit(1)
	}

	presenter := presenter.NewPresenter(db, cld, tel)
	router := router.NewRouter(presenter, db, tel, cfg)

	addr := ":" + cfg.SERVER_PORT

	// --- Jalankan Server & Handle Graceful Shutdown ---

	// Channel untuk listen error dari app.Listen
	listenErr := make(chan error, 1)

	// Jalankan server di goroutine
	go func() {
		zap.L().Info("Server starting", zap.String("address", addr))
		if err := router.Listen(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		} else {
			listenErr <- nil
		}
	}()

	// Channel untuk sinyal shutdown OS
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Blokir sampai menerima sinyal shutdown atau error dari Listen
	select {
	case sig := <-shutdown:
		zap.L().Info("Received shutdown signal", zap.String("signal", sig.String()))
	case err := <-listenErr:
		if err != nil {
			zap.L().Error("Server listen error", zap.Error(err))
			// Mungkin perlu exit atau logic lain jika Listen gagal start
			os.Exit(1)
		}
	}

	// Mulai proses graceful shutdown
	zap.L().Info("Starting graceful shutdown...")
	shutdownTimeout := 10 * time.Second
	if err := router.ShutdownWithTimeout(shutdownTimeout); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			zap.L().Warn("Server shutdown timed out", zap.Duration("timeout", shutdownTimeout))
		} else {
			zap.L().Error("Server shutdown error", zap.Error(err))
		}
	} else {
		zap.L().Info("Server gracefully stopped.")
	}

	zap.L().Info("Application shutdown complete.")
}

const (
	AdminID  uint64 = 1
	AdminNIK string = "1010010110100101"
)

func SeedAdmin(db *gorm.DB) {
	slog.Info("Checking for admin user...")

	var adminUser model.Customer
	err := db.First(&adminUser, AdminID).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Info("Admin user not found, creating one...")

		newAdmin := model.Customer{
			ID:                 AdminID,
			NIK:                AdminNIK,
			FullName:           "Administrator",
			LegalName:          "System Administrator",
			BirthPlace:         "System",
			BirthDate:          time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             99999999,
			KtpPhotoUrl:        "https://via.placeholder.com/150",
			SelfiePhotoUrl:     "https://via.placeholder.com/150",
			VerificationStatus: model.VerificationVerified,
		}

		if err := db.Create(&newAdmin).Error; err != nil {
			slog.Error("Failed to seed admin user", "error", err)
			os.Exit(1)
		}
		slog.Info("Admin user created successfully.")
	} else if err != nil {
		slog.Error("Error checking for admin user", "error", err)
		os.Exit(1)
	} else {
		slog.Info("Admin user already exists.")
	}
}

func SeedTenors(db *gorm.DB) {
	slog.Info("Seeding master tenors...")

	tenors := []model.Tenor{
		{ID: 1, DurationMonths: 1, Description: "1 Months"},
		{ID: 2, DurationMonths: 2, Description: "2 Months"},
		{ID: 3, DurationMonths: 3, Description: "3 Months"},
		{ID: 4, DurationMonths: 6, Description: "6 Months"},
		{ID: 5, DurationMonths: 9, Description: "9 Months"},
		{ID: 6, DurationMonths: 12, Description: "12 Months"},
		{ID: 7, DurationMonths: 24, Description: "24 Months"},
	}

	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "duration_months"}},
		DoNothing: true,
	}).Create(&tenors).Error; err != nil {
		slog.Error("Failed to seed tenors", "error", err)
		os.Exit(1)
	}

	slog.Info("Master tenors seeded successfully.")
}
