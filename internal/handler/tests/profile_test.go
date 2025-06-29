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
	"github.com/fazamuttaqien/multifinance/middleware"
	"github.com/golang-jwt/jwt/v5"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"

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

	store     *session.Store
	jwtSecret string

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *ProfileHandlerTestSuite) SetupTest() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	suite.mockProfileService = &MockProfileService{}
	suite.mockCloudinary = &MockCloudinaryService{}

	suite.store = session.New(session.Config{
		KeyLookup: "cookie:test-keylookup",
	})
	suite.jwtSecret = "test-secret-key"

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-profile-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-profile-handler-meter")

	suite.handler = profilehandler.NewProfileHandler(
		suite.mockProfileService,
		suite.mockCloudinary,
		suite.meter,
		suite.tracer,
		suite.log,
	)

	suite.app = suite.setupProfileApp()
}

func (suite *ProfileHandlerTestSuite) setupProfileApp() *fiber.App {
	app := fiber.New()

	jwtAuth := middleware.NewJWTAuthMiddleware(suite.jwtSecret)
	customCSRF := middleware.NewCustomCSRFMiddleware(suite.store)
	requireCustomer := middleware.RequireRole(domain.CustomerRole)

	app.Post("/register", customCSRF, suite.handler.Register)

	app.Get("/csrf-token", func(c *fiber.Ctx) error {
		sess, _ := suite.store.Get(c)
		token := sess.Get("csrf_token")
		if token == nil {
			newToken, _ := middleware.GenerateCSRFToken()
			sess.Set("csrf_token", newToken)
			sess.Save()
			token = newToken
		}
		return c.JSON(fiber.Map{"csrf_token": token})
	})

	meApi := app.Group("/me", jwtAuth, requireCustomer)
	{
		meApi.Get("/profile", suite.handler.GetMyProfile)
		meApi.Put("/profile", customCSRF, suite.handler.UpdateMyProfile)
		meApi.Get("/limits", suite.handler.GetMyLimits)
		meApi.Get("/transactions", suite.handler.GetMyTransactions)
	}

	return app
}

func (suite *ProfileHandlerTestSuite) getCsrfToken() (string, []*http.Cookie) {
	req := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	var body map[string]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(suite.T(), err)

	return body["csrf_token"], resp.Cookies()
}

func (suite *ProfileHandlerTestSuite) getAuthCookieAndCsrfToken(userID uint64, role domain.Role) (string, []*http.Cookie) {
	claims := &domain.JwtCustomClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(suite.jwtSecret))
	assert.NoError(suite.T(), err)

	jwtCookie := &http.Cookie{
		Name:  "private",
		Value: signedToken,
	}

	csrfReq := httptest.NewRequest(http.MethodGet, "/csrf-token", nil)
	csrfReq.AddCookie(jwtCookie)

	csrfResp, err := suite.app.Test(csrfReq)
	assert.NoError(suite.T(), err)
	defer csrfResp.Body.Close()

	var csrfBody map[string]string
	err = json.NewDecoder(csrfResp.Body).Decode(&csrfBody)
	assert.NoError(suite.T(), err)
	csrfToken := csrfBody["csrf_token"]
	assert.NotEmpty(suite.T(), csrfToken)

	var sessionCookie *http.Cookie
	for _, c := range csrfResp.Cookies() {
		if strings.Contains(c.Name, "test-keylookup") {
			sessionCookie = c
			break
		}
	}
	assert.NotNil(suite.T(), sessionCookie, "Session cookie not found in response")

	return csrfToken, []*http.Cookie{jwtCookie, sessionCookie}
}

func (suite *ProfileHandlerTestSuite) TestRegister_Success() {
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))

	csrfToken, sessionCookies := suite.getCsrfToken()
	assert.NotEmpty(suite.T(), csrfToken)

	fields := map[string]string{
		"nik":         nik,
		"full_name":   "Alan Smith",
		"legal_name":  "Alan Smith",
		"password":    "alansmith123",
		"birth_place": "Surabaya",
		"birth_date":  "1990-05-15",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	suite.mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
	suite.mockCloudinary.MockUploadError = nil

	birthDate, err := time.Parse("2006-01-02", fields["birth_date"])
	assert.NoError(suite.T(), err)

	suite.mockProfileService.MockRegisterResult = &domain.Customer{
		ID:         1,
		NIK:        fields["nik"],
		FullName:   fields["full_name"],
		LegalName:  fields["legal_name"],
		BirthPlace: fields["birth_place"],
		BirthDate:  birthDate,
		Salary:     5000000,
		KtpUrl:     suite.mockCloudinary.MockUploadURL,
		SelfieUrl:  suite.mockCloudinary.MockUploadURL,
	}
	suite.mockProfileService.MockError = nil

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range sessionCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)
	var result domain.Customer
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), fields["nik"], result.NIK)
}

