package handler_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupAppWithAuth(handler *handler.ProfileHandler) *fiber.App {
	app := fiber.New()

	// Middleware dummy untuk menyetel customerID
	authMiddleware := func(c *fiber.Ctx) error {
		c.Locals("customerID", uint64(2))
		return c.Next()
	}

	app.Post("/register", handler.Register)
	app.Get("/profile", authMiddleware, handler.GetMyProfile)
	app.Put("/profile", authMiddleware, handler.UpdateMyProfile)
	app.Get("/limits", authMiddleware, handler.GetMyLimits)
	app.Get("/transactions", authMiddleware, handler.GetMyTransactions)

	return app
}

// createMultipartRequest adalah helper untuk membuat request multipart/form-data
func createMultipartRequest(t *testing.T, fields map[string]string, files map[string]string) (*http.Request, string) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Tulis field teks
	for key, val := range fields {
		assert.NoError(t, writer.WriteField(key, val))
	}

	// Tulis field file
	for key, path := range files {
		part, err := writer.CreateFormFile(key, path)
		assert.NoError(t, err)
		
		// Tulis konten dummy ke file part
		_, err = io.WriteString(part, "dummy content")
		assert.NoError(t, err)
	}

	assert.NoError(t, writer.Close()) // Close untuk menulis boundary akhir

	req := httptest.NewRequest(http.MethodPost, "/register", body)
	return req, writer.FormDataContentType()
}
