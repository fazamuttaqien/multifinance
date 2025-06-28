package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fazamuttaqien/multifinance/config"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OpenTelemetry struct {
	Log            *zap.Logger
	TracerProvider *sdktrace.TracerProvider
	LoggerProvider *sdklog.LoggerProvider
	MeterProvider  *sdkmetric.MeterProvider
	Meter          metric.Meter
	Shutdown       func(context.Context) error
}

// New menginisialisasi semua komponen telemetri
func New(ctx context.Context, cfg *config.Config) (*OpenTelemetry, error) {
	res, err := NewResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Buat koneksi gRPC untuk semua exporter
	// !! WARNING: Using insecure connection for example !!
	// !!          Configure TLS and auth for production !!
	conn, err := NewOTLPClient(cfg.OTEL_EXPORTER_OTLP_ENDPOINT)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP client: %w", err)
	}

	// Setup Providers
	tracerProvider, err := NewTracerProvider(ctx, conn, res)
	if err != nil {
		conn.Close() // Cleanup
		return nil, fmt.Errorf("failed to create tracer provider: %w", err)
	}
	otel.SetTracerProvider(tracerProvider) // Set global

	loggerProvider, err := NewLoggerProvider(ctx, conn, res)
	if err != nil {
		conn.Close()
		tracerProvider.Shutdown(context.Background()) // Cleanup
		return nil, fmt.Errorf("failed to create logger provider: %w", err)
	}
	// No global logger provider, use via otelzap

	meterProvider, err := NewMeterProvider(ctx, conn, res, cfg)
	if err != nil {
		conn.Close()
		tracerProvider.Shutdown(context.Background())
		loggerProvider.Shutdown(context.Background())
		return nil, fmt.Errorf("failed to create meter provider: %w", err)
	}
	otel.SetMeterProvider(meterProvider) // Set global

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Buat Zap logger yang terintegrasi
	log := NewZapLogger(cfg, loggerProvider)

	// Daftarkan logger yang dibuat oleh New sebagai global
	zap.ReplaceGlobals(log)

	// Anda juga bisa mendaftarkan SugaredLogger jika sering menggunakannya
	// zap.ReplaceGlobals(log.Sugar())

	// Aktifkan runtime metrics jika dikonfigurasi
	var runtimeErr error
	if cfg.RUNTIME_METRICS {
		zap.L().Info("Starting runtime metrics collection")
		runtimeErr = runtime.Start(runtime.WithMeterProvider(meterProvider),
			runtime.WithMinimumReadMemStatsInterval(time.Second))
		if runtimeErr != nil {
			zap.L().Warn("Failed to start runtime metrics collector", zap.Error(runtimeErr))
			// Non-fatal, continue startup
		}
	}

	// Buat meter untuk aplikasi
	appMeter := meterProvider.Meter(cfg.SERVICE_NAME)

	// Fungsi untuk shutdown semua komponen telemetri
	shutdown := func(ctx context.Context) error {
		zap.L().Info("Shutting down telemetry components...")
		var firstErr error

		// 1. Flush logger
		if err := zap.L().Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing zap logger: %v\n", err)
			firstErr = fmt.Errorf("zap sync failed: %w", err)
		}

		// 2. Shutdown providers (reverse order or based on dependency)
		if err := meterProvider.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down meter provider: %v\n", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("meter shutdown failed: %w", err)
			}
		}
		// Logger provider shutdown is important for otelzap bridge
		if err := loggerProvider.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down logger provider: %v\n", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("logger shutdown failed: %w", err)
			}
		}
		if err := tracerProvider.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error shutting down tracer provider: %v\n", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("tracer shutdown failed: %w", err)
			}
		}

		// 3. Close connection (after providers using it are shutdown)
		if err := conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing OTLP gRPC connection: %v\n", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("grpc conn close failed: %w", err)
			}
		}

		zap.L().Info("Telemetry shutdown complete.")
		return firstErr
	}

	zap.L().Info("Telemetry initialized successfully", zap.String("otel_endpoint", cfg.OTEL_EXPORTER_OTLP_ENDPOINT))

	return &OpenTelemetry{
		Log:            log,
		TracerProvider: tracerProvider,
		LoggerProvider: loggerProvider,
		MeterProvider:  meterProvider,
		Meter:          appMeter,
		Shutdown:       shutdown,
	}, nil
}

