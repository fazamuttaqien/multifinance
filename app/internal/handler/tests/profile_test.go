package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	profilehandler "github.com/fazamuttaqien/multifinance/internal/handler/profile"

	"github.com/gofiber/fiber/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"go.opentelemetry.io/otel/metric"
	noop_metric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	noop_trace "go.opentelemetry.io/otel/trace/noop"

	"go.uber.org/zap"
)

type ProfileHandlerTestSuite struct {
	suite.Suite
	app                *fiber.App
	handler            *profilehandler.ProfileHandler
	mockProfileService *MockProfileService
	mockCloudinary     *MockCloudinaryService

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *ProfileHandlerTestSuite) SetupTest() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Reset mock services for each test
	suite.mockProfileService = &MockProfileService{}
	suite.mockCloudinary = &MockCloudinaryService{}

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-profile-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-profile-handler-meter")

	// Create handler with dependencies
	suite.handler = profilehandler.NewProfileHandler(
		suite.mockProfileService,
		suite.mockCloudinary,
		suite.meter,
		suite.tracer,
		suite.log,
	)

	// Setup fiber app with routes
	suite.app = suite.setupProfileApp()
}

func (suite *ProfileHandlerTestSuite) setupProfileApp() *fiber.App {
	app := fiber.New()

	authMiddleware := func(c *fiber.Ctx) error {
		c.Locals("customerID", uint64(2))
		return c.Next()
	}

	app.Post("/register", suite.handler.Register)

	app.Get("/me/profile", authMiddleware, suite.handler.GetMyProfile)
	app.Put("/me/profile", authMiddleware, suite.handler.UpdateMyProfile)
	app.Get("/me/limits", authMiddleware, suite.handler.GetMyLimits)
	app.Get("/me/transactions", authMiddleware, suite.handler.GetMyTransactions)

	return app
}

func (suite *ProfileHandlerTestSuite) TestRegister_Success() {
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))

	// Arrange
	fields := map[string]string{
		"nik":         nik,
		"full_name":   "Alan Smith",
		"legal_name":  "Alan Smith",
		"birth_place": "Surabaya",
		"birth_date":  "1990-05-15",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	suite.mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
	suite.mockProfileService.MockRegisterResult = &domain.Customer{
		ID:       1,
		NIK:      fields["nik"],
		FullName: fields["full_name"],
	}

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)

	// Act
	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)
	var result domain.Customer
	err = json.NewDecoder(resp.Body).Decode(&result)

	assert.NoError(suite.T(), err, "Failed to decode response body")
	assert.Equal(suite.T(), uint64(1), result.ID)
	assert.Equal(suite.T(), fields["nik"], result.NIK)
	assert.Equal(suite.T(), fields["full_name"], result.FullName)
}

