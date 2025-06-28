package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/cloudinary"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ProfileHandler struct {
	profileService    service.ProfileServices
	validate          *validator.Validate
	cloudinaryService service.CloudinaryService
	meter             metric.Meter
	tracer            trace.Tracer
	log               *zap.Logger
	requestCount      metric.Int64Counter
	requestDuration   metric.Float64Histogram
	errorCount        metric.Int64Counter
	responseSize      metric.Int64Histogram
}

func NewProfileHandler(
	profileService service.ProfileServices,
	cloudinaryService service.CloudinaryService,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) *ProfileHandler {
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

	return &ProfileHandler{
		profileService:    profileService,
		validate:          validator.New(validator.WithRequiredStructEnabled()),
		cloudinaryService: cloudinaryService,
		meter:             meter,
		tracer:            tracer,
		log:               log,
		requestCount:      requestCount,
		requestDuration:   requestDuration,
		errorCount:        errorCount,
		responseSize:      responseSize,
	}
}

// recordError helper function to record errors with observability
func (h *ProfileHandler) recordError(
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
func (h *ProfileHandler) recordSuccess(
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

func (h *ProfileHandler) Register(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.CreateProfile")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
		attribute.String("http.client_ip", c.IP()),
	)

	h.log.Debug("Received create profile request",
		zap.String("method", c.Method()),
		zap.String("path", c.Path()),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	h.requestCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("endpoint", c.Path()),
		attribute.String("method", c.Method()),
	))

	var req dto.CreateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Invalid request body")
	}

	ktpFile, err := c.FormFile("ktp_photo")
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "form_file_error", "KTP photo is a required form field")
	}
	req.KtpPhoto = ktpFile

	selfieFile, err := c.FormFile("selfie_photo")
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "form_file_error", "Selfie photo is a required form field")
	}
	req.SelfiePhoto = selfieFile

	if err := h.validate.Struct(&req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "validation_error", err.Error())
	}

	span.SetAttributes(attribute.String("customer.nik", req.NIK))
	serviceCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	resultChan := make(chan cloudinary.UploadResult, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		url, err := h.cloudinaryService.UploadImage(serviceCtx, ktpFile, "multifinance")
		resultChan <- cloudinary.UploadResult{URL: url, Error: err, Type: "ktp"}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		url, err := h.cloudinaryService.UploadImage(serviceCtx, selfieFile, "multifinance")
		resultChan <- cloudinary.UploadResult{URL: url, Error: err, Type: "selfie"}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var ktpUrl, selfieUrl string
	var uploadErrors []string
	for result := range resultChan {
		if result.Error != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s upload failed: %v", result.Type, result.Error))
			continue
		}
		if result.Type == "ktp" {
			ktpUrl = result.URL
		} else {
			selfieUrl = result.URL
		}
	}

	if len(uploadErrors) > 0 {
		err := fmt.Errorf("upload errors: %v", uploadErrors)
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "upload_error", "One or more file uploads failed", zap.Strings("upload_errors", uploadErrors))
	}

	dtoRegister := dto.RegisterToEntity(req, ktpUrl, selfieUrl)
	newCustomer, err := h.profileService.Create(serviceCtx, dtoRegister)
	if err != nil {
		if err.Error() == "nik already registered" || errors.Is(err, gorm.ErrRecordNotFound) {
			return h.recordError(ctx, span, c, start, err, fiber.StatusConflict, "conflict_error", "NIK already registered", zap.String("nik", req.NIK))
		}
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Could not process registration")
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusCreated, newCustomer, zap.String("nik", newCustomer.NIK))
}

func (h *ProfileHandler) GetMyProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.GetMyProfile")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received get my profile request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusUnauthorized, "auth_error", "Unauthorized: Customer ID not found")
	}
	span.SetAttributes(attribute.Int64("customer.id", int64(customerID)))

	customer, err := h.profileService.GetMyProfile(c.Context(), customerID)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to get profile")
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, customer)
}

func (h *ProfileHandler) UpdateMyProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.UpdateMyProfile")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received update my profile request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusUnauthorized, "auth_error", "Unauthorized: Customer ID not found")
	}
	span.SetAttributes(attribute.Int64("customer.id", int64(customerID)))

	var req dto.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "parse_error", "Cannot parse request body")
	}

	if err := h.validate.Struct(req); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusBadRequest, "validation_error", err.Error())
	}

	dtoUpdate := dto.UpdateToEntity(req)
	if err := h.profileService.Update(c.Context(), customerID, dtoUpdate); err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to update profile")
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, fiber.Map{"message": "Profile updated successfully"})
}

func (h *ProfileHandler) GetMyLimits(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.GetMyLimits")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received get my limits request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusUnauthorized, "auth_error", "Unauthorized: Customer ID not found")
	}
	span.SetAttributes(attribute.Int64("customer.id", int64(customerID)))

	limits, err := h.profileService.GetMyLimits(c.Context(), customerID)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to get limits")
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, limits)
}

func (h *ProfileHandler) GetMyTransactions(c *fiber.Ctx) error {
	ctx := c.UserContext()
	ctx, span := h.tracer.Start(ctx, "handler.GetMyTransactions")
	defer span.End()
	start := time.Now()

	span.SetAttributes(
		attribute.String("http.method", c.Method()),
		attribute.String("http.route", c.Path()),
	)
	h.log.Debug("Received get my transactions request", zap.String("path", c.Path()))
	h.requestCount.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", c.Path()), attribute.String("method", c.Method())))

	customerID, err := getCustomerIDFromLocals(c)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusUnauthorized, "auth_error", "Unauthorized: Customer ID not found")
	}

	params := domain.Params{
		Status: c.Query("status"),
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
	}

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("query.status", params.Status),
		attribute.Int("query.page", params.Page),
		attribute.Int("query.limit", params.Limit),
	)

	response, err := h.profileService.GetMyTransactions(c.Context(), customerID, params)
	if err != nil {
		return h.recordError(ctx, span, c, start, err, fiber.StatusInternalServerError, "service_error", "Failed to get transactions")
	}

	return h.recordSuccess(ctx, span, c, start, fiber.StatusOK, response)
}

// getCustomerIDFromLocals adalah helper untuk mengambil ID customer dari context Fiber
func getCustomerIDFromLocals(c *fiber.Ctx) (uint64, error) {
	idVal := c.Locals("customerID")
	id, ok := idVal.(uint64)
	if !ok {
		return 0, errors.New("customerID not found or invalid in context")
	}
	return id, nil
}
