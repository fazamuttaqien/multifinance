package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/model"
	"github.com/fazamuttaqien/multifinance/repository"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type adminService struct {
	db                 *gorm.DB
	customerRepository repository.CustomerRepository
	meter              metric.Meter
	tracer             trace.Tracer
	log                *zap.Logger
	operationDuration  metric.Float64Histogram
	operationCount     metric.Int64Counter
	errorCount         metric.Int64Counter
	limitsSet          metric.Int64Counter
	customersVerified  metric.Int64Counter
	customersRetrieved metric.Int64Counter
}

// SetLimits implements AdminUsecases.
func (a *adminService) SetLimits(ctx context.Context, customerID uint64, req dto.SetLimits) error {
	ctx, span := a.tracer.Start(ctx, "service.SetLimits")
	defer span.End()

	start := time.Now()

	a.log.Debug("Setting customer limits",
		zap.Uint64("customer_id", customerID),
		zap.Int("limits_count", len(req.Limits)),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	a.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "set_limits"),
			attribute.String("service", "admin"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("limits.count", len(req.Limits)),
		attribute.String("service", "admin"),
	)

	// Start transaction
	tx := a.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		span.SetStatus(codes.Error, "Failed to begin transaction")
		span.RecordError(tx.Error)

		a.log.Error("Failed to begin transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(tx.Error),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "transaction_begin_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return tx.Error
	}
	defer tx.Rollback()

	// 1. Validasi customer
	customerTx := repository.NewCustomerRepository(tx, a.meter, a.tracer, a.log)
	customer, err := customerTx.FindByID(ctx, customerID)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to find customer")
		span.RecordError(err)

		a.log.Error("Failed to find customer",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "customer_lookup_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return fmt.Errorf("error finding customer: %w", err)
	}

	if customer == nil {
		err := common.ErrCustomerNotFound
		span.SetStatus(codes.Error, "Customer not found")
		span.RecordError(err)

		a.log.Warn("Customer not found for setting limits",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "customer_not_found"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	limitsToUpsert := make([]domain.CustomerLimit, 0, len(req.Limits))
	tenorTx := repository.NewTenorRepository(
		tx,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// 2. Loop dan validasi setiap item limit dalam request
	for _, item := range req.Limits {
		if item.LimitAmount < 0 {
			err := common.ErrInvalidLimitAmount
			span.SetStatus(codes.Error, "Invalid limit amount")
			span.RecordError(err)

			a.log.Error("Invalid limit amount",
				zap.Uint64("customer_id", customerID),
				zap.Uint8("tenor_months", item.TenorMonths),
				zap.Float64("limit_amount", item.LimitAmount),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			a.errorCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("error_type", "invalid_limit_amount"),
				),
			)

			duration := float64(time.Since(start).Milliseconds())
			a.operationDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("status", "error"),
				),
			)

			return err
		}

		// Cari tenor ID berdasarkan durasi bulan
		tenor, err := tenorTx.FindByDuration(ctx, item.TenorMonths)
		if err != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("Failed to find tenor for %d months", item.TenorMonths))
			span.RecordError(err)

			a.log.Error("Failed to find tenor",
				zap.Uint64("customer_id", customerID),
				zap.Uint8("tenor_months", item.TenorMonths),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
				zap.Error(err),
			)

			a.errorCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("error_type", "tenor_lookup_error"),
				),
			)

			duration := float64(time.Since(start).Milliseconds())
			a.operationDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("status", "error"),
				),
			)

			return fmt.Errorf("error finding tenor for %d months: %w", item.TenorMonths, err)
		}

		if tenor == nil {
			err := fmt.Errorf("%w: for %d months", common.ErrTenorNotFound, item.TenorMonths)
			span.SetStatus(codes.Error, fmt.Sprintf("Tenor not found for %d months", item.TenorMonths))
			span.RecordError(err)

			a.log.Error("Tenor not found",
				zap.Uint64("customer_id", customerID),
				zap.Uint8("tenor_months", item.TenorMonths),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
			)

			a.errorCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("error_type", "tenor_not_found"),
				),
			)

			duration := float64(time.Since(start).Milliseconds())
			a.operationDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("status", "error"),
				),
			)

			return err
		}

		// Menyiapkan data untuk di upsert
		limitsToUpsert = append(limitsToUpsert, domain.CustomerLimit{
			CustomerID:  customerID,
			TenorID:     tenor.ID,
			LimitAmount: item.LimitAmount,
		})

		a.log.Debug("Prepared limit for upsert",
			zap.Uint64("customer_id", customerID),
			zap.Uint("tenor_id", tenor.ID),
			zap.Uint8("tenor_months", item.TenorMonths),
			zap.Float64("limit_amount", item.LimitAmount),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)
	}

	// 3. Melakukan operasi upsert massal
	if len(limitsToUpsert) > 0 {
		limitTx := repository.NewLimitRepository(
			tx,
			otel.GetMeterProvider().Meter(""),
			otel.GetTracerProvider().Tracer(""),
			zap.L(),
		)
		if err := limitTx.UpsertMany(ctx, limitsToUpsert); err != nil {
			span.SetStatus(codes.Error, "Failed to upsert limits")
			span.RecordError(err)

			a.log.Error("Failed to upsert limits",
				zap.Uint64("customer_id", customerID),
				zap.Int("limits_count", len(limitsToUpsert)),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
				zap.Error(err),
			)

			a.errorCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("error_type", "upsert_failed"),
				),
			)

			duration := float64(time.Since(start).Milliseconds())
			a.operationDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "set_limits"),
					attribute.String("service", "admin"),
					attribute.String("status", "error"),
				),
			)

			return fmt.Errorf("failed to upsert limits: %w", err)
		}
	}

	// 4. Jika semua berhasil, commit transaksi
	if err := tx.Commit().Error; err != nil {
		span.SetStatus(codes.Error, "Failed to commit transaction")
		span.RecordError(err)

		a.log.Error("Failed to commit transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "transaction_commit_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "set_limits"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	a.limitsSet.Add(ctx, int64(len(limitsToUpsert)),
		metric.WithAttributes(
			attribute.String("service", "admin"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	a.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "set_limits"),
			attribute.String("service", "admin"),
			attribute.String("status", "success"),
		),
	)

	a.log.Info("Customer limits set successfully",
		zap.Uint64("customer_id", customerID),
		zap.Int("limits_count", len(limitsToUpsert)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer limits set successfully")
	span.SetAttributes(
		attribute.Int("limits.processed", len(limitsToUpsert)),
	)

	return nil
}

// GetCustomerByNIK implements AdminUsecases.
func (a *adminService) GetCustomerByID(ctx context.Context, customerID uint64) (*domain.Customer, error) {
	ctx, span := a.tracer.Start(ctx, "service.GetCustomerByID")
	defer span.End()

	start := time.Now()

	a.log.Debug("Getting customer by ID",
		zap.Uint64("customer_id", customerID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	a.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "get_customer_by_id"),
			attribute.String("service", "admin"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("service", "admin"),
	)

	customer, err := a.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer")
		span.RecordError(err)

		a.log.Error("Failed to fetch customer by ID",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_customer_by_id"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_customer_by_id"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	if customer == nil {
		err := common.ErrCustomerNotFound
		span.SetStatus(codes.Error, "Customer not found")
		span.RecordError(err)

		a.log.Warn("Customer not found",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_customer_by_id"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "customer_not_found"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_customer_by_id"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	a.customersRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("service", "admin"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	a.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "get_customer_by_id"),
			attribute.String("service", "admin"),
			attribute.String("status", "success"),
		),
	)

	a.log.Info("Customer retrieved successfully",
		zap.Uint64("customer_id", customerID),
		zap.String("full_name", customer.FullName),
		zap.String("verification_status", string(customer.VerificationStatus)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer retrieved successfully")
	span.SetAttributes(
		attribute.String("customer.full_name", customer.FullName),
		attribute.String("verification_status", string(customer.VerificationStatus)),
	)

	return customer, nil
}

// ListCustomers implements AdminUsecases.
func (a *adminService) ListCustomers(ctx context.Context, params domain.Params) (*domain.Paginated, error) {
	ctx, span := a.tracer.Start(ctx, "service.ListCustomers")
	defer span.End()

	start := time.Now()

	a.log.Debug("Listing customers",
		zap.Int("page", params.Page),
		zap.Int("limit", params.Limit),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	a.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "list_customers"),
			attribute.String("service", "admin"),
		),
	)

	span.SetAttributes(
		attribute.Int("pagination.page", params.Page),
		attribute.Int("pagination.limit", params.Limit),
		attribute.String("service", "admin"),
	)

	customers, total, err := a.customerRepository.FindPaginated(ctx, params)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customers")
		span.RecordError(err)

		a.log.Error("Failed to fetch customers",
			zap.Int("page", params.Page),
			zap.Int("limit", params.Limit),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "list_customers"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "list_customers"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	result := &domain.Paginated{
		Data:       customers,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}

	duration := float64(time.Since(start).Milliseconds())
	a.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "list_customers"),
			attribute.String("service", "admin"),
			attribute.String("status", "success"),
		),
	)

	a.log.Info("Customers listed successfully",
		zap.Int64("total_customers", total),
		zap.Int("current_page", params.Page),
		zap.Int("total_pages", totalPages),
		zap.Int("returned_count", len(customers)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customers listed successfully")
	span.SetAttributes(
		attribute.Int64("customers.total", total),
		attribute.Int("pagination.total_pages", totalPages),
		attribute.Int("customers.returned", len(customers)),
	)

	return result, nil
}

// VerifyCustomer implements AdminUsecases.
func (a *adminService) VerifyCustomer(ctx context.Context, customerID uint64, req dto.VerificationRequest) error {
	ctx, span := a.tracer.Start(ctx, "service.VerifyCustomer")
	defer span.End()

	start := time.Now()

	a.log.Debug("Verifying customer",
		zap.Uint64("customer_id", customerID),
		zap.String("new_status", string(req.Status)),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	a.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "verify_customer"),
			attribute.String("service", "admin"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("verification.new_status", string(req.Status)),
		attribute.String("service", "admin"),
	)

	tx := a.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		span.SetStatus(codes.Error, "Failed to begin transaction")
		span.RecordError(tx.Error)

		a.log.Error("Failed to begin transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(tx.Error),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "transaction_begin_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return tx.Error
	}
	defer tx.Rollback()

	var customer model.Customer
	if err := tx.First(&customer, customerID).Error; err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer for verification")
		span.RecordError(err)

		var errorType string
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorType = "customer_not_found"
			err = common.ErrCustomerNotFound
		} else {
			errorType = "repository_error"
		}

		a.log.Error("Failed to fetch customer for verification",
			zap.Uint64("customer_id", customerID),
			zap.String("error_type", errorType),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("error_type", errorType),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	// Validasi: hanya bisa verifikasi customer yang statusnya PENDING
	if customer.VerificationStatus != model.VerificationPending {
		err := fmt.Errorf("customer is not in PENDING state, current state: %s", customer.VerificationStatus)
		span.SetStatus(codes.Error, "Customer not in pending state")
		span.RecordError(err)

		a.log.Error("Customer verification failed - not in pending state",
			zap.Uint64("customer_id", customerID),
			zap.String("current_status", string(customer.VerificationStatus)),
			zap.String("requested_status", string(req.Status)),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "invalid_state_transition"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	oldStatus := customer.VerificationStatus
	customer.VerificationStatus = model.VerificationStatus(req.Status)

	if err := tx.Model(&customer).Update("verification_status", req.Status).Error; err != nil {
		span.SetStatus(codes.Error, "Failed to update verification status")
		span.RecordError(err)

		a.log.Error("Failed to update verification status",
			zap.Uint64("customer_id", customerID),
			zap.String("old_status", string(oldStatus)),
			zap.String("new_status", string(req.Status)),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "update_failed"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	if err := tx.Commit().Error; err != nil {
		span.SetStatus(codes.Error, "Failed to commit transaction")
		span.RecordError(err)

		a.log.Error("Failed to commit transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		a.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("error_type", "transaction_commit_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		a.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "verify_customer"),
				attribute.String("service", "admin"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	a.customersVerified.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("service", "admin"),
			attribute.String("verification_status", string(req.Status)),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	a.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "verify_customer"),
			attribute.String("service", "admin"),
			attribute.String("status", "success"),
		),
	)

	a.log.Info("Customer verification completed successfully",
		zap.Uint64("customer_id", customerID),
		zap.String("old_status", string(oldStatus)),
		zap.String("new_status", string(req.Status)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer verification completed successfully")
	span.SetAttributes(
		attribute.String("verification.old_status", string(oldStatus)),
		attribute.String("verification.new_status", string(req.Status)),
	)

	return nil
}

func NewAdminService(
	db *gorm.DB,
	customerRepository repository.CustomerRepository,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) AdminServices {
	operationDuration, _ := meter.Float64Histogram(
		"service.operation.duration",
		metric.WithDescription("Duration of service operations"),
		metric.WithUnit("ms"),
	)

	operationCount, _ := meter.Int64Counter(
		"service.operation.count",
		metric.WithDescription("Number of service operations"),
		metric.WithUnit("{operation}"),
	)

	errorCount, _ := meter.Int64Counter(
		"service.error.count",
		metric.WithDescription("Number of service errors"),
		metric.WithUnit("{error}"),
	)

	limitsSet, _ := meter.Int64Counter(
		"service.limit.sets",
		metric.WithDescription("Number of service limit sets"),
		metric.WithUnit("{limit}"),
	)

	customersVerified, _ := meter.Int64Counter(
		"service.customers.verified",
		metric.WithDescription("Number of customers verified"),
		metric.WithUnit("{customer}"),
	)

	customersRetrieved, _ := meter.Int64Counter(
		"service.customers.retrieved",
		metric.WithDescription("Number of customers retrieved"),
		metric.WithUnit("{customer}"),
	)

	return &adminService{
		db:                 db,
		customerRepository: customerRepository,
		meter:              meter,
		tracer:             tracer,
		log:                log,
		operationDuration:  operationDuration,
		operationCount:     operationCount,
		errorCount:         errorCount,
		limitsSet:          limitsSet,
		customersVerified:  customersVerified,
		customersRetrieved: customersRetrieved,
	}
}