func NewResource(cfg *config.Config) (*sdkresource.Resource, error) {
	hostName, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s-%d", hostName, time.Now().UnixNano())

	return sdkresource.New(
		context.Background(),
		sdkresource.WithProcess(),
		sdkresource.WithOS(),
		sdkresource.WithContainer(),
		sdkresource.WithHost(),
		sdkresource.WithFromEnv(),
		sdkresource.WithAttributes(
			semconv.ServiceVersionKey.String(cfg.SERVICE_VERSION),
			semconv.ServiceInstanceIDKey.String(instanceID),
		),
		// sdkresource.WithSchemaURL(semconv.SchemaURL), // Penting untuk kesesuaian semantic versioning
	)
}

// NewTracerProvider
func NewTracerProvider(ctx context.Context, conn *grpc.ClientConn, res *sdkresource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	// Use BatchSpanProcessor for production
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)

	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))), // 10% sampling rate
	)
	return tracerProvider, nil
}

// NewLoggerProvider
func NewLoggerProvider(ctx context.Context, conn *grpc.ClientConn, res *sdkresource.Resource) (*sdklog.LoggerProvider, error) {
	logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	// Batch Log Processor
	blp := sdklog.NewBatchProcessor(logExporter)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(blp),
	)
	return loggerProvider, nil
}

// NewMeterProvider 
func NewMeterProvider(ctx context.Context, conn *grpc.ClientConn, res *sdkresource.Resource, cfg *config.Config) (*sdkmetric.MeterProvider, error) {
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(cfg.METRIC_INTERVAL),
			),
		),
	)
	return meterProvider, nil
}

// NewOTLPClient
func NewOTLPClient(endpoint string) (*grpc.ClientConn, error) {
	// !! PRODUCTION: Use secure credentials (TLS) and potentially auth !!
	// secureCreds := credentials.NewTLS(&tls.Config{}) // Basic TLS
	// opts := []grpc.DialOption{grpc.WithTransportCredentials(secureCreds)}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: 5 * time.Second}),
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to %s: %w", endpoint, err)
	}
	return conn, nil
}

// Ditambahkan field context langsung dari provider
func NewZapLogger(cfg *config.Config, loggerProvider *sdklog.LoggerProvider) *zap.Logger {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.LOG_LEVEL)); err != nil {
		level = zapcore.InfoLevel // fallback
	}

	var encoderConfig zapcore.EncoderConfig
	if cfg.DEV_MODE {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		// Customize for production JSON logs if needed
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.LevelKey = "level"
		encoderConfig.NameKey = "logger"
		encoderConfig.CallerKey = "caller"
		encoderConfig.MessageKey = "message"
		encoderConfig.StacktraceKey = "stacktrace"
		encoderConfig.FunctionKey = zapcore.OmitKey
		encoderConfig.LineEnding = zapcore.DefaultLineEnding
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	var encoder zapcore.Encoder
	if cfg.DEV_MODE {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Core 1: Output ke stdout
	stdoutSink := zapcore.AddSync(os.Stdout)
	stdoutCore := zapcore.NewCore(encoder, stdoutSink, level)

	// Core 2: Kirim log via OpenTelemetry LoggerProvider
	otelCore := otelzap.NewCore(
		cfg.SERVICE_NAME, // Atribut nama layanan bisa ditambahkan otomatis oleh resource provider juga
		otelzap.WithLoggerProvider(loggerProvider), // Mengarahkan log ke pipeline OTel
	)

	// Gabungkan kedua core: log akan ke stdout DAN dikirim via OTLP
	core := zapcore.NewTee(stdoutCore, otelCore)

	// Tambahkan Caller dan Stacktrace
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel), // Stacktrace hanya untuk error level
		zap.Fields( // Tambahkan field default ke semua log
			zap.String("service.name", cfg.SERVICE_NAME),
			zap.String("service.version", cfg.SERVICE_VERSION),
			zap.String("deployment.environment", cfg.ENVIRONMENT),
		),
	}
	// Optimasi: Hanya lakukan caller skip jika perlu (misal dari wrapper custom)
	// opts = append(opts, zap.AddCallerSkip(1))

	zapLogger := zap.New(core, opts...)

	return zapLogger
}
