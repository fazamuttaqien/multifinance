package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	adminhandler "github.com/fazamuttaqien/multifinance/internal/handler/admin"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"github.com/gofiber/fiber/v2"

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

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *AdminHandlerTestSuite) SetupTest() {
	// Reset mock service for each test
	suite.mockAdminService = &MockAdminService{}

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-admin-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-admin-handler-meter")

	// Create handler with dependencies
	suite.handler = adminhandler.NewAdminHandler(
		suite.mockAdminService,
		suite.meter,
		suite.tracer,
		suite.log,
	)

	// Setup fiber app with routes
	suite.app = suite.setupAdminApp()
}

func (suite *AdminHandlerTestSuite) setupAdminApp() *fiber.App {
	app := fiber.New()

	adminGroup := app.Group("/admin")

	adminGroup.Get("/customers", suite.handler.ListCustomers)
	adminGroup.Get("/customers/:customerId", suite.handler.GetCustomerByID)
	adminGroup.Post("/customers/:customerId/verify", suite.handler.VerifyCustomer)
	adminGroup.Post("/customers/:customerId/limits", suite.handler.SetLimits)

	return app
}

func (suite *AdminHandlerTestSuite) TestListCustomers_Success() {
	// Arrange
	suite.mockAdminService.MockListCustomersResult = &domain.Paginated{
		Data:       []domain.Customer{{ID: 2}},
		Total:      1,
		Page:       1,
		Limit:      10,
		TotalPages: 1,
	}
	suite.mockAdminService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/admin/customers?status=PENDING", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestGetCustomerByID_Success() {
	// Arrange
	suite.mockAdminService.MockGetCustomerByIDResult = &domain.Customer{ID: 2, FullName: "Test Customer"}
	suite.mockAdminService.MockError = nil

	req := httptest.NewRequest(http.MethodGet, "/admin/customers/2", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	var customer domain.Customer
	json.NewDecoder(resp.Body).Decode(&customer)
	assert.Equal(suite.T(), uint(2), uint(customer.ID))
}

func (suite *AdminHandlerTestSuite) TestGetCustomerByID_CustomerNotFound() {
	// Arrange
	suite.mockAdminService.MockGetCustomerByIDResult = nil
	suite.mockAdminService.MockError = common.ErrCustomerNotFound

	req := httptest.NewRequest(http.MethodGet, "/admin/customers/99", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestGetCustomerByID_InvalidCustomerID() {
	// Arrange
	req := httptest.NewRequest(http.MethodGet, "/admin/customers/abc", nil)

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestVerifyCustomer_Success() {
	// Arrange
	suite.mockAdminService.MockError = nil

	body := `{"status": "VERIFIED"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestVerifyCustomer_InvalidRequestBody() {
	// Arrange
	body := `{"status": "INVALID_STATUS"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestSetLimits_Success() {
	// Arrange
	suite.mockAdminService.MockError = nil

	body := `{"limits": [{"tenor_months": 3, "limit_amount": 1000}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/limits", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

func (suite *AdminHandlerTestSuite) TestSetLimits_ServiceReturnsNotFound() {
	// Arrange
	suite.mockAdminService.MockError = common.ErrTenorNotFound

	body := `{"limits": [{"tenor_months": 99, "limit_amount": 1000}]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/customers/2/limits", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Act
	resp, _ := suite.app.Test(req)
	defer resp.Body.Close()

	// Assert
	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
}

// TestAdminHandlerSuite runs the test suite
func TestAdminHandlerSuite(t *testing.T) {
	suite.Run(t, new(AdminHandlerTestSuite))
}
