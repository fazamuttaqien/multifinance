package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func NewCustomCSRFMiddleware(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Lewati pemeriksaan untuk metode yang tidak mengubah state
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// Ambil sesi dari store
		sess, err := store.Get(c)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Session error"})
		}

		// Ambil token CSRF yang tersimpan di sesi
		storedToken := sess.Get("csrf_token")
		if storedToken == nil {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "CSRF token not found in session"})
		}

		// Ambil token CSRF yang dikirim oleh klien (dari header)
		clientToken := c.Get("X-CSRF-Token")
		if clientToken == "" {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "CSRF token missing from request header"})
		}

		// Bandingkan token
		if clientToken != storedToken.(string) {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "CSRF token mismatch"})
		}

		return c.Next()
	}
}