func (suite *ProfileHandlerTestSuite) TestRegister_CloudinaryUploadFails() {
	csrfToken, sessionCookies := suite.getCsrfToken()

	fields := map[string]string{
		"nik":         "1234567890123456",
		"full_name":   "Test User",
		"legal_name":  "TEST USER",
		"password":    "testpass123",
		"birth_place": "Test City",
		"birth_date":  "2000-01-01",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	suite.mockCloudinary.MockUploadError = errors.New("connection timeout")

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range sessionCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusInternalServerError, resp.StatusCode)
}

func (suite *ProfileHandlerTestSuite) TestRegister_ServiceReturnsConflict() {
	csrfToken, sessionCookies := suite.getCsrfToken()

	fields := map[string]string{
		"nik":         "1234567890123456",
		"full_name":   "Test User",
		"legal_name":  "TEST USER",
		"password":    "testpass123",
		"birth_place": "Test City",
		"birth_date":  "2000-01-01",
		"salary":      "5000000",
	}
	files := map[string]string{"ktp_photo": "ktp.jpg", "selfie_photo": "selfie.jpg"}

	suite.mockCloudinary.MockUploadURL = "http://fake-url.com/image.jpg"
	suite.mockCloudinary.MockUploadError = nil
	suite.mockProfileService.MockError = errors.New("nik already registered")

	req, contentType := createMultipartRequest(suite.T(), fields, files)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", csrfToken)
	for _, c := range sessionCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusConflict, resp.StatusCode)
}

func (suite *ProfileHandlerTestSuite) TestGetMyProfile_Success() {
	_, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)

	suite.mockProfileService.MockGetMyProfileResult = &domain.Customer{
		ID:       2,
		FullName: "Alan Smith",
		Role:     domain.CustomerRole,
	}
	suite.mockProfileService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	var customer domain.Customer
	err = json.NewDecoder(resp.Body).Decode(&customer)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), uint64(2), customer.ID)
}

func (suite *ProfileHandlerTestSuite) TestUpdateMyProfile_Success() {
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)
	suite.mockProfileService.MockError = nil

	updateBody := `{"full_name": "Jane Doe", "salary": 12000000}`

	req := httptest.NewRequest(http.MethodPut, "/me/profile", strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *ProfileHandlerTestSuite) TestGetMyLimits_Success() {
	_, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)

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

	req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var actualLimits []dto.LimitDetailResponse
	err = json.NewDecoder(resp.Body).Decode(&actualLimits)
	assert.NoError(suite.T(), err)

	assert.Len(suite.T(), actualLimits, 2)
	assert.Equal(suite.T(), uint8(3), actualLimits[0].TenorMonths)
	assert.Equal(suite.T(), float64(800000), actualLimits[0].RemainingLimit)
	assert.Equal(suite.T(), uint8(6), actualLimits[1].TenorMonths)
}

func (suite *ProfileHandlerTestSuite) TestGetMyLimits_ServiceReturnsError() {
	_, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)

	suite.mockProfileService.MockGetMyLimitsResult = nil
	suite.mockProfileService.MockError = errors.New("Failed to get limits")

	req := httptest.NewRequest(http.MethodGet, "/me/limits", nil)
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusInternalServerError, resp.StatusCode)

	var bodyMap map[string]string
	err = json.NewDecoder(resp.Body).Decode(&bodyMap)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), bodyMap["error"], "Failed to get limits")
}

func (suite *ProfileHandlerTestSuite) TestGetMyTransactions_SuccessWithQueryParameters() {
	_, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)

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

	req := httptest.NewRequest(http.MethodGet, "/me/transactions?status=ACTIVE&page=1&limit=5", nil)
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

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
	_, authCookies := suite.getAuthCookieAndCsrfToken(2, domain.CustomerRole)

	suite.mockProfileService.MockGetMyTransactionsResult = &domain.Paginated{
		Data:       []domain.Transaction{},
		Total:      0,
		Page:       1,
		Limit:      10, // Default yang disetel di handler
		TotalPages: 0,
	}
	suite.mockProfileService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/me/transactions", nil)
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	resp, err := suite.app.Test(req)
	assert.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var actualResponse domain.Paginated
	err = json.NewDecoder(resp.Body).Decode(&actualResponse)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, actualResponse.Page)
	assert.Equal(suite.T(), 10, actualResponse.Limit)
}

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
