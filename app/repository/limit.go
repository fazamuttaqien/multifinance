package repository

import (
	"context"
	"errors"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type limitRepository struct {
	db                 *gorm.DB
	meter              metric.Meter
	tracer             trace.Tracer
	log                *zap.Logger
	queryDuration      metric.Float64Histogram
	queryCount         metric.Int64Counter
	errorCount         metric.Int64Counter
	connectionGauge    metric.Int64UpDownCounter
	documentsInserted  metric.Int64Counter
	documentsRetrieved metric.Int64Counter
}

// FindAllByCustomerID implements LimitRepository.
func (l *limitRepository) FindAllByCustomerID(ctx context.Context, customerID uint64) ([]domain.CustomerLimit, error) {
	ctx, span := l.tracer.Start(ctx, "repository.FindAllByCustomerID")
	defer span.End()

	start := time.Now()

	l.log.Debug("Find all limits by customer ID",
		zap.Uint64("customer_id", customerID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	l.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_all_by_customer_id"),
			attribute.String("table", "customer_limits"),
		),
	)
	defer l.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_all_by_customer_id"),
			attribute.String("table", "customer_limits"),
		),
	)

	l.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customer_limits"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "customer_limits"),
		attribute.Int64("customer.id", int64(customerID)),
	)

	var limits []model.CustomerLimit
	err := l.db.WithContext(ctx).Where("customer_id = ?", customerID).Find(&limits).Error
	if err != nil {
		span.SetStatus(codes.Error, "Error finding limits by customer ID")
		span.RecordError(err)

		l.log.Error("Error finding limits by customer ID",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		l.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customer_limits"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		l.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customer_limits"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	l.documentsRetrieved.Add(ctx, int64(len(limits)),
		metric.WithAttributes(
			attribute.String("table", "customer_limits"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	l.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customer_limits"),
			attribute.String("status", "success"),
		),
	)

	l.log.Info("Limits found by customer ID",
		zap.Uint64("customer_id", customerID),
		zap.Int("retrieved_count", len(limits)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Limits found successfully")
	span.SetAttributes(attribute.Int("result.retrieved", len(limits)))

	return model.LimitsToEntity(limits), nil
}

// UpsertMany implements LimitRepository.
func (l *limitRepository) UpsertMany(ctx context.Context, limits []domain.CustomerLimit) error {
	ctx, span := l.tracer.Start(ctx, "repository.UpsertMany")
	defer span.End()

	if len(limits) == 0 {
		span.SetStatus(codes.Ok, "No limits to upsert")
		return nil
	}

	start := time.Now()

	l.log.Debug("Upserting many limits",
		zap.Int("count", len(limits)),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	l.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "upsert_many"),
			attribute.String("table", "customer_limits"),
		),
	)
	defer l.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "upsert_many"),
			attribute.String("table", "customer_limits"),
		),
	)

	l.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "upsert"),
			attribute.String("table", "customer_limits"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "upsert"),
		attribute.String("db.table", "customer_limits"),
		attribute.Int("limits.count", len(limits)),
	)

	// Menggunakan OnConflict untuk melakukan UPSERT
	// Jika terdapat konflik pada composite primary key (customer_id, tenor_id),
	// perbarui kolom 'limit_amount'
	err := l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "customer_id"}, {Name: "tenor_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"limit_amount"}),
	}).Create(&limits).Error

	if err != nil {
		span.SetStatus(codes.Error, "Error upserting limits")
		span.RecordError(err)

		l.log.Error("Error upserting limits",
			zap.Int("count", len(limits)),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		l.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "upsert"),
				attribute.String("table", "customer_limits"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		l.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "upsert"),
				attribute.String("table", "customer_limits"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	l.documentsInserted.Add(ctx, int64(len(limits)),
		metric.WithAttributes(
			attribute.String("table", "customer_limits"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	l.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "upsert"),
			attribute.String("table", "customer_limits"),
			attribute.String("status", "success"),
		),
	)

	l.log.Info("Limits upserted successfully",
		zap.Int("upserted_count", len(limits)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Limits upserted successfully")
	span.SetAttributes(attribute.Int("result.upserted", len(limits)))

	return nil
}

// FindByCustomerIDAndTenorID implements LimitRepository.
func (l *limitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*domain.CustomerLimit, error) {
	ctx, span := l.tracer.Start(ctx, "repository.FindByCustomerIDAndTenorID")
	defer span.End()

	start := time.Now()

	l.log.Debug("Find limit by customer and tenor ID",
		zap.Uint64("customer_id", customerID),
		zap.Uint("tenor_id", tenorID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	l.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_customer_tenor"),
			attribute.String("table", "customer_limits"),
		),
	)
	defer l.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_customer_tenor"),
			attribute.String("table", "customer_limits"),
		),
	)

	l.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customer_limits"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "customer_limits"),
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("tenor.id", int(tenorID)),
	)

	var limit model.CustomerLimit
	if err := l.db.WithContext(ctx).Where("customer_id = ? AND tenor_id = ?", customerID, tenorID).First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Ok, "Limit not found")

			l.log.Info("Limit not found by customer and tenor ID",
				zap.Uint64("customer_id", customerID),
				zap.Uint("tenor_id", tenorID),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			duration := float64(time.Since(start).Milliseconds())
			l.queryDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "select"),
					attribute.String("table", "customer_limits"),
					attribute.String("status", "not_found"),
				),
			)

			return nil, nil
		}

		span.SetStatus(codes.Error, "Error finding limit by customer and tenor ID")
		span.RecordError(err)

		l.log.Error("Error finding limit by customer and tenor ID",
			zap.Uint64("customer_id", customerID),
			zap.Uint("tenor_id", tenorID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		l.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customer_limits"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		l.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customer_limits"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	l.documentsRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "customer_limits"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	l.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customer_limits"),
			attribute.String("status", "success"),
		),
	)

	l.log.Info("Limit found by customer and tenor ID",
		zap.Uint64("customer_id", customerID),
		zap.Uint("tenor_id", tenorID),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Limit found successfully")

	return model.LimitToEntity(limit), nil
}

func NewLimitRepository(
	db *gorm.DB,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) LimitRepository {
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

	documentsInserted, _ := meter.Int64Counter(
		"db.documents.inserted",
		metric.WithDescription("Number of documents inserted into the database"),
		metric.WithUnit("{document}"),
	)

	documentsRetrieved, _ := meter.Int64Counter(
		"db.documents.retrieved",
		metric.WithDescription("Number of documents retrieved from the database"),
		metric.WithUnit("{document}"),
	)

	return &limitRepository{
		db:                 db,
		meter:              meter,
		tracer:             tracer,
		log:                log,
		queryDuration:      queryDuration,
		queryCount:         queryCount,
		errorCount:         errorCount,
		connectionGauge:    connectionGauge,
		documentsInserted:  documentsInserted,
		documentsRetrieved: documentsRetrieved,
	}
}
