package repository

import (
	"context"
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

type transactionRepository struct {
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

// FindPaginatedByCustomerID implements TransactionRepository.
func (t *transactionRepository) FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params domain.Params) ([]domain.Transaction, int64, error) {
	ctx, span := t.tracer.Start(ctx, "repository.FindPaginatedByCustomerID")
	defer span.End()

	start := time.Now()

	t.log.Debug("Find transactions paginated by customer ID",
		zap.Uint64("customer_id", customerID),
		zap.Int("page", params.Page),
		zap.Int("limit", params.Limit),
		zap.String("status", params.Status),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	t.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "find_paginated_by_customer_id"),
			attribute.String("table", "transactions"),
		),
	)
	defer t.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "find_paginated_by_customer_id"),
			attribute.String("table", "transactions"),
		),
	)

	t.queryCount.Add(ctx, 2, // Count query + Select query
		metric.WithAttributes(
			attribute.String("operation", "select_paginated"),
			attribute.String("table", "transactions"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select_paginated"),
		attribute.String("db.table", "transactions"),
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("pagination.page", params.Page),
		attribute.Int("pagination.limit", params.Limit),
		attribute.String("filter.status", params.Status),
	)

	var transactions []model.Transaction
	var total int64

	// Buat query dasar
	query := t.db.WithContext(ctx).Model(&model.Transaction{}).Where("customer_id = ?", customerID)
	countQuery := t.db.WithContext(ctx).Model(&model.Transaction{}).Where("customer_id = ?", customerID)

	// Terapkan filter status jika ada
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
		countQuery = countQuery.Where("status = ?", params.Status)
	}

	// Hitung total record (sebelum limit dan offset)
	if err := countQuery.Count(&total).Error; err != nil {
		span.SetStatus(codes.Error, "Error counting transactions")
		span.RecordError(err)

		t.log.Error("Error counting transactions",
			zap.Uint64("customer_id", customerID),
			zap.String("status", params.Status),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "count"),
				attribute.String("table", "transactions"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "transactions"),
				attribute.String("status", "error"),
			),
		)

		return nil, 0, err
	}

	// Terapkan paginasi
	offset := (params.Page - 1) * params.Limit
	query = query.Limit(params.Limit).Offset(offset).Order("transaction_date DESC")

	if err := query.Find(&transactions).Error; err != nil {
		span.SetStatus(codes.Error, "Error finding transactions paginated")
		span.RecordError(err)

		t.log.Error("Error finding transactions paginated",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "transactions"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_paginated"),
				attribute.String("table", "transactions"),
				attribute.String("status", "error"),
			),
		)

		return nil, 0, err
	}

	t.documentsRetrieved.Add(ctx, int64(len(transactions)),
		metric.WithAttributes(
			attribute.String("table", "transactions"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	t.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select_paginated"),
			attribute.String("table", "transactions"),
			attribute.String("status", "success"),
		),
	)

	t.log.Info("Transactions found paginated",
		zap.Uint64("customer_id", customerID),
		zap.Int64("total", total),
		zap.Int("retrieved", len(transactions)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Transactions found paginated")
	span.SetAttributes(
		attribute.Int64("result.total", total),
		attribute.Int("result.retrieved", len(transactions)),
	)

	return model.TransactionsToEntity(transactions), total, nil
}

// CreateTransaction implements TransactionRepository.
func (t *transactionRepository) CreateTransaction(ctx context.Context, transaction *domain.Transaction) error {
	ctx, span := t.tracer.Start(ctx, "repository.CreateTransaction")
	defer span.End()

	start := time.Now()

	t.log.Debug("Create transaction",
		zap.String("contract_number", transaction.ContractNumber),
		zap.Uint64("customer_id", transaction.CustomerID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	t.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "create_transaction"),
			attribute.String("table", "transactions"),
		),
	)
	defer t.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "create_transaction"),
			attribute.String("table", "transactions"),
		),
	)

	t.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "insert"),
			attribute.String("table", "transactions"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "insert"),
		attribute.String("db.table", "transactions"),
		attribute.String("transaction.contract_number", transaction.ContractNumber),
	)

	data := model.TransactionFromEntity(transaction)
	err := t.db.WithContext(ctx).Create(&data).Error
	if err != nil {
		span.SetStatus(codes.Error, "Error creating transaction")
		span.RecordError(err)

		t.log.Error("Error creating transaction",
			zap.String("contract_number", transaction.ContractNumber),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "insert"),
				attribute.String("table", "transactions"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "insert"),
				attribute.String("table", "transactions"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	t.documentsInserted.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("table", "transactions"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	t.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "insert"),
			attribute.String("table", "transactions"),
			attribute.String("status", "success"),
		),
	)

	t.log.Info("Transaction created successfully",
		zap.Uint64("transaction_id", data.ID),
		zap.String("contract_number", data.ContractNumber),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Transaction created successfully")
	span.SetAttributes(attribute.Int64("transaction.id", int64(data.ID)))

	// Update the original domain object with the generated ID
	transaction.ID = data.ID

	return nil
}

// SumActivePrincipalByCustomerIDAndTenorID implements TransactionRepository.
func (t *transactionRepository) SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error) {
	ctx, span := t.tracer.Start(ctx, "repository.SumActivePrincipalByCustomerIDAndTenorID")
	defer span.End()

	start := time.Now()

	t.log.Debug("Summing active principal for customer and tenor",
		zap.Uint64("customer_id", customerID),
		zap.Uint("tenor_id", tenorID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	t.connectionGauge.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "sum_active_principal"),
			attribute.String("table", "transactions"),
		),
	)
	defer t.connectionGauge.Add(ctx, -1,
		metric.WithAttributes(
			attribute.String("operation", "sum_active_principal"),
			attribute.String("table", "transactions"),
		),
	)

	t.queryCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "select_sum"),
			attribute.String("table", "transactions"),
		),
	)

	span.SetAttributes(
		attribute.String("db.operation", "select_sum"),
		attribute.String("db.table", "transactions"),
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("tenor.id", int(tenorID)),
	)

	var totalUsed float64
	err := t.db.WithContext(ctx).Model(&model.Transaction{}).
		Where("customer_id = ? AND tenor_id = ? AND status = ?", customerID, tenorID, model.TransactionActive).
		Select("COALESCE(SUM(otr_amount + admin_fee), 0)").
		Row().
		Scan(&totalUsed)
	if err != nil {
		span.SetStatus(codes.Error, "Error summing active principal")
		span.RecordError(err)

		t.log.Error("Error summing active principal",
			zap.Uint64("customer_id", customerID),
			zap.Uint("tenor_id", tenorID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		t.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "select_sum"),
				attribute.String("table", "transactions"),
				attribute.String("error", err.Error()),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		t.queryDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "select_sum"),
				attribute.String("table", "transactions"),
				attribute.String("status", "error"),
			),
		)

		return 0, err
	}

	duration := float64(time.Since(start).Milliseconds())
	t.queryDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "select_sum"),
			attribute.String("table", "transactions"),
			attribute.String("status", "success"),
		),
	)

	t.log.Debug("Sum of active principal retrieved successfully",
		zap.Uint64("customer_id", customerID),
		zap.Uint("tenor_id", tenorID),
		zap.Float64("total_used", totalUsed),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	span.SetStatus(codes.Ok, "Sum of active principal retrieved")
	span.SetAttributes(attribute.Float64("result.sum", totalUsed))

	return totalUsed, nil
}

func NewTransactionRepository(
	db *gorm.DB,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) TransactionRepository {
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

	return &transactionRepository{
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
