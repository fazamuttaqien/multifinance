package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/handler"
	"github.com/stretchr/testify/assert"
)

func TestProfileHandler_Register(t *testing.T) {
	// Arrange
	mockProfileService := &mockProfileService{}
	mockCloudinary := &mockCloudinaryService{}

	handler := handler.NewProfileHandler(mockProfileService, mockCloudinary)

	app := setupAppWithAuth(handler)

	// Data dummy untuk request
	fields := map[string]string{
		"nik":         "1234567890123456",
		"full_name":   "Test User",
		"legal_name":  "TEST USER",
		"birth_place": "Test City",
		"birth_date":  "2000-01-01T00:00:00Z",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
		mockCloudinary.MockUploadError = nil
		mockProfileService.MockRegisterResult = &domain.Customer{ID: 1, NIK: fields["nik"]}
		mockProfileService.MockError = nil

		req, contentType := createMultipartRequest(t, fields, files)
		req.Header.Set("Content-Type", contentType)

		// Act
		resp, err := app.Test(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result domain.Customer
		err = json.NewDecoder(resp.Body).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), result.ID)
		assert.Equal(t, fields["nik"], result.NIK)
	})

	t.Run("Cloudinary Upload Fails", func(t *testing.T) {
		// Konfigurasi mock
		mockCloudinary.MockUploadError = errors.New("connection timeout")

		req, contentType := createMultipartRequest(t, fields, files)
		req.Header.Set("Content-Type", contentType)

		// Act
		resp, _ := app.Test(req)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("Service Returns Conflict (NIK Exists)", func(t *testing.T) {
		// Konfigurasi mock
		mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
		mockCloudinary.MockUploadError = nil
		mockProfileService.MockError = errors.New("nik already registered")

		req, contentType := createMultipartRequest(t, fields, files)
		req.Header.Set("Content-Type", contentType)

		// Act
		resp, _ := app.Test(req)

		// Assert
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestProfileHandler_GetMyProfile(t *testing.T) {
	// Arrange
	mockService := &mockProfileService{}
	handler := handler.NewProfileHandler(mockService, nil)
	app := setupAppWithAuth(handler)

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockGetMyProfileResult = &domain.Customer{ID: 2, FullName: "Authenticated User"}

		req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var customer domain.Customer
		json.NewDecoder(resp.Body).Decode(&customer)
		assert.Equal(t, uint(2), customer.ID)
		assert.Equal(t, "Authenticated User", customer.FullName)
	})
}

func TestProfileHandler_UpdateMyProfile(t *testing.T) {
	// Arrange
	mockService := &mockProfileService{}
	handler := handler.NewProfileHandler(mockService, nil)
	app := setupAppWithAuth(handler)

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockError = nil

		// Buat body request
		updateBody := `{"full_name": "Updated Name", "salary": 12000000}`
		req := httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var bodyMap map[string]string
		json.NewDecoder(resp.Body).Decode(&bodyMap)
		assert.Equal(t, "Profile updated successfully", bodyMap["message"])
	})
}

func TestProfileHandler_GetMyLimits(t *testing.T) {
	// Arrange
	mockService := &mockProfileService{}
	handler := handler.NewProfileHandler(mockService, nil)
	app := setupAppWithAuth(handler)

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock untuk mengembalikan data limit
		expectedLimits := []dto.LimitDetail{
			{
				TenorMonths:    3,
				LimitAmount:    1000000,
				UsedAmount:     200000,
				RemainingLimit: 800000,
			},
			{
				TenorMonths:    6,
				LimitAmount:    2000000,
				UsedAmount:     0,
				RemainingLimit: 2000000,
			},
		}
		mockService.MockGetMyLimitsResult = expectedLimits
		mockService.MockError = nil

		// Buat request HTTP
		req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)

		// Act
		resp, err := app.Test(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Baca dan validasi body response
		var actualLimits []dto.LimitDetail
		err = json.NewDecoder(resp.Body).Decode(&actualLimits)
		assert.NoError(t, err)

		assert.Len(t, actualLimits, 2)
		assert.Equal(t, uint8(3), actualLimits[0].TenorMonths)
		assert.Equal(t, float64(800000), actualLimits[0].RemainingLimit)
		assert.Equal(t, uint8(6), actualLimits[1].TenorMonths)
	})

	t.Run("Service returns error", func(t *testing.T) {
		// Konfigurasi mock untuk mengembalikan error
		mockService.MockGetMyLimitsResult = nil
		mockService.MockError = errors.New("database connection failed")

		req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var bodyMap map[string]string
		json.NewDecoder(resp.Body).Decode(&bodyMap)
		assert.Contains(t, bodyMap["error"], "database connection failed")
	})
}

func TestProfileHandler_GetMyTransactions(t *testing.T) {
	// Arrange
	mockService := &mockProfileService{}
	handler := handler.NewProfileHandler(mockService, nil)
	app := setupAppWithAuth(handler)

	t.Run("Success with query parameters", func(t *testing.T) {
		// Konfigurasi mock
		expectedResponse := &domain.Paginated{
			Data: []domain.Transaction{
				{ID: 1, AssetName: "Laptop"},
			},
			Total:      1,
			Page:       1,
			Limit:      5,
			TotalPages: 1,
		}
		mockService.MockGetMyTransactionsResult = expectedResponse
		mockService.MockError = nil

		// Buat request dengan query params
		req := httptest.NewRequest(http.MethodGet, "/me/transactions?status=ACTIVE&page=1&limit=5", nil)

		// Act
		resp, err := app.Test(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Baca dan validasi body response
		// Karena 'Data' adalah interface{}, kita perlu unmarshal ke struct dengan tipe yang sesuai
		var actualResponse struct {
			Data       []domain.Transaction `json:"data"`
			Total      int64                `json:"total"`
			Page       int                  `json:"page"`
			Limit      int                  `json:"limit"`
			TotalPages int                  `json:"total_pages"`
		}
		err = json.NewDecoder(resp.Body).Decode(&actualResponse)
		assert.NoError(t, err)

		assert.Equal(t, int64(1), actualResponse.Total)
		assert.Equal(t, 1, actualResponse.Page)
		assert.Equal(t, 5, actualResponse.Limit)
		assert.Len(t, actualResponse.Data, 1)
		assert.Equal(t, "Laptop", actualResponse.Data[0].AssetName)
	})

	t.Run("Success without query parameters (uses defaults)", func(t *testing.T) {
		// Konfigurasi mock
		mockService.MockGetMyTransactionsResult = &domain.Paginated{
			Data:       []domain.Transaction{},
			Total:      0,
			Page:       1,
			Limit:      10, // Ini adalah default yang disetel di handler
			TotalPages: 0,
		}
		mockService.MockError = nil

		req := httptest.NewRequest(http.MethodGet, "/me/transactions", nil)

		// Act
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var actualResponse domain.Paginated
		json.NewDecoder(resp.Body).Decode(&actualResponse)
		assert.Equal(t, 1, actualResponse.Page)
		assert.Equal(t, 10, actualResponse.Limit)
	})
}
