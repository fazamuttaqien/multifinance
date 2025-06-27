package router

import (
	"github.com/fazamuttaqien/multifinance/middleware"
	"github.com/fazamuttaqien/multifinance/presenter"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"gorm.io/gorm"
)

func NewRouter(presenter presenter.Presenter, db *gorm.DB) *fiber.App {

	// --- Middlewares ---
	adminAuth := middleware.NewAdminMiddleware(db)
	customerAuth := middleware.NewCustomerMiddleware(db)

	app := fiber.New()
	app.Use(logger.New())

	api := app.Group("/api/v1")

	customersAPI := api.Group("/profile", customerAuth)
	{
		customersAPI.Post("/register", presenter.ProfilePresenter.Register)
		customersAPI.Post("/transactions", presenter.ProfilePresenter.CreateTransaction)
		customersAPI.Get("/", presenter.ProfilePresenter.GetMyProfile)
		customersAPI.Put("/", presenter.ProfilePresenter.UpdateMyProfile)
		customersAPI.Get("/limits", presenter.ProfilePresenter.GetMyLimits)
		customersAPI.Get("/transactions", presenter.ProfilePresenter.GetMyTransactions)
	}

	adminAPI := api.Group("/admin", adminAuth)
	{
		adminAPI.Post("/:customerId/limits", presenter.AdminPresenter.SetLimits)
		adminAPI.Get("/", presenter.AdminPresenter.ListCustomers)
		adminAPI.Get("/:customerId", presenter.AdminPresenter.GetCustomerByID)
		adminAPI.Post("/:customerId/verify", presenter.AdminPresenter.VerifyCustomer)
	}

	return app
}
