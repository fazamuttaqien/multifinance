package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	partnerhandler "github.com/fazamuttaqien/multifinance/internal/handler/partner"
	"github.com/fazamuttaqien/multifinance/middleware"
	"github.com/fazamuttaqien/multifinance/pkg/common"
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

type PartnerHandlerTestSuite struct {
	suite.Suite
	app                *fiber.App
	handler            *partnerhandler.PartnerHandler
	mockPartnerService *MockPartnerService

	store     *session.Store
	jwtSecret string

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *PartnerHandlerTestSuite) SetupTest() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	suite.mockPartnerService = &MockPartnerService{}
	suite.store = session.New(session.Config{KeyLookup: "cookie:test-keylookup-partner"})
	suite.jwtSecret = "test-partner-secret-key"

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-partner-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-partner-handler-meter")

	suite.handler = partnerhandler.NewPartnerHandler(
		suite.mockPartnerService,
		suite.meter,
		suite.tracer,
		suite.log,
	)

	suite.app = suite.setupPartnerApp()
}

func (suite *PartnerHandlerTestSuite) setupPartnerApp() *fiber.App {
	app := fiber.New()

	jwtAuth := middleware.NewJWTAuthMiddleware(suite.jwtSecret)
	customCSRF := middleware.NewCustomCSRFMiddleware(suite.store)
	requireCustomer := middleware.RequireRole(domain.CustomerRole)

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

	partnerGroup := app.Group("/partners", jwtAuth, requireCustomer, customCSRF)
	{
		partnerGroup.Post("/check-limit", suite.handler.CheckLimit)
		partnerGroup.Post("/transactions", suite.handler.CreateTransaction)
	}

	return app
}

func (suite *PartnerHandlerTestSuite) getAuthCookieAndCsrfToken() (string, []*http.Cookie) {
	claims := &domain.JwtCustomClaims{
		UserID: 3,
		Role:   domain.CustomerRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 1)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(suite.jwtSecret))
	assert.NoError(suite.T(), err)

	jwtCookie := &http.Cookie{Name: "private", Value: signedToken}

	csrfReq := httptest.NewRequest(http.MethodGet, "/test/csrf-token", nil)
	csrfReq.AddCookie(jwtCookie)
	csrfResp, err := suite.app.Test(csrfReq)
	assert.NoError(suite.T(), err)
	defer csrfResp.Body.Close()

	var csrfBody map[string]string
	json.NewDecoder(csrfResp.Body).Decode(&csrfBody)
	csrfToken := csrfBody["csrf_token"]
	assert.NotEmpty(suite.T(), csrfToken)

	var allCookies []*http.Cookie
	allCookies = append(allCookies, jwtCookie)
	allCookies = append(allCookies, csrfResp.Cookies()...)

	return csrfToken, allCookies
}

func (suite *PartnerHandlerTestSuite) TestCheckLimit() {
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken()
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))
	requestBodyMap := map[string]any{
		"customer_nik":       nik,
		"tenor_months":       6,
		"transaction_amount": 5000.0,
	}

	suite.Run("Success - Limit Approved", func() {
		suite.mockPartnerService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "approved"}
		suite.mockPartnerService.MockError = nil
		req := createJSONRequestWithAuth(suite.T(), csrfToken, authCookies, http.MethodPost, "/partners/check-limit", requestBodyMap)
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	})

	suite.Run("Success - Limit Rejected", func() {
		suite.mockPartnerService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "rejected"}
		suite.mockPartnerService.MockError = nil
		req := createJSONRequestWithAuth(suite.T(), csrfToken, authCookies, http.MethodPost, "/partners/check-limit", requestBodyMap)

		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		assert.Equal(suite.T(), http.StatusUnprocessableEntity, resp.StatusCode)
	})

	suite.Run("Failure - Customer Not Found", func() {
		suite.mockPartnerService.MockError = common.ErrCustomerNotFound
		req := createJSONRequestWithAuth(suite.T(), csrfToken, authCookies, http.MethodPost, "/partners/check-limit", requestBodyMap)

		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	})
}

func (suite *PartnerHandlerTestSuite) TestCreateTransaction() {
	csrfToken, authCookies := suite.getAuthCookieAndCsrfToken()
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))
	requestBodyMap := map[string]any{
		"customer_nik": nik,
		"tenor_months": 6,
		"asset_name":   "Laptop",
		"otr_amount":   10000.0,
		"admin_fee":    500.0,
	}

	suite.Run("Success - Transaction Created", func() {
		suite.mockPartnerService.MockCreateTransactionResult = &domain.Transaction{ID: 1, AssetName: "Laptop"}
		suite.mockPartnerService.MockError = nil
		req := createJSONRequestWithAuth(suite.T(), csrfToken, authCookies, http.MethodPost, "/partners/transactions", requestBodyMap)
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()
		assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)
	})

	suite.Run("Failure - Insufficient Limit", func() {
		suite.mockPartnerService.MockError = common.ErrInsufficientLimit
		req := createJSONRequestWithAuth(suite.T(), csrfToken, authCookies, http.MethodPost, "/partners/transactions", requestBodyMap)

		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		assert.Equal(suite.T(), http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func (suite *PartnerHandlerTestSuite) TestPartnerRoutes_Security() {
	requestBodyMap := map[string]any{"customer_nik": "1234567890123456"}

	suite.Run("Failure - No Auth Cookie", func() {
		req := createJSONRequestWithAuth(suite.T(), "dummy-csrf", nil, http.MethodPost, "/partners/check-limit", requestBodyMap)
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()
		assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode)
	})

	suite.Run("Failure - Wrong Role (Admin)", func() {
		claims := &domain.JwtCustomClaims{UserID: 1, Role: domain.AdminRole}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, _ := token.SignedString([]byte(suite.jwtSecret))
		cookie := &http.Cookie{Name: "jwt_auth_token", Value: signedToken}

		req := createJSONRequestWithAuth(suite.T(), "dummy-csrf", []*http.Cookie{cookie}, http.MethodPost, "/partners/check-limit", requestBodyMap)
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()
		assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode)
	})

	suite.Run("Failure - No CSRF Header", func() {
		_, authCookies := suite.getAuthCookieAndCsrfToken()
		req := createJSONRequestWithAuth(suite.T(), "", authCookies, http.MethodPost, "/partners/check-limit", requestBodyMap)
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()
		assert.Equal(suite.T(), http.StatusForbidden, resp.StatusCode)
	})
}

func TestPartnerHandlerSuite(t *testing.T) {
	suite.Run(t, new(PartnerHandlerTestSuite))
}

func createJSONRequestWithAuth(t *testing.T, csrfToken string, cookies []*http.Cookie, method, url string, body map[string]interface{}) *http.Request {
	jsonBody, err := json.Marshal(body)
	assert.NoError(t, err, "Failed to marshal request body")

	req := httptest.NewRequest(method, url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}
