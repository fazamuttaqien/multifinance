package middleware

import (
	"errors"
	"strconv"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const AdminID uint64 = 1

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

func NewJWTAuthMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Cookies("private")
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing auth token cookie"})
		}

		claims := &domain.JwtCustomClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired JWT"})
		}

		c.Locals("user", claims)
		return c.Next()
	}
}

func RequireRole(allowedRoles ...domain.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userClaims, ok := c.Locals("user").(*domain.JwtCustomClaims)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not parse user claims"})
		}

		for _, role := range allowedRoles {
			if userClaims.Role == role {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied: insufficient permissions"})
	}
}

func GetClaimsFromLocals(c *fiber.Ctx) (*domain.JwtCustomClaims, error) {
	claims, ok := c.Locals("user").(*domain.JwtCustomClaims)
	if !ok {
		return nil, errors.New("user claims not found in context")
	}
	return claims, nil
}
