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

	api.Post("/register", presenter.ProfilePresenter.Register)

	customersAPI := api.Group("/customers", customerAuth)
	{
		customersAPI.Post("/transactions", presenter.ProfilePresenter.CreateTransaction)
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

	return app
}
