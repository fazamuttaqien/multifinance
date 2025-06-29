package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	adminhandler "github.com/fazamuttaqien/multifinance/internal/handler/admin"
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

type AdminHandlerTestSuite struct {
	suite.Suite
	app              *fiber.App
	handler          *adminhandler.AdminHandler
	mockAdminService *MockAdminService

	store     *session.Store
	jwtSecret string

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *AdminHandlerTestSuite) SetupTest() {
	suite.mockAdminService = &MockAdminService{}

	// Setup dependensi auth & CSRF
	suite.store = session.New(session.Config{
		KeyLookup: "cookie:test-keylookup-admin", // Gunakan key lookup yang berbeda
	})
	suite.jwtSecret = "test-admin-secret-key"

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-admin-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-admin-handler-meter")

	suite.handler = adminhandler.NewAdminHandler(
		suite.mockAdminService,
		suite.meter,
		suite.tracer,
		suite.log,
	)

	suite.app = suite.setupAdminApp()
}

func (suite *AdminHandlerTestSuite) setupAdminApp() *fiber.App {
	app := fiber.New()

	jwtAuth := middleware.NewJWTAuthMiddleware(suite.jwtSecret)
	customCSRF := middleware.NewCustomCSRFMiddleware(suite.store)
	requireAdmin := middleware.RequireRole(domain.AdminRole)

	// Endpoint ini diperlukan oleh helper getAuthCookieAndCsrfToken
	app.Get("/test/csrf-token", func(c *fiber.Ctx) error {
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

	adminGroup := app.Group("/admin", jwtAuth, requireAdmin) // Melindungi seluruh grup admin
	{
		adminGroup.Get("/customers", suite.handler.ListCustomers)
		adminGroup.Get("/customers/:customerId", suite.handler.GetCustomerByID)
		adminGroup.Post("/customers/:customerId/verify", customCSRF, suite.handler.VerifyCustomer)
		adminGroup.Post("/customers/:customerId/limits", customCSRF, suite.handler.SetLimits)
	}

	return app
}

func (suite *AdminHandlerTestSuite) getAuthCookieAndCsrfToken() (string, []*http.Cookie) {
	// 1. Buat token JWT untuk user admin (ID 1)
	claims := &domain.JwtCustomClaims{
		UserID: 1, // Admin ID
		Role:   domain.AdminRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(suite.jwtSecret))
	assert.NoError(suite.T(), err)

	jwtCookie := &http.Cookie{
		Name:  "private", // Gunakan nama cookie yang sesuai dengan implementasi
		Value: signedToken,
	}

	// 2. Dapatkan CSRF token dengan menggunakan cookie JWT yang sudah dibuat
	csrfReq := httptest.NewRequest(http.MethodGet, "/test/csrf-token", nil)
	csrfReq.AddCookie(jwtCookie)

	csrfResp, err := suite.app.Test(csrfReq)
	assert.NoError(suite.T(), err)
	defer csrfResp.Body.Close()

	var csrfBody map[string]string
	err = json.NewDecoder(csrfResp.Body).Decode(&csrfBody)
	assert.NoError(suite.T(), err)
	csrfToken := csrfBody["csrf_token"]
	assert.NotEmpty(suite.T(), csrfToken)

	// 3. Gabungkan cookie JWT dan cookie sesi
	var allCookies []*http.Cookie
	allCookies = append(allCookies, jwtCookie)
	allCookies = append(allCookies, csrfResp.Cookies()...)

	return csrfToken, allCookies
}

func (suite *AdminHandlerTestSuite) TestListCustomers_Success() {
	// Arrange: Dapatkan auth artifacts
	_, authCookies := suite.getAuthCookieAndCsrfToken()
	suite.mockAdminService.MockListCustomersResult = &domain.Paginated{Data: []domain.Customer{{ID: 2}}}
	suite.mockAdminService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/admin/customers?status=PENDING", nil)
	// Tambahkan cookie ke request
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestGetCustomerByID_Success() {
	// Arrange
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken()
	suite.mockAdminService.MockGetCustomerByIDResult = &domain.Customer{ID: 2, FullName: "Test Customer"}
	suite.mockAdminService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/admin/customers/2", nil)
	req.Header.Set("X-CSRF-Token", csrfToken) // GET tidak perlu, tapi tidak masalah jika ada
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	var customer domain.Customer
	json.NewDecoder(resp.Body).Decode(&customer)
	assert.Equal(suite.T(), uint64(2), customer.ID)
}

func (suite *AdminHandlerTestSuite) TestVerifyCustomer_Success() {
	// Arrange
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken()
	suite.mockAdminService.MockError = nil

	body := `{"status": "VERIFIED"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken) // Diperlukan untuk POST
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestSetLimits_Success() {
	// Arrange
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken()
	suite.mockAdminService.MockError = nil

	body := `{"limits": [{"tenor_months": 3, "limit_amount": 1000}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/limits", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken) // Diperlukan untuk POST
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestAdminRoutes_FailWithoutAuth() {
	// Arrange
	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil) // Tanpa cookie

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode, "Should fail without JWT cookie")
}

func (suite *AdminHandlerTestSuite) TestAdminRoutes_FailWithWrongRole() {
	// Arrange: Buat cookie dengan role Customer, bukan Admin
	claims := &domain.JwtCustomClaims{
		UserID: 2,
		Role:   domain.CustomerRole, // Role yang salah
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString([]byte(suite.jwtSecret))
	jwtCookie := &http.Cookie{Name: "jwt_auth_token", Value: signedToken}

	req := httptest.NewRequest(http.MethodGet, "/admin/customers", nil)
	req.AddCookie(jwtCookie)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode, "Should fail with incorrect role")
}

func (suite *AdminHandlerTestSuite) TestAdminPostRoutes_FailWithoutCsrfHeader() {
	// Arrange
	_, authCookies := suite.getAuthCookieAndCsrfToken() // Kita punya auth, tapi tidak mengirim header CSRF

	body := `{"status": "VERIFIED"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Header X-CSRF-Token sengaja tidak diset
	for _, c := range authCookies {
		req.AddCookie(c)
	}

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusForbidden, resp.StatusCode, "Should fail without CSRF token header")
}

func TestAdminHandlerSuite(t *testing.T) {
	suite.Run(t, new(AdminHandlerTestSuite))
}
