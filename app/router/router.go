package router

import (
	"errors"
	"time"

	"github.com/fazamuttaqien/multifinance/config"
	"github.com/fazamuttaqien/multifinance/database"
	"github.com/fazamuttaqien/multifinance/middleware"
	"github.com/fazamuttaqien/multifinance/presenter"
	"github.com/fazamuttaqien/multifinance/telemetry"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewRouter(
	presenter presenter.Presenter,
	db *gorm.DB,
	tel *telemetry.OpenTelemetry,
	cfg *config.Config,
) *fiber.App {

	adminAuth := middleware.NewAdminMiddleware(db)
	customerAuth := middleware.NewCustomerMiddleware(db)

	app := fiber.New(fiber.Config{
		BodyLimit:    10 * 1024 * 1024,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: ErrorCustomHandler(tel.Log),
	})

	// 1. Recovery dari panic
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))
	// 2. Security Headers
	app.Use(helmet.New())
	// 3. CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-User-Id, X-User-Email, X-User-Name",
		AllowMethods: "GET, POST, PUT, DELETE, PATCH, OPTIONS",
	}))

	// (Opsional, karena sudah ada Zap)
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${ip} ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	// OpenTelemetry Fiber
	app.Use(otelfiber.Middleware(
		otelfiber.WithTracerProvider(tel.TracerProvider),
		otelfiber.WithPropagators(otel.GetTextMapPropagator()),
	))

	// Middleware Kustom untuk Metrik HTTP (jika diaktifkan di config)
	if !cfg.REQUESTS_METRIC {
		zap.L().Info("Enabling HTTP request metrics middleware")
		app.Use(middleware.NewOtelMiddleware())
	} else {
		zap.L().Info("HTTP request metrics middleware is disabled")
	}

	app.Get("/health", func(c *fiber.Ctx) error {
		if err := database.Ping(db, c.Context()); err != nil {
			zap.L().Fatal(err.Error())
			return err
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":      fiber.StatusOK,
			"service":     cfg.SERVICE_NAME,
			"version":     cfg.SERVICE_VERSION,
			"environment": cfg.ENVIRONMENT,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api := app.Group("/api/v1")

	api.Post("/register", presenter.ProfilePresenter.Register)

	customersAPI := api.Group("/customers", customerAuth)
	{
		customersAPI.Get("/profile", presenter.ProfilePresenter.GetMyProfile)
		customersAPI.Put("/profile", presenter.ProfilePresenter.UpdateMyProfile)
		customersAPI.Get("/limits", presenter.ProfilePresenter.GetMyLimits)
		customersAPI.Get("/transactions", presenter.ProfilePresenter.GetMyTransactions)
	}

	adminAPI := api.Group("/admin", adminAuth)
	adminCustomersAPI := adminAPI.Group("/customers")
	{
		adminCustomersAPI.Post("/:customerId/limits", presenter.AdminPresenter.SetLimits)
		adminCustomersAPI.Get("/", presenter.AdminPresenter.ListCustomers)
		adminCustomersAPI.Get("/:customerId", presenter.AdminPresenter.GetCustomerByID)
		adminCustomersAPI.Post("/:customerId/verify", presenter.AdminPresenter.VerifyCustomer)
	}

	partnerAPI := api.Group("/partners")
	{
		partnerAPI.Post("/transactions", presenter.PartnerPresenter.CreateTransaction)
		partnerAPI.Post("/check-limit", presenter.PartnerPresenter.CheckLimit)
	}

	// --- Handler 404 Not Found (terakhir) ---
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Resource not found",
			"path":    c.Path(),
		})
	})

	return app
}

func ErrorCustomHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		message := "Internal Server Error"

		var e *fiber.Error
		if errors.As(err, &e) {
			code = e.Code
			message = e.Message
		}

		log.Error("Request error occured",
			zap.Error(err),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()),
			zap.Int("status_code", code),
		)

		return c.Status(code).JSON(fiber.Map{
			"error":   true,
			"message": message,
			"code":    code,
		})
	}
}