func (suite *ProfileHandlerTestSuite) TestRegister_CloudinaryUploadFails() {
	// Arrange
	fields := map[string]string{
		"nik":         "1234567890123456",
		"full_name":   "Test User",
		"legal_name":  "TEST USER",
		"birth_place": "Test City",
		"birth_date":  "2000-01-01T00:00:00Z",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	// Konfigurasi mock
	suite.mockCloudinary.MockUploadError = errors.New("connection timeout")

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

func (suite *ProfileHandlerTestSuite) TestRegister_ServiceReturnsConflict() {
	// Arrange
	fields := map[string]string{
		"nik":         "1234567890123456",
		"full_name":   "Test User",
		"legal_name":  "TEST USER",
		"birth_place": "Test City",
		"birth_date":  "2000-01-01T00:00:00Z",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	// Konfigurasi mock
	suite.mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
	suite.mockCloudinary.MockUploadError = nil
	suite.mockProfileService.MockError = errors.New("nik already registered")

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

func (suite *ProfileHandlerTestSuite) TestGetMyProfile_Success() {
	// Arrange
	suite.mockProfileService.MockGetMyProfileResult = &domain.Customer{ID: 2, FullName: "Authenticated User"}

	req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var customer domain.Customer
	json.NewDecoder(resp.Body).Decode(&customer)
	assert.Equal(suite.T(), uint64(2), customer.ID)
	assert.Equal(suite.T(), "Authenticated User", customer.FullName)
}

func (suite *ProfileHandlerTestSuite) TestUpdateMyProfile_Success() {
	// Arrange
	suite.mockProfileService.MockError = nil

	// Buat body request
	updateBody := `{"full_name": "Updated Name", "salary": 12000000}`
	req := httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	var bodyMap map[string]string
	json.NewDecoder(resp.Body).Decode(&bodyMap)
	assert.Equal(suite.T(), "Profile updated successfully", bodyMap["message"])
}

func (suite *ProfileHandlerTestSuite) TestGetMyLimits_Success() {
	// Arrange
	expectedLimits := []dto.LimitDetailResponse{
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
	suite.mockProfileService.MockGetMyLimitsResult = expectedLimits
	suite.mockProfileService.MockError = nil

	// Buat request HTTP
	req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)

	// Act
	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	// Baca dan validasi body response
	var actualLimits []dto.LimitDetailResponse
	err = json.NewDecoder(resp.Body).Decode(&actualLimits)
	assert.NoError(suite.T(), err)

	assert.Len(suite.T(), actualLimits, 2)
	assert.Equal(suite.T(), uint8(3), actualLimits[0].TenorMonths)
	assert.Equal(suite.T(), float64(800000), actualLimits[0].RemainingLimit)
	assert.Equal(suite.T(), uint8(6), actualLimits[1].TenorMonths)
}

func (suite *ProfileHandlerTestSuite) TestGetMyLimits_ServiceReturnsError() {
	// Arrange
	suite.mockProfileService.MockGetMyLimitsResult = nil
	suite.mockProfileService.MockError = errors.New("Failed to get limits")

	req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusInternalServerError, resp.StatusCode)

	var bodyMap map[string]string
	json.NewDecoder(resp.Body).Decode(&bodyMap)
	assert.Contains(suite.T(), bodyMap["error"], "Failed to get limits")
}

func (suite *ProfileHandlerTestSuite) TestGetMyTransactions_SuccessWithQueryParameters() {
	// Arrange
	expectedResponse := &domain.Paginated{
		Data: []domain.Transaction{
			{ID: 1, AssetName: "Laptop"},
		},
		Total:      1,
		Page:       1,
		Limit:      5,
		TotalPages: 1,
	}
	suite.mockProfileService.MockGetMyTransactionsResult = expectedResponse
	suite.mockProfileService.MockError = nil

	// Buat request dengan query params
	req := httptest.NewRequest(http.MethodGet, "/me/transactions?status=ACTIVE&page=1&limit=5", nil)

	// Act
	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	// Baca dan validasi body response
	var actualResponse struct {
		Data       []domain.Transaction `json:"data"`
		Total      int64                `json:"total"`
		Page       int                  `json:"page"`
		Limit      int                  `json:"limit"`
		TotalPages int                  `json:"total_pages"`
	}
	err = json.NewDecoder(resp.Body).Decode(&actualResponse)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), int64(1), actualResponse.Total)
	assert.Equal(suite.T(), 1, actualResponse.Page)
	assert.Equal(suite.T(), 5, actualResponse.Limit)
	assert.Len(suite.T(), actualResponse.Data, 1)
	assert.Equal(suite.T(), "Laptop", actualResponse.Data[0].AssetName)
}

func (suite *ProfileHandlerTestSuite) TestGetMyTransactions_SuccessWithoutQueryParameters() {
	// Arrange
	suite.mockProfileService.MockGetMyTransactionsResult = &domain.Paginated{
		Data:       []domain.Transaction{},
		Total:      0,
		Page:       1,
		Limit:      10, // Default yang disetel di handler
		TotalPages: 0,
	}
	suite.mockProfileService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/me/transactions", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var actualResponse domain.Paginated
	json.NewDecoder(resp.Body).Decode(&actualResponse)
	assert.Equal(suite.T(), 1, actualResponse.Page)
	assert.Equal(suite.T(), 10, actualResponse.Limit)
}

// TestProfileHandlerSuite runs the test suite
func TestProfileHandlerSuite(t *testing.T) {
	suite.Run(t, new(ProfileHandlerTestSuite))
}

func createMultipartRequest(
	t *testing.T,
	fields, files map[string]string,
) (*http.Request, string) {

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	for key, val := range fields {
		assert.NoError(t, writer.WriteField(key, val))
	}

	for key, path := range files {
		part, err := writer.CreateFormFile(key, path)
		assert.NoError(t, err)

		_, err = io.WriteString(part, "dummy content")
		assert.NoError(t, err)
	}

	assert.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/register", body)
	return req, writer.FormDataContentType()
}
