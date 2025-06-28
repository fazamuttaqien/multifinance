package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/stretchr/testify/assert"
)

func TestPartnerHandler_CheckLimit(t *testing.T) {
	// Arrange
	mockService := &mockPartnerService{}
	handler := handler.NewPartnerHandler(mockService)
	app := setupPartnerApp(handler)

	validBody := `{"customer_nik": "1234567890123456", "tenor_months": 6, "transaction_amount": 5000}`

	t.Run("Success - Limit Approved", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "approved", Message: "Limit is sufficient."}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodPost, "/partners/check-limit", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result dto.CheckLimitResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "approved", result.Status)
	})

	t.Run("Success - Limit Rejected", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "rejected", Message: "Insufficient limit."}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodPost, "/partners/check-limit", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		var result dto.CheckLimitResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, "rejected", result.Status)
	})

	t.Run("Failure - Customer Not Found", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCheckLimitResult = nil
		mockService.MockError = common.ErrCustomerNotFound

		req := httptest.NewRequest(http.MethodPost, "/partners/check-limit", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Failure - Invalid Request Body", func(t *testing.T) {
		// Body dengan field yang hilang (tenor_months)
		invalidBody := `{"customer_nik": "1234567890123456", "transaction_amount": 5000}`
		req := httptest.NewRequest(http.MethodPost, "/partners/check-limit", strings.NewReader(invalidBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestPartnerHandler_CreateTransaction(t *testing.T) {
	// Arrange
	mockService := &mockPartnerService{}
	handler := handler.NewPartnerHandler(mockService)
	app := setupPartnerApp(handler)

	validBody := `{"customer_nik": "1234567890123456", "tenor_months": 6, "asset_name": "Laptop", "otr_amount": 10000, "admin_fee": 500}`

	t.Run("Success - Transaction Created", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCreateTransactionResult = &domain.Transaction{ID: 1, AssetName: "Laptop"}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodPost, "/partners/transactions", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result domain.Transaction
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(t, uint(1), result.ID)
	})

	t.Run("Failure - Insufficient Limit", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCreateTransactionResult = nil
		mockService.MockError = common.ErrInsufficientLimit

		req := httptest.NewRequest(http.MethodPost, "/partners/transactions", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		var bodyMap map[string]string
		json.NewDecoder(resp.Body).Decode(&bodyMap)
		assert.Equal(t, "rejected", bodyMap["status"])
		assert.Equal(t, common.ErrInsufficientLimit.Error(), bodyMap["reason"])
	})

	t.Run("Failure - Customer Not Found", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockCreateTransactionResult = nil
		mockService.MockError = common.ErrCustomerNotFound

		req := httptest.NewRequest(http.MethodPost, "/partners/transactions", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
