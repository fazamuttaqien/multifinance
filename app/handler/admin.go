package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/fazamuttaqien/multifinance/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AdminHandler struct {
	adminService    service.AdminServices
	validate        *validator.Validate
	meter           metric.Meter
	tracer          trace.Tracer
	log             *zap.Logger
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorCount      metric.Int64Counter
	responseSize    metric.Int64Histogram
}

func NewAdminHandler(
	adminService service.AdminServices,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) *AdminHandler {
	requestCount, err := meter.Int64Counter(
		"api.request.count",
		metric.WithDescription("Number of API requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		zap.L().Fatal("Failed to create request count metric", zap.Error(err))
	}

	requestDuration, err := meter.Float64Histogram(
		"api.request.duration",
		metric.WithDescription("Duration of API requests"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		zap.L().Fatal("Failed to create request duration metric", zap.Error(err))
	}

	errorCount, err := meter.Int64Counter(
		"api.error.count",
		metric.WithDescription("Number of API errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		zap.L().Fatal("Failed to create error count metric", zap.Error(err))
	}

	responseSize, err := meter.Int64Histogram(
		"api.response.size",
		metric.WithDescription("Size of API responses in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		zap.L().Fatal("Failed to create response size metric", zap.Error(err))
	}

	return &AdminHandler{
		adminService:    adminService,
		validate:        validator.New(validator.WithRequiredStructEnabled()),
		meter:           meter,
		tracer:          tracer,
		log:             log,
		requestCount:    requestCount,
		requestDuration: requestDuration,
		errorCount:      errorCount,
		responseSize:    responseSize,
	}
}

// recordError helper function to record errors with observability
func (h *AdminHandler) recordError(
	ctx context.Context, span trace.Span, c *fiber.Ctx,
	start time.Time, err error, statusCode int, errorType, message string, fields ...zap.Field) error {
	// Record error metrics
	h.errorCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
		attribute.String("error_type", errorType),
		attribute.Int("status_code", statusCode),
	))

	// Record request duration
	duration := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds
	h.requestDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
		attribute.Int("status_code", statusCode),
	))

	// Set span attributes for error
	span.SetAttributes(
		attribute.String("error.type", errorType),
		attribute.String("error.message", err.Error()),
		attribute.Int("http.status_code", statusCode),
	)
	span.RecordError(err)

	// Log error
	logFields := append([]zap.Field{
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
		zap.Int("status_code", statusCode),
		zap.String("error_type", errorType),
		zap.Float64("duration_ms", duration),
		zap.Error(err),
	}, fields...)

	h.log.Error(message, logFields...)

	// Return HTTP error response
	return c.Status(statusCode).JSON(fiber.Map{"error": message})
}

// recordSuccess helper function to record successful responses with observability
func (h *AdminHandler) recordSuccess(
	ctx context.Context, span trace.Span, c *fiber.Ctx,
	start time.Time, statusCode int, responseData interface{}, fields ...zap.Field) error {
	// Record request duration
	duration := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds
	h.requestDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
		attribute.Int("status_code", statusCode),
	))

	// Set span attributes for success
	span.SetAttributes(
		attribute.Int("http.status_code", statusCode),
		attribute.Float64("request.duration_ms", duration),
	)

	// Log success
	logFields := append([]zap.Field{
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
		zap.Int("status_code", statusCode),
		zap.Float64("duration_ms", duration),
	}, fields...)

	h.log.Info("Request completed successfully", logFields...)

	// Return HTTP success response
	return c.Status(statusCode).JSON(responseData)
}

func (h *AdminHandler) ListCustomers(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.ListCustomers")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received list customers request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	params := domain.Params{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
	}

	span.SetAttributes(
		attribute.String("query.status", params.Status),
		attribute.Int("query.page", params.Page),
		attribute.Int("query.limit", params.Limit),
	)

	res, err := h.adminService.ListCustomers(ctx, params)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to list customers")
	}
	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, res)
}

func (h *AdminHandler) GetCustomerByID(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.GetCustomerByID")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received get customer by ID request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Invalid customer ID")
	}

	span.SetAttributes(attribute.Int64("customer.id", int64(customerID)))

	customer, err := h.adminService.GetCustomerByID(ctx, customerID)
	if err != nil {
		if errors.Is(err, common.ErrCustomerNotFound) {
			return h.recordError(ctx, span, c, start, err, fiber.StatusNotFound, "not_found", "Customer not found")
		}
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to get customer")
	}
	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, customer)
}

func (h *AdminHandler) VerifyCustomer(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.VerifyCustomer")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received verify customer request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Invalid customer ID")
	}

	var req dto.VerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Cannot parse request body")
	}

	if err := h.validate.Struct(req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "validation_error", err.Error())
	}

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("verification.new_status", string(req.Status)),
	)

	if err := h.adminService.VerifyCustomer(ctx, customerID, req); err != nil {
		if errors.Is(err, common.ErrCustomerNotFound) {
			return h.recordError(ctx, span, c, start, err, fiber.StatusNotFound, "not_found", "Customer not found")
		}
		// This can also be an invalid state transition error, which is a client error.
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "service_error", err.Error())
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, fiber.Map{"message": "Customer verification status updated"})
}

func (h *AdminHandler) SetLimits(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.SetLimits")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received set limits request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := strconv.ParseUint(c.Params("customerId"), 10, 64)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Invalid customer ID format")
	}

	var req dto.SetLimits
	if err := c.BodyParser(&req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Cannot parse request body")
	}

	if err := h.validate.Struct(req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "validation_error", "Validation failed: "+err.Error())
	}

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("limits.count", len(req.Limits)),
	)

	if err := h.adminService.SetLimits(ctx, customerID, req); err != nil {
		switch {
		case errors.Is(err, common.ErrCustomerNotFound), errors.Is(err, common.ErrTenorNotFound):
			return h.recordError(ctx, span, c, start, err, fiber.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, common.ErrInvalidLimitAmount):
			return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "invalid_request", err.Error())
		default:
			return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "An internal server error occurred")
		}
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, fiber.Map{
		"message": "Customer limits updated successfully",
	})
}
