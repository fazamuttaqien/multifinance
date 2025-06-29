package partnerhandler

import (
	"context"
	"errors"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/service"
	"github.com/fazamuttaqien/multifinance/pkg/common"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type PartnerHandler struct {
	partnerService  service.PartnerServices
	validate        *validator.Validate
	meter           metric.Meter
	tracer          trace.Tracer
	log             *zap.Logger
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorCount      metric.Int64Counter
	responseSize    metric.Int64Histogram
}

func NewPartnerHandler(
	partnerService service.PartnerServices,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) *PartnerHandler {
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

	return &PartnerHandler{
		partnerService:  partnerService,
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
func (p *PartnerHandler) recordError(
	ctx context.Context, span trace.Span, c *fiber.Ctx,
	start time.Time, err error, statusCode int, errorType, message string, fields ...zap.Field) error {
	// Record error metrics
	p.errorCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
		attribute.String("error_type", errorType),
		attribute.Int("status_code", statusCode),
	))

	// Record request duration
	duration := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds
	p.requestDuration.Record(ctx, duration, metric.WithAttributes(
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
	}, fields...)

	p.log.Error(message, logFields...)

	// Return HTTP error response
	return c.Status(statusCode).JSON(fiber.Map{"error": message})
}

// recordSuccess helper function to record successful responses with observability
func (p *PartnerHandler) recordSuccess(
	ctx context.Context, span trace.Span, c *fiber.Ctx,
	start time.Time, statusCode int, responseData interface{}, fields ...zap.Field) error {
	// Record request duration
	duration := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds
	p.requestDuration.Record(ctx, duration, metric.WithAttributes(
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

	p.log.Info("Request completed successfully", logFields...)

	// Return HTTP success response
	return c.Status(statusCode).JSON(responseData)
}

func (p *PartnerHandler) CheckLimit(c *fiber.Ctx) error {
	// 1. Observability Setup
	ctx := c.UserContext()
	ctx, span := p.tracer.Start(ctx, "handler.CheckLimit")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
		attribute.String("http.user_agent", string(c.Request().Header.UserAgent())),
		attribute.String("http.client_ip", c.IP()),
	)

	p.log.Debug("Received check limit request",
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("client_ip", c.IP()),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.requestCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
	))

	// 2. Parse Request Body
	var req dto.CheckLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return p.recordError(
			ctx, span, c, start, err,
			fiber.StatusBadRequest, "parse_error", "Cannot parse request body", zap.Error(err))
	}

	// 3. Validate Request
	if err := p.validate.Struct(req); err != nil {
		return p.recordError(
			ctx, span, c, start, err,
			fiber.StatusBadRequest, "validation_error", "Validation failed", zap.Error(err))
	}

	// Add request attributes to span
	span.SetAttributes(
		attribute.String("customer.nik", req.CustomerNIK),
		attribute.Int("tenor.months", int(req.TenorMonths)),
	)

	p.log.Debug("Processing check limit",
		zap.String("nik", req.CustomerNIK),
		zap.Int("tenor_months", int(req.TenorMonths)),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	// 4. Context with Timeout for Service Call
	serviceCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 5. Call Service
	res, err := p.partnerService.CheckLimit(serviceCtx, req)
	if err != nil {
		switch {
		case errors.Is(err, common.ErrCustomerNotFound):
			return p.recordError(
				ctx, span, c, start, err,
				fiber.StatusNotFound, "customer_not_found", "Customer not found", zap.String("nik", req.CustomerNIK))
		case errors.Is(err, common.ErrTenorNotFound):
			return p.recordError(
				ctx, span, c, start, err,
				fiber.StatusNotFound, "tenor_not_found", "Tenor not found", zap.Int("tenor_months", int(req.TenorMonths)))
		case errors.Is(err, common.ErrLimitNotSet):
			return p.recordError(
				ctx, span, c, start, err,
				fiber.StatusNotFound, "limit_not_set", "Limit not set", zap.String("nik", req.CustomerNIK))
		default:
			return p.recordError(
				ctx, span, c, start, err,
				fiber.StatusInternalServerError, "service_error", "Internal server error", zap.Error(err))
		}
	}

	// Add response attributes to span
	span.SetAttributes(
		attribute.String("limit_check.status", res.Status),
	)

	// 6. Send Response based on status
	if res.Status == "rejected" {
		return p.recordSuccess(ctx, span, c, start, fiber.StatusUnprocessableEntity, res,
			zap.String("nik", req.CustomerNIK),
			zap.String("status", res.Status),
		)
	}

	return p.recordSuccess(ctx, span, c, start, fiber.StatusOK, res,
		zap.String("nik", req.CustomerNIK),
		zap.String("status", res.Status),
	)
}

func (h *PartnerHandler) CreateTransaction(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.CreateTransaction")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
		attribute.String("http.user_agent", string(c.Request().Header.UserAgent())),
		attribute.String("http.client_ip", c.IP()),
	)

	h.log.Debug("Received create transaction request",
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("client_ip", c.IP()),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	h.requestCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
	))

	var req dto.CreateTransactionRequest
	if err := c.BodyParser(&req); err != nil {
		return h.recordError(
			ctx, span, c, start, err,
			fiber.StatusBadRequest, "parse_error", "Cannot parse request body", zap.Error(err))
	}

	if err := h.validate.Struct(req); err != nil {
		return h.recordError(
			ctx, span, c, start, err,
			fiber.StatusBadRequest, "validation_error", "Validation failed", zap.Error(err))
	}

	span.SetAttributes(
		attribute.String("customer.nik", req.CustomerNIK),
		attribute.Int("tenor.months", int(req.TenorMonths)),
		attribute.Float64("transaction.amount", req.OTRAmount),
		attribute.String("transaction.asset_name", req.AssetName),
	)

	h.log.Debug("Processing create transaction",
		zap.String("nik", req.CustomerNIK),
		zap.Int("tenor_months", int(req.TenorMonths)),
		zap.Float64("amount", req.OTRAmount),
		zap.String("asset_name", req.AssetName),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	serviceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 5. Panggil service
	createdTx, err := h.partnerService.CreateTransaction(serviceCtx, req)
	if err != nil {
		// Mapping error
		switch {
		case errors.Is(err, common.ErrCustomerNotFound):
			return h.recordError(
				ctx, span, c, start, err,
				fiber.StatusNotFound, "customer_not_found", "Customer not found", zap.String("nik", req.CustomerNIK))
		case errors.Is(err, common.ErrTenorNotFound):
			return h.recordError(
				ctx, span, c, start, err,
				fiber.StatusNotFound, "tenor_not_found", "Tenor not found", zap.Int("tenor_months", int(req.TenorMonths)))
		case errors.Is(err, common.ErrInsufficientLimit):
			return h.recordError(
				ctx, span, c, start, err,
				fiber.StatusUnprocessableEntity, "insufficient_limit", "Insufficient limit", zap.String("nik", req.CustomerNIK), zap.Float64("amount", req.OTRAmount))
		case errors.Is(err, common.ErrLimitNotSet):
			return h.recordError(
				ctx, span, c, start, err,
				fiber.StatusUnprocessableEntity, "limit_not_set", "Limit not set", zap.String("nik", req.CustomerNIK))
		default:
			return h.recordError(
				ctx, span, c, start, err,
				fiber.StatusInternalServerError, "service_error", "An internal server error occurred", zap.Error(err))
		}
	}

	// Add transaction result attributes to span
	if createdTx != nil {
		// Assuming createdTx has ID field or similar
		span.SetAttributes(
			attribute.String("transaction.status", "created"),
		)
	}

	// 6. Kirim response sukses
	return h.recordSuccess(ctx, span, c, start, fiber.StatusCreated, createdTx,
		zap.String("nik", req.CustomerNIK),
		zap.Float64("amount", req.OTRAmount),
		zap.String("asset_name", req.AssetName),
	)
}
