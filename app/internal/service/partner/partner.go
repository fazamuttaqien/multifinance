package partnersrv

import (
	"context"
	"fmt"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
	limitrepo "github.com/fazamuttaqien/multifinance/internal/repository/limit"
	tenorrepo "github.com/fazamuttaqien/multifinance/internal/repository/tenor"
	transactionrepo "github.com/fazamuttaqien/multifinance/internal/repository/transaction"
	"github.com/fazamuttaqien/multifinance/internal/service"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type partnerService struct {
	db                    *gorm.DB
	customerRepository    repository.CustomerRepository
	tenorRepository       repository.TenorRepository
	limitRepository       repository.LimitRepository
	transactionRepository repository.TransactionRepository

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger

	operationDuration   metric.Float64Histogram
	operationCount      metric.Int64Counter
	errorCount          metric.Int64Counter
	transactionsCreated metric.Int64Counter
	limitsChecked       metric.Int64Counter
}

// CreateTransaction implements PartnerServices.
func (p *partnerService) CreateTransaction(ctx context.Context, req dto.CreateTransactionRequest) (*domain.Transaction, error) {
	ctx, span := p.tracer.Start(ctx, "service.CreateTransaction")
	defer span.End()

	start := time.Now()

	p.log.Debug("Creating new transaction",
		zap.String("customer_nik", req.CustomerNIK),
		zap.Float64("otr_amount", req.OTRAmount),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "create_transaction"),
			attribute.String("service", "partner"),
		),
	)

	span.SetAttributes(
		attribute.String("customer.nik", req.CustomerNIK),
		attribute.Float64("transaction.otr_amount", req.OTRAmount),
		attribute.Int("transaction.tenor_months", int(req.TenorMonths)),
		attribute.String("service", "partner"),
	)

	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		span.SetStatus(codes.Error, "Failed to begin transaction")
		span.RecordError(tx.Error)
		p.log.Error("Failed to begin transaction",
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(tx.Error),
		)
		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "create_transaction"),
				attribute.String("service", "partner"),
				attribute.String("error_type", "transaction_begin_error"),
			),
		)
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "create_transaction"),
				attribute.String("service", "partner"),
				attribute.String("status", "error"),
			),
		)
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	// 1. Mendapatkan Customer berdasarkan NIK dan KUNCI barisnya untuk mencegah race condition
	customerTx := customerrepo.NewCustomerRepository(tx, p.meter, p.tracer, p.log)
	lockedCustomer, err := customerTx.FindByNIKWithLock(ctx, req.CustomerNIK)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding customer")
		span.RecordError(err)
		p.log.Error("Error finding customer by NIK with lock",
			zap.String("customer_nik", req.CustomerNIK),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)
		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "create_transaction"),
				attribute.String("service", "partner"),
				attribute.String("error_type", "customer_lookup_error"),
			),
		)
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, fmt.Errorf("error finding customer: %w", err)
	}
	if lockedCustomer == nil {
		err = common.ErrCustomerNotFound
		span.SetStatus(codes.Error, "Customer not found")
		span.RecordError(err)
		p.log.Warn("Customer not found for transaction creation", zap.String("customer_nik", req.CustomerNIK), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "customer_not_found")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	// Memastikan costumer sudah terverifikasi
	if lockedCustomer.VerificationStatus != domain.VerificationVerified {
		err = fmt.Errorf("customer with NIK %s is not verified", req.CustomerNIK)
		span.SetStatus(codes.Error, "Customer not verified")
		span.RecordError(err)
		p.log.Warn("Attempted transaction for unverified customer", zap.String("customer_nik", req.CustomerNIK), zap.String("status", string(lockedCustomer.VerificationStatus)), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "customer_not_verified")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	// 2. Mendapatkan Tenor
	tenorTx := tenorrepo.NewTenorRepository(
		tx,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	tenor, err := tenorTx.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding tenor")
		span.RecordError(err)
		p.log.Error("Error finding tenor", zap.Uint8("tenor_months", req.TenorMonths), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "tenor_lookup_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if tenor == nil {
		err = common.ErrTenorNotFound
		span.SetStatus(codes.Error, "Tenor not found")
		span.RecordError(err)
		p.log.Warn("Tenor not found for transaction creation", zap.Uint8("tenor_months", req.TenorMonths), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "tenor_not_found")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	// 3. Validasi ulang limit di dalam transanksi yang terkunci
	limitTx := limitrepo.NewLimitRepository(
		tx,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	limit, err := limitTx.FindByCustomerIDAndTenorID(ctx, lockedCustomer.ID, tenor.ID)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding limit")
		span.RecordError(err)
		p.log.Error("Error finding limit for customer and tenor", zap.Uint64("customer_id", lockedCustomer.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "limit_lookup_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if limit == nil {
		err = common.ErrLimitNotSet
		span.SetStatus(codes.Error, "Limit not set for customer")
		span.RecordError(err)
		p.log.Warn("Limit not set for customer", zap.Uint64("customer_id", lockedCustomer.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "limit_not_set")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	totalLimit := limit.LimitAmount

	transactionTx := transactionrepo.NewTransactionRepository(
		tx,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	usedAmount, err := transactionTx.SumActivePrincipalByCustomerIDAndTenorID(ctx, lockedCustomer.ID, tenor.ID)
	if err != nil {
		span.SetStatus(codes.Error, "Error calculating used amount")
		span.RecordError(err)
		p.log.Error("Error summing active principal", zap.Uint64("customer_id", lockedCustomer.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "sum_principal_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	remainingLimit := totalLimit - usedAmount
	transactionPrincipal := req.OTRAmount + req.AdminFee

	if remainingLimit < transactionPrincipal {
		err = common.ErrInsufficientLimit
		span.SetStatus(codes.Error, "Insufficient limit")
		span.RecordError(err)
		p.log.Warn("Insufficient limit for transaction",
			zap.String("customer_nik", req.CustomerNIK),
			zap.Float64("remaining_limit", remainingLimit),
			zap.Float64("required_principal", transactionPrincipal),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "insufficient_limit")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	// 4. Hitung komponen finansial lainnya (business logic)
	totalInterest := req.OTRAmount * 0.02 * float64(req.TenorMonths)
	totalInstallment := transactionPrincipal + totalInterest

	// 5. Generate contract number
	contractNumber := fmt.Sprintf("KTR-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%100000)

	// 6. Buat entitas Transaction baru
	newTransaction := domain.Transaction{
		ContractNumber:         contractNumber,
		CustomerID:             lockedCustomer.ID,
		TenorID:                tenor.ID,
		AssetName:              req.AssetName,
		OTRAmount:              req.OTRAmount,
		AdminFee:               req.AdminFee,
		TotalInterest:          totalInterest,
		TotalInstallmentAmount: totalInstallment,
		Status:                 domain.TransactionActive,
	}

	// 7. Simpan transaksi baru ke DB
	if err := transactionTx.CreateTransaction(ctx, &newTransaction); err != nil {
		span.SetStatus(codes.Error, "Failed to create transaction record")
		span.RecordError(err)
		p.log.Error("Failed to create transaction record", zap.String("contract_number", contractNumber), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "create_record_failed")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	// 8. Jika semua berhasil, commit transaksi
	if err := tx.Commit().Error; err != nil {
		span.SetStatus(codes.Error, "Failed to commit transaction")
		span.RecordError(err)
		p.log.Error("Failed to commit transaction", zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("error_type", "transaction_commit_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	p.transactionsCreated.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "partner")))
	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "create_transaction"), attribute.String("service", "partner"), attribute.String("status", "success")))
	p.log.Info("Transaction created successfully",
		zap.String("contract_number", newTransaction.ContractNumber),
		zap.Uint64("customer_id", newTransaction.CustomerID),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)
	span.SetStatus(codes.Ok, "Transaction created successfully")
	span.SetAttributes(attribute.String("transaction.contract_number", newTransaction.ContractNumber))

	return &newTransaction, nil
}

// CheckLimit implements PartnerUsecases.
func (p *partnerService) CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error) {
	ctx, span := p.tracer.Start(ctx, "service.CheckLimit")
	defer span.End()

	start := time.Now()

	p.log.Debug("Checking customer limit",
		zap.String("customer_nik", req.CustomerNIK),
		zap.Uint8("tenor_months", req.TenorMonths),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "check_limit"),
			attribute.String("service", "partner"),
		),
	)

	span.SetAttributes(
		attribute.String("customer.nik", req.CustomerNIK),
		attribute.Int("transaction.tenor_months", int(req.TenorMonths)),
		attribute.Float64("transaction.amount", req.TransactionAmount),
		attribute.String("service", "partner"),
	)

	// 1. Validasi Customer & Tenor
	cust, err := p.customerRepository.FindByNIK(ctx, req.CustomerNIK)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding customer")
		span.RecordError(err)
		p.log.Error("Error finding customer by NIK", zap.String("customer_nik", req.CustomerNIK), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "customer_lookup_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if cust == nil {
		err = common.ErrCustomerNotFound
		span.SetStatus(codes.Error, "Customer not found")
		span.RecordError(err)
		p.log.Warn("Customer not found for limit check", zap.String("customer_nik", req.CustomerNIK), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "customer_not_found")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if cust.VerificationStatus != domain.VerificationVerified {
		err = fmt.Errorf("customer %s is not verified", req.CustomerNIK)
		span.SetStatus(codes.Error, "Customer not verified")
		span.RecordError(err)
		p.log.Warn("Attempted limit check for unverified customer", zap.String("customer_nik", req.CustomerNIK), zap.String("status", string(cust.VerificationStatus)), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "customer_not_verified")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	tenor, err := p.tenorRepository.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding tenor")
		span.RecordError(err)
		p.log.Error("Error finding tenor", zap.Uint8("tenor_months", req.TenorMonths), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "tenor_lookup_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if tenor == nil {
		err = common.ErrTenorNotFound
		span.SetStatus(codes.Error, "Tenor not found")
		span.RecordError(err)
		p.log.Warn("Tenor not found for limit check", zap.Uint8("tenor_months", req.TenorMonths), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "tenor_not_found")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	// 2. Hitung Sisa Limit
	limit, err := p.limitRepository.FindByCustomerIDAndTenorID(ctx, cust.ID, tenor.ID)
	if err != nil {
		span.SetStatus(codes.Error, "Error finding limit")
		span.RecordError(err)
		p.log.Error("Error finding limit for customer and tenor", zap.Uint64("customer_id", cust.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "limit_lookup_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}
	if limit == nil {
		err = common.ErrLimitNotSet
		span.SetStatus(codes.Error, "Limit not set for customer")
		span.RecordError(err)
		p.log.Warn("Limit not set for customer", zap.Uint64("customer_id", cust.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "limit_not_set")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(
		ctx, cust.ID, tenor.ID)
	if err != nil {
		span.SetStatus(codes.Error, "Error calculating used amount")
		span.RecordError(err)
		p.log.Error("Error summing active principal", zap.Uint64("customer_id", cust.ID), zap.Uint("tenor_id", tenor.ID), zap.String("trace_id", span.SpanContext().TraceID().String()), zap.Error(err))
		p.errorCount.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("error_type", "sum_principal_error")))
		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "error")))
		return nil, err
	}

	remainingLimit := limit.LimitAmount - usedAmount

	// 3. Buat Response
	var response *dto.CheckLimitResponse
	if remainingLimit >= req.TransactionAmount {
		response = &dto.CheckLimitResponse{
			Status:         "approved",
			Message:        "Limit is sufficient.",
			RemainingLimit: remainingLimit,
		}
	} else {
		response = &dto.CheckLimitResponse{
			Status:         "rejected",
			Message:        "Insufficient limit for this transaction.",
			RemainingLimit: remainingLimit,
		}
	}

	p.limitsChecked.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "partner"), attribute.String("status", response.Status)))
	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration, metric.WithAttributes(attribute.String("operation", "check_limit"), attribute.String("service", "partner"), attribute.String("status", "success")))
	p.log.Info("Limit check completed successfully",
		zap.String("customer_nik", req.CustomerNIK),
		zap.String("check_status", response.Status),
		zap.Float64("remaining_limit", remainingLimit),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)
	span.SetStatus(codes.Ok, "Limit check completed")
	span.SetAttributes(
		attribute.String("limit_check.status", response.Status),
		attribute.Float64("limit_check.remaining", remainingLimit),
	)

	return response, nil
}

func NewPartnerService(
	db *gorm.DB,
	customerRepository repository.CustomerRepository,
	tenorRepository repository.TenorRepository,
	limitRepository repository.LimitRepository,
	transactionRepository repository.TransactionRepository,

	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) service.PartnerServices {
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
	transactionsCreated, _ := meter.Int64Counter(
		"service.transactions.created",
		metric.WithDescription("Number of transactions created"),
		metric.WithUnit("{transaction}"),
	)
	limitsChecked, _ := meter.Int64Counter(
		"service.limits.checked",
		metric.WithDescription("Number of limit checks performed"),
		metric.WithUnit("{check}"),
	)

	return &partnerService{
		db:                    db,
		customerRepository:    customerRepository,
		tenorRepository:       tenorRepository,
		limitRepository:       limitRepository,
		transactionRepository: transactionRepository,
		meter:                 meter,
		tracer:                tracer,
		log:                   log,
		operationDuration:     operationDuration,
		operationCount:        operationCount,
		errorCount:            errorCount,
		transactionsCreated:   transactionsCreated,
		limitsChecked:         limitsChecked,
	}
}
