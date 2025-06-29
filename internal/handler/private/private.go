package private_handler

import (
	"errors"

	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/service"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type PrivateHandler struct {
	privateService service.PrivateService
	validate       *validator.Validate

	meter           metric.Meter
	tracer          trace.Tracer
	log             *zap.Logger
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorCount      metric.Int64Counter
	responseSize    metric.Int64Histogram
}

func (h *PrivateHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := h.privateService.Login(c.Context(), req)
	if err != nil {
		if errors.Is(err, common.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(res)
}

func NewPrivateHandler(
	privateService service.PrivateService,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) *PrivateHandler {
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

	return &PrivateHandler{
		privateService:  privateService,
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
