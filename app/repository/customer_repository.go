package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type customerRepository struct {
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

// FindByNIKWithLock implements CustomerRepository.
func (c *customerRepository) FindByNIKWithLock(ctx context.Context, nik string) (*domain.Customer, error) {
	ctx, span := c.tracer.Start(ctx, "repository.FindByNIKWithLock")
	defer span.End()

	start := time.Now()

	c.log.Debug("Find customer by NIK with lock",
		zap.String("nik", nik),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	c.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_nik_with_lock"),
			attribute.String("table", "customers"),
		),
	)
	defer c.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_nik_with_lock"),
			attribute.String("table", "customers"),
		),
	)

	c.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select_for_update"),
			attribute.String("table", "customers"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select_for_update"),
		attribute.String("db.table", "customers"),
		attribute.String("customer.nik", nik),
		attribute.String("trace_id", span.SpanContext().TraceID().String()),
	)

	var customer model.Customer

	// Menggunakan Clauses(clause.Locking{Strength: "UPDATE"}) untuk SELECT ... FOR UPDATE
	err := c.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("nik = ?", nik).First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Ok, "Customer not found")

			c.log.Info("Customer not found by NIK",
				zap.String("nik", nik),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			duration := float64(time.Since(start).Milliseconds())
			c.queryDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "select_for_update"),
					attribute.String("table", "customers"),
					attribute.String("status", "not_found"),
				),
			)

			return nil, nil
		}

		span.SetStatus(codes.Error, "Error finding customer by NIK with lock")
		span.RecordError(err)

		c.log.Error("Error finding customer by NIK with lock",
			zap.String("nik", nik),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select_for_update"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_for_update"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	c.documentsRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "customers"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	c.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select_for_update"),
			attribute.String("table", "customers"),
			attribute.String("status", "success"),
		),
	)

	c.log.Info("Customer found by NIK with lock",
		zap.String("nik", nik),
		zap.Uint64("customer_id", customer.ID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer found by NIK with lock")
	span.SetAttributes(
		attribute.String("customer.id", fmt.Sprintf("%d", customer.ID)),
	)

	return model.CustomerToEntity(customer), nil
}

// FindByID implements CustomerRepository.
func (c *customerRepository) FindByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	ctx, span := c.tracer.Start(ctx, "repository.FindByID")
	defer span.End()

	start := time.Now()

	c.log.Debug("Find customer by ID",
		zap.Uint64("id", id),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	c.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_id"),
			attribute.String("table", "customers"),
		),
	)
	defer c.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_id"),
			attribute.String("table", "customers"),
		),
	)

	c.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customers"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "customers"),
		attribute.String("customer.id", fmt.Sprintf("%d", id)),
		attribute.String("trace_id", span.SpanContext().TraceID().String()),
	)

	var customer model.Customer
	if err := c.db.WithContext(ctx).First(&customer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Ok, "Customer not found")

			c.log.Info("Customer not found by ID",
				zap.Uint64("id", id),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			duration := float64(time.Since(start).Milliseconds())
			c.queryDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "select"),
					attribute.String("table", "customers"),
					attribute.String("status", "not_found"),
				),
			)

			return nil, nil
		}

		span.SetStatus(codes.Error, "Error finding customer by ID")
		span.RecordError(err)

		c.log.Error("Error finding customer by ID",
			zap.Uint64("id", id),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	c.documentsRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "customers"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	c.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customers"),
			attribute.String("status", "success"),
		),
	)

	c.log.Info("Customer found by ID",
		zap.Uint64("id", id),
		zap.String("nik", customer.NIK),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer found by ID")
	span.SetAttributes(
		attribute.String("customer.nik", customer.NIK),
	)

	return model.CustomerToEntity(customer), nil
}

