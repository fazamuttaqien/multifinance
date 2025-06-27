package common

import (
	"errors"
	"os"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrTenorNotFound      = errors.New("tenor not found")
	ErrLimitNotSet        = errors.New("limit for this tenor is not set for the customer")
	ErrInvalidLimitAmount = errors.New("limit amount cannot be negative")
	ErrInsufficientLimit  = errors.New("insufficient limit for this transaction")
)

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

func SuccessResponse(c *fiber.Ctx, statusCode int, data any) error {
	return c.Status(statusCode).JSON(fiber.Map{
		"status": "success",
		"data":   data,
	})
}

func ErrorResponse(c *fiber.Ctx, statusCode int, message string) error {
	return c.Status(statusCode).JSON(fiber.Map{
		"status":  "success",
		"message": message,
	})
}
