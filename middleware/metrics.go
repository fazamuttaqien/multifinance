package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	
	"go.uber.org/zap"
)

// OtelMiddleware is a middleware for OpenTelemetry and logging in Fiber
type OtelMiddleware struct {
	meter                     metric.Meter
	tracer                    trace.Tracer
	httpRequestCounter        metric.Int64Counter
	httpRequestDuration       metric.Float64Histogram
	httpResponseStatusCounter metric.Int64Counter
	httpRequestSize           metric.Int64Histogram
	httpResponseSize          metric.Int64Histogram
	httpActiveRequests        metric.Int64UpDownCounter
	propagator                propagation.TextMapPropagator
}

func NewOtelMiddleware() *OtelMiddleware {
	meter := otel.GetMeterProvider().Meter("fiber-middleware")
	tracer := otel.GetTracerProvider().Tracer("fiber-middleware")

	// Counter for request
	httpRequestCounter, _ := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP request"),
		metric.WithUnit("{request}"),
	)

	// Histogram for request duration
	httpRequestDuration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("ms"),
	)

	// Counter for status
	httpResponseStatusCounter, _ := meter.Int64Counter(
		"http.server.response.status",
		metric.WithDescription("HTTP response status codes"),
		metric.WithUnit("{status}"),
	)

	// Histogram for request size
	httpRequestSize, _ := meter.Int64Histogram(
		"http.server.request.size",
		metric.WithDescription("Size of HTTP requests"),
		metric.WithUnit("bytes"),
	)

	// Histogram for response size
	httpResponseSize, _ := meter.Int64Histogram(
		"http.server.response.size",
		metric.WithDescription("Size of HTTP responses"),
		metric.WithUnit("bytes"),
	)

	// Gauge for active requests
	httpActiveRequests, _ := meter.Int64UpDownCounter(
		"http.server.active.requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)

	// Propagator to forward context tracing
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		propagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}

	return &OtelMiddleware{
		meter:                     meter,
		tracer:                    tracer,
		httpRequestCounter:        httpRequestCounter,
		httpRequestDuration:       httpRequestDuration,
		httpResponseStatusCounter: httpResponseStatusCounter,
		httpRequestSize:           httpRequestSize,
		httpResponseSize:          httpResponseSize,
		httpActiveRequests:        httpActiveRequests,
		propagator:                propagator,
	}
}

// Handle return handler middleware
func (m *OtelMiddleware) Handle() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract the SpanContext and other values from the request headers
		// and create a Context.
		ctx := m.propagator.Extract(c.Context(), propagation.HeaderCarrier(c.GetReqHeaders()))

		path := c.Path()
		method := c.Method()

		// Start a new span with the extracted context
		spanName := fmt.Sprintf("%s %s", method, path)
		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.target", path),
				attribute.String("http.scheme", "http"),
				attribute.String("http.host", c.Hostname()),
				attribute.String("http.user_agent", string(c.Request().Header.UserAgent())),
				attribute.String("http.client_ip", c.IP()),
			),
		)
		defer span.End()

		// Store the span context in Fiber's context
		c.Locals("otel-context", ctx)

		// Get request size
		reqContentLength := int64(c.Request().Header.ContentLength())

		// Record request size
		m.httpRequestSize.Record(ctx, reqContentLength,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
			),
		)

		// Increment active request counter
		m.httpActiveRequests.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
			),
		)

		// Log request start
		zap.L().Info("HTTP request started",
			zap.String("method", method),
			zap.String("path", path),
			zap.String("client_ip", c.IP()),
			zap.String("user_agent", string(c.Request().Header.UserAgent())),
			zap.Int("content_length", int(reqContentLength)),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)

		startTime := time.Now()

		// Record request count
		m.httpRequestCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
			),
		)

		// Process the request
		err := c.Next()

		// Calculate request duration
		duration := float64(time.Since(startTime).Milliseconds())

		// Get response status
		status := c.Response().StatusCode()

		// Record metrics based on response
		m.httpRequestDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
				attribute.Int("http.status_code", status),
			),
		)

		// Record response status counter
		m.httpResponseStatusCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
				attribute.Int("http.status_code", status),
			),
		)

		// Record response size
		resContentLength := int64(len(c.Response().Body()))
		m.httpRequestSize.Record(ctx, resContentLength,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
				attribute.Int("http.status_code", status),
			),
		)

		// Decrement active requests counter
		m.httpActiveRequests.Add(ctx, -1,
			metric.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.route", path),
			),
		)

		// Set span status based on response
		if status >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
			// Add error details to span
			span.SetAttributes(attribute.String("error.message", fmt.Sprintf("HTTP Error %d", status)))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Log request completion
		zap.L().Info("HTTP request completed",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Float64("duration_ms", duration),
			zap.Int64("response_size", resContentLength),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)

		return err
	}
}
