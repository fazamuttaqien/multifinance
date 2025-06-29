package middleware

import (
	"errors"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

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