// FindByNIK implements CustomerRepository.
func (c *customerRepository) FindByNIK(ctx context.Context, nik string) (*domain.Customer, error) {
	ctx, span := c.tracer.Start(ctx, "repository.FindByNIK")
	defer span.End()

	start := time.Now()

	c.log.Debug("Find customer by NIK",
		zap.String("nik", nik),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	c.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_nik"),
			attribute.String("table", "customers"),
		),
	)
	defer c.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_by_nik"),
			attribute.String("table", "customers"),
		),
	)

	c.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customers"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select"),
		attribute.String("db.table", "customers"),
		attribute.String("customer.nik", nik),
		attribute.String("trace_id", span.SpanContext().TraceID().String()),
	)

	var customer model.Customer

	if err := c.db.WithContext(ctx).Where("nik = ?", nik).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Ok, "Customer not found")

			c.log.Info("Customer not found by NIK",
				zap.String("nik", nik),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			duration := float64(time.Since(start).Milliseconds())
			c.queryDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "select"),
					attribute.String("table", "customers"),
					attribute.String("status", "not_found"),
				),
			)

			return nil, nil
		}

		span.SetStatus(codes.Error, "Error finding customer by NIK")
		span.RecordError(err)

		c.log.Error("Error finding customer by NIK",
			zap.String("nik", nik),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	c.documentsRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "customers"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	c.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select"),
			attribute.String("table", "customers"),
			attribute.String("status", "success"),
		),
	)

	c.log.Info("Customer found by NIK",
		zap.String("nik", nik),
		zap.Uint64("customer_id", customer.ID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer found by NIK")
	span.SetAttributes(
		attribute.String("customer.id", fmt.Sprintf("%d", customer.ID)),
	)

	return model.CustomerToEntity(customer), nil
}

// FindPaginated implements CustomerRepository.
func (c *customerRepository) FindPaginated(ctx context.Context, params domain.Params) ([]domain.Customer, int64, error) {
	ctx, span := c.tracer.Start(ctx, "repository.FindPaginated")
	defer span.End()

	start := time.Now()

	c.log.Debug("Find customers paginated",
		zap.Int("page", params.Page),
		zap.Int("limit", params.Limit),
		zap.String("status", params.Status),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	c.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_paginated"),
			attribute.String("table", "customers"),
		),
	)
	defer c.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_paginated"),
			attribute.String("table", "customers"),
		),
	)

	c.queryCount.Add(ctx, 2, // Count query + Select query
		metric.WithAttributes(
			attribute.String("operation", "select_paginated"),
			attribute.String("table", "customers"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select_paginated"),
		attribute.String("db.table", "customers"),
		attribute.Int("pagination.page", params.Page),
		attribute.Int("pagination.limit", params.Limit),
		attribute.String("filter.status", params.Status),
		attribute.String("trace_id", span.SpanContext().TraceID().String()),
	)

	var customers []model.Customer
	var total int64

	query := c.db.WithContext(ctx).Model(&model.Customer{})
	countQuery := c.db.WithContext(ctx).Model(&model.Customer{})

	// Filter berdasarkan status
	if params.Status != "" {
		query = query.Where("verification_status = ?", params.Status)
		countQuery = countQuery.Where("verification_status = ?", params.Status)
	}

	// Hitung total sebelum paginasi
	if err := countQuery.Count(&total).Error; err != nil {
		span.SetStatus(codes.Error, "Error counting customers")
		span.RecordError(err)

		c.log.Error("Error counting customers",
			zap.String("status", params.Status),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "count"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return nil, 0, err
	}

	// Terapkan paginasi
	offset := (params.Page - 1) * params.Limit
	query = query.Limit(params.Limit).Offset(offset).Order("created_at DESC")

	if err := query.Find(&customers).Error; err != nil {
		span.SetStatus(codes.Error, "Error finding customers paginated")
		span.RecordError(err)

		c.log.Error("Error finding customers paginated",
			zap.Int("page", params.Page),
			zap.Int("limit", params.Limit),
			zap.String("status", params.Status),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return nil, 0, err
	}

	c.documentsRetrieved.Add(ctx, int64(len(customers)),
		metric.WithAttributes(
			attribute.String("table", "customers"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	c.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select_paginated"),
			attribute.String("table", "customers"),
			attribute.String("status", "success"),
		),
	)

	c.log.Info("Customers found paginated",
		zap.Int("page", params.Page),
		zap.Int("limit", params.Limit),
		zap.String("status", params.Status),
		zap.Int64("total", total),
		zap.Int("retrieved", len(customers)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customers found paginated")
	span.SetAttributes(
		attribute.Int64("result.total", total),
		attribute.Int("result.retrieved", len(customers)),
	)

	return model.CustomersToEntity(customers), total, nil
}

// CreateCustomer implements CustomerRepository.
func (c *customerRepository) CreateCustomer(ctx context.Context, customer *domain.Customer) error {
	ctx, span := c.tracer.Start(ctx, "repository.CreateCustomer")
	defer span.End()

	start := time.Now()

	c.log.Debug("Create customer",
		zap.String("nik", customer.NIK),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	c.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "create_customer"),
			attribute.String("table", "customers"),
		),
	)
	defer c.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "create_customer"),
			attribute.String("table", "customers"),
		),
	)

	c.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "insert"),
			attribute.String("table", "customers"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "insert"),
		attribute.String("db.table", "customers"),
		attribute.String("customer.nik", customer.NIK),
		attribute.String("trace_id", span.SpanContext().TraceID().String()),
	)

	data := model.CustomerFromEntity(customer)
	if err := c.db.WithContext(ctx).Create(&data).Error; err != nil {
		span.SetStatus(codes.Error, "Error creating customer")
		span.RecordError(err)

		c.log.Error("Error creating customer",
			zap.String("nik", customer.NIK),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		c.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "insert"),
				attribute.String("table", "customers"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		c.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "insert"),
				attribute.String("table", "customers"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	c.documentsInserted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "customers"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	c.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "insert"),
			attribute.String("table", "customers"),
			attribute.String("status", "success"),
		),
	)

	c.log.Info("Customer created successfully",
		zap.String("nik", customer.NIK),
		zap.Uint64("customer_id", data.ID),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer created successfully")
	span.SetAttributes(
		attribute.String("customer.id", fmt.Sprintf("%d", data.ID)),
	)

	return nil
}

func NewCustomerRepository(
	db *gorm.DB,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) CustomerRepository {
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

	return &customerRepository{
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
