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

type PartnerHandlerTestSuite struct {
	suite.Suite
	app                *fiber.App
	handler            *partnerhandler.PartnerHandler
	mockPartnerService *MockPartnerService

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *PartnerHandlerTestSuite) SetupTest() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	suite.mockPartnerService = &MockPartnerService{}

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-profile-handler-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-profile-handler-meter")

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
	partnerGroup := app.Group("/partners")

	partnerGroup.Post("/check-limit", suite.handler.CheckLimit)
	partnerGroup.Post("/transactions", suite.handler.CreateTransaction)

	return app
}

func (suite *PartnerHandlerTestSuite) TestCheckLimit() {
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))

	requestBodyMap := map[string]any{
		"customer_nik":       nik,
		"tenor_months":       6,
		"transaction_amount": 5000.0,
	}

	suite.Run("Success - Limit Approved", func() {
		// Arrange
		suite.mockPartnerService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "approved"}
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/check-limit", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	})

	suite.Run("Success - Limit Rejected", func() {
		// Arrange
		suite.mockPartnerService.MockCheckLimitResult = &dto.CheckLimitResponse{Status: "rejected"}
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/check-limit", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusUnprocessableEntity, resp.StatusCode)
	})

	suite.Run("Failure - Customer Not Found", func() {
		// Arrange
		suite.mockPartnerService.MockError = common.ErrCustomerNotFound
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/check-limit", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	})

	suite.Run("Failure - Invalid Request Body", func() {
		// Arrange
		invalidBodyMap := map[string]interface{}{
			"customer_nik": nik,
		}
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/check-limit", invalidBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
	})
}

func (suite *PartnerHandlerTestSuite) TestCreateTransaction() {
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))

	requestBodyMap := map[string]any{
		"customer_nik": nik,
		"tenor_months": 6,
		"asset_name":   "Laptop",
		"otr_amount":   10000.0,
		"admin_fee":    500.0,
	}

	suite.Run("Success - Transaction Created", func() {
		// Arrange
		suite.mockPartnerService.MockCreateTransactionResult = &domain.Transaction{ID: 1, AssetName: "Laptop"}
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/transactions", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)
		var result domain.Transaction
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Equal(suite.T(), uint64(1), result.ID)
	})

	suite.Run("Failure - Insufficient Limit", func() {
		// Arrange
		suite.mockPartnerService.MockError = common.ErrInsufficientLimit
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/transactions", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusUnprocessableEntity, resp.StatusCode)
	})

	suite.Run("Failure - Customer Not Found", func() {
		// Arrange
		suite.mockPartnerService.MockError = common.ErrCustomerNotFound
		req := createJSONRequest(suite.T(), http.MethodPost, "/partners/transactions", requestBodyMap)

		// Act
		resp, _ := suite.app.Test(req)
		defer resp.Body.Close()

		// Assert
		assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	})
}

// TestPartnerHandlerSuite menjalankan seluruh test suite
func TestPartnerHandlerSuite(t *testing.T) {
	suite.Run(t, new(PartnerHandlerTestSuite))
}

func createJSONRequest(t *testing.T, method, url string, body map[string]interface{}) *http.Request {
	jsonBody, err := json.Marshal(body)
	assert.NoError(t, err, "Failed to marshal request body")

	req := httptest.NewRequest(method, url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}
