package middleware

import (
	"errors"
	"strconv"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const AdminID uint64 = 1

// NewAdminMiddleware membuat middleware yang hanya mengizinkan akses untuk admin.
func NewAdminMiddleware(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, err := getUserFromHeader(c, db)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		// Validasi apakah user ini adalah admin
		if user.ID != AdminID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied: admin rights required"})
		}

		// Simpan ID di locals untuk konsistensi, meskipun mungkin tidak digunakan
		c.Locals("adminID", user.ID)
		return c.Next()
	}
}

// NewCustomerMiddleware membuat middleware yang hanya mengizinkan akses untuk customer biasa.
func NewCustomerMiddleware(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, err := getUserFromHeader(c, db)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		// Validasi apakah user ini BUKAN admin
		if user.ID == AdminID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied: this endpoint is for customers only"})
		}

		// Simpan ID di locals untuk digunakan oleh handler selanjutnya
		c.Locals("customerID", user.ID)
		return c.Next()
	}
}

func getUserFromHeader(c *fiber.Ctx, db *gorm.DB) (*domain.Customer, error) {
	userIDStr := c.Get("X-User-ID")
	if userIDStr == "" {
		return nil, errors.New("missing X-User-ID header")
	}

	customerID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return nil, errors.New("invalid X-User-ID header")
	}

	var customer domain.Customer
	if err := db.First(&customer, customerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &customer, nil
}
