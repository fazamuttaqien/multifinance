package repository

import (
	"context"
	"errors"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type tenorRepository struct {
	db                 *gorm.DB
	meter              metric.Meter
	tracer             trace.Tracer
	log                *zap.Logger
	queryDuration      metric.Float64Histogram
	queryCount         metric.Int64Counter
	errorCount         metric.Int64Counter
	connectionGauge    metric.Int64UpDownCounter
	documentsRetrieved metric.Int64Counter
}

// FindAll implements TenorRepository.
func (t *tenorRepository) FindAll(ctx context.Context) ([]domain.Tenor, error) {
	ctx, span := t.tracer.Start(ctx, "repository.FindAllTenors")
	defer span.End()

	start := time.Now()

	t.log.Debug("Find all tenors",
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	t.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_all_tenors"),
			attribute.String("table", "tenors"),
		),
	)
	defer t.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_all_tenors"),
			attribute.String("table", "tenors"),
		),
	)

	t.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "tenors"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "tenors"),
	)

	var tenors []model.Tenor
	err := t.db.WithContext(ctx).Find(&tenors).Error

	if err != nil {
		span.SetStatus(codes.Error, "Error finding all tenors")
		span.RecordError(err)

		t.log.Error("Error finding all tenors",
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "tenors"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "tenors"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	t.documentsRetrieved.Add(ctx, int64(len(tenors)),
		metric.WithAttributes(
			attribute.String("table", "tenors"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	t.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "tenors"),
			attribute.String("status", "success"),
		),
	)

	t.log.Info("All tenors retrieved",
		zap.Int("retrieved_count", len(tenors)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Tenors found successfully")
	span.SetAttributes(attribute.Int("result.retrieved", len(tenors)))

	return model.TenorsToEntity(tenors), nil
}

// FindByDuration implements TenorRepository.
func (t *tenorRepository) FindByDuration(ctx context.Context, durationMonths uint8) (*domain.Tenor, error) {
	ctx, span := t.tracer.Start(ctx, "repository.FindByDuration")
	defer span.End()

	start := time.Now()

	t.log.Debug("Find tenor by duration",
		zap.Uint8("duration_months", durationMonths),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	t.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_duration"),
			attribute.String("table", "tenors"),
		),
	)
	defer t.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_duration"),
			attribute.String("table", "tenors"),
		),
	)

	t.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "tenors"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "tenors"),
		attribute.Int("tenor.duration_months", int(durationMonths)),
	)

	var tenor model.Tenor
	if err := t.db.WithContext(ctx).Where("duration_months = ?", durationMonths).First(&tenor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Ok, "Tenor not found")

			t.log.Info("Tenor not found by duration",
				zap.Uint8("duration_months", durationMonths),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			duration := float64(time.Since(start).Milliseconds())
			t.queryDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "select"),
					attribute.String("table", "tenors"),
					attribute.String("status", "not_found"),
				),
			)

			return nil, nil
		}

		span.SetStatus(codes.Error, "Error finding tenor by duration")
		span.RecordError(err)

		t.log.Error("Error finding tenor by duration",
			zap.Uint8("duration_months", durationMonths),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "tenors"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "tenors"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	t.documentsRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "tenors"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	t.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "tenors"),
			attribute.String("status", "success"),
		),
	)

	t.log.Info("Tenor found by duration",
		zap.Uint8("duration_months", durationMonths),
		zap.Uint("tenor_id", tenor.ID),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Tenor found successfully")
	span.SetAttributes(
		attribute.Int("tenor.id", int(tenor.ID)),
	)

	return model.TenorToEntity(tenor), nil
}

func NewTenorRepository(
	db *gorm.DB,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) TenorRepository {
	queryDuration, _ := meter.Float64Histogram(
		"db.query.duration",
		metric.WithDescription("Duration of database queries"),
		metric.WithUnit("ms"),
	)

	queryCount, _ := meter.Int64Counter(
		"db.query.count",
		metric.WithDescription("Number of database queries"),
		metric.WithUnit("{query}"),
	)

	errorCount, _ := meter.Int64Counter(
		"db.error.count",
		metric.WithDescription("Number of database errors"),
		metric.WithUnit("{error}"),
	)

	connectionGauge, _ := meter.Int64UpDownCounter(
		"db.connections",
		metric.WithDescription("Number of active database connections"),
		metric.WithUnit("{connection}"),
	)

	documentsRetrieved, _ := meter.Int64Counter(
		"db.documents.retrieved",
		metric.WithDescription("Number of documents retrieved from the database"),
		metric.WithUnit("{document}"),
	)

	return &tenorRepository{
		db:                 db,
		meter:              meter,
		tracer:             tracer,
		log:                log,
		queryDuration:      queryDuration,
		queryCount:         queryCount,
		errorCount:         errorCount,
		connectionGauge:    connectionGauge,
		documentsRetrieved: documentsRetrieved,
	}
}
