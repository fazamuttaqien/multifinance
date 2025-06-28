package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/stretchr/testify/assert"
)

func TestAdminHandler_ListCustomers(t *testing.T) {
	// Arrange
	mockService := &mockAdminService{}
	handler := handler.NewAdminHandler(mockService)
	app := setupAdminApp(handler)

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockListCustomersResult = &domain.Paginated{
			Data:  []domain.Customer{{ID: 2}},
			Total: 1, Page: 1, Limit: 10, TotalPages: 1,
		}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodGet, "/admin/customers?status=PENDING", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAdminHandler_GetCustomerByID(t *testing.T) {
	// Arrange
	mockService := &mockAdminService{}
	handler := handler.NewAdminHandler(mockService)
	app := setupAdminApp(handler)

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockGetCustomerByIDResult = &domain.Customer{ID: 2, FullName: "Test Customer"}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodGet, "/admin/customers/2", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var customer domain.Customer
		json.NewDecoder(resp.Body).Decode(&customer)
		assert.Equal(t, uint(2), customer.ID)
	})

	t.Run("Customer Not Found", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockGetCustomerByIDResult = nil
		mockService.MockError = common.ErrCustomerNotFound // Gunakan error yang didefinisikan

		req := httptest.NewRequest(http.MethodGet, "/admin/customers/99", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Invalid Customer ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/customers/abc", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAdminHandler_VerifyCustomer(t *testing.T) {
	// Arrange
	mockService := &mockAdminService{}
	handler := handler.NewAdminHandler(mockService)
	app := setupAdminApp(handler)

	t.Run("Success", func(t *testing.T) {
		mockService.MockError = nil

		body := `{"status": "VERIFIED"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Invalid Request Body", func(t *testing.T) {
		// Body tidak valid (misal, status salah)
		body := `{"status": "INVALID_STATUS"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAdminHandler_SetLimits(t *testing.T) {
	// Arrange
	mockService := &mockAdminService{}
	handler := handler.NewAdminHandler(mockService)
	app := setupAdminApp(handler)

	t.Run("Success", func(t *testing.T) {
		mockService.MockError = nil

		body := `{"limits": [{"tenor_months": 3, "limit_amount": 1000}]}`
		req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/limits", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Service returns Not Found", func(t *testing.T) {
		mockService.MockError = common.ErrTenorNotFound // Atau ErrCustomerNotFound

		body := `{"limits": [{"tenor_months": 99, "limit_amount": 1000}]}` // Tenor 99 tidak ada
		req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/limits", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
