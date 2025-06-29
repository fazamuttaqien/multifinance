package router

import (
	"errors"
	"time"

	"github.com/fazamuttaqien/multifinance/config"
	mysqldb "github.com/fazamuttaqien/multifinance/infra/mysql"
	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/middleware"
	ratelimiter "github.com/fazamuttaqien/multifinance/pkg/rate-limiter"
	"github.com/fazamuttaqien/multifinance/pkg/telemetry"
	"github.com/fazamuttaqien/multifinance/presenter"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewRouter(
	presenter presenter.Presenter,
	db *gorm.DB,
	tel *telemetry.OpenTelemetry,
	cfg *config.Config,
	limiter *ratelimiter.RateLimiter,
	store *session.Store,
) *fiber.App {

	jwtAuth := middleware.NewJWTAuthMiddleware(cfg.JWT_SECRET_KEY)
	customCSRF := middleware.NewCustomCSRFMiddleware(store)
	requireAdmin := middleware.RequireRole(domain.AdminRole)
	requireCustomer := middleware.RequireRole(domain.CustomerRole)
	// requirePartner := middleware.RequireRole(domain.PartnerRole)

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
		AllowOrigins: "http://localhost:5000",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-User-Id, X-User-Email, X-User-Name",
		AllowMethods: "GET, POST, PUT, DELETE, PATCH, OPTIONS",
	}))

	// (Opsional, karena punya Zap)
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${ip} ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	app.Use(otelfiber.Middleware(
		otelfiber.WithTracerProvider(tel.TracerProvider),
		otelfiber.WithPropagators(otel.GetTextMapPropagator()),
	))

	if !cfg.REQUESTS_METRIC {
		zap.L().Info("Enabling HTTP request metrics middleware")
		app.Use(middleware.NewOtelMiddleware())
	} else {
		zap.L().Info("HTTP request metrics middleware is disabled")
	}

	app.Get("/health", func(c *fiber.Ctx) error {
		if err := mysqldb.Ping(db, c.Context()); err != nil {
			zap.L().Error("Health check failed: database ping error", zap.Error(err))
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":      "healthy",
			"service":     cfg.SERVICE_NAME,
			"version":     cfg.SERVICE_VERSION,
			"environment": cfg.ENVIRONMENT,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api := app.Group("/api/v1")

	api.Use(limiter.RateLimitMiddleware())

	authAPI := api.Group("/auth")
	{
		authAPI.Post("/register", customCSRF, presenter.ProfilePresenter.Register)
		authAPI.Post("/login", presenter.PrivatePresenter.Login)
		authAPI.Post("/logout", jwtAuth, customCSRF, presenter.PrivatePresenter.Logout)
		authAPI.Get("/csrf-token", func(c *fiber.Ctx) error {
			sess, err := store.Get(c)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session error"})
			}

			token := sess.Get("csrf_token")
			if token == nil {
				newToken, err := middleware.GenerateCSRFToken()
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate CSRF token"})
				}
				sess.Set("csrf_token", newToken)
				sess.Save()
				token = newToken
			}
			return c.JSON(fiber.Map{"csrf_token": token})
		})
	}

	customersAPI := api.Group("/me", jwtAuth, requireCustomer)
	{
		customersAPI.Get("/profile", presenter.ProfilePresenter.GetMyProfile)
		customersAPI.Put("/profile", customCSRF, presenter.ProfilePresenter.UpdateMyProfile)
		customersAPI.Put("/profile", presenter.ProfilePresenter.UpdateMyProfile)
		customersAPI.Get("/limits", presenter.ProfilePresenter.GetMyLimits)
		customersAPI.Get("/transactions", presenter.ProfilePresenter.GetMyTransactions)
	}

	adminAPI := api.Group("/admin", jwtAuth, customCSRF, requireAdmin)

	adminCustomersAPI := adminAPI.Group("/customers")
	{
		adminCustomersAPI.Post("/:customerId/limits", presenter.AdminPresenter.SetLimits)
		adminCustomersAPI.Get("/", presenter.AdminPresenter.ListCustomers)
		adminCustomersAPI.Get("/:customerId", presenter.AdminPresenter.GetCustomerByID)
		adminCustomersAPI.Post("/:customerId/verify", presenter.AdminPresenter.VerifyCustomer)
	}

	partnerAPI := api.Group("/partners", jwtAuth, customCSRF, requireCustomer)
	{
		partnerAPI.Post("/transactions", presenter.PartnerPresenter.CreateTransaction)
		partnerAPI.Post("/check-limit", presenter.PartnerPresenter.CheckLimit)
	}

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
