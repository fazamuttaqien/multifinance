package profilesrv

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	"github.com/fazamuttaqien/multifinance/internal/service"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type profileService struct {
	db                    *gorm.DB
	customerRepository    repository.CustomerRepository
	limitRepository       repository.LimitRepository
	tenorRepository       repository.TenorRepository
	transactionRepository repository.TransactionRepository

	meter             metric.Meter
	tracer            trace.Tracer
	log               *zap.Logger
	operationDuration metric.Float64Histogram
	operationCount    metric.Int64Counter
	errorCount        metric.Int64Counter
	profilesCreated   metric.Int64Counter
	profilesRetrieved metric.Int64Counter
	profilesUpdated   metric.Int64Counter
}

// Create implements ProfileUsecases
func (p *profileService) Create(ctx context.Context, customer *domain.Customer) (*domain.Customer, error) {
	ctx, span := p.tracer.Start(ctx, "service.CreateProfile")
	defer span.End()

	start := time.Now()

	p.log.Debug("Creating new customer profile",
		zap.String("nik", customer.NIK),
		zap.String("full_name", customer.FullName),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "create_profile"),
			attribute.String("service", "profile"),
		),
	)

	span.SetAttributes(
		attribute.String("customer.nik", customer.NIK),
		attribute.String("customer.full_name", customer.FullName),
		attribute.String("service", "profile"),
	)

	// 1. Cek duplikasi NIK
	existingCustomer, err := p.customerRepository.FindByNIK(ctx, customer.NIK)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		span.SetStatus(codes.Error, "Failed to check existing customer")
		span.RecordError(err)

		p.log.Error("Failed to check existing customer",
			zap.String("nik", customer.NIK),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	if existingCustomer != nil {
		err := common.ErrNIKExists
		span.SetStatus(codes.Error, "Customer already exists")
		span.RecordError(err)

		p.log.Warn("Customer already registered",
			zap.String("nik", customer.NIK),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "duplicate_customer"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	// 4. Buat entitas customer baru
	// newCustomer := domain.Customer{
	// 	NIK:                customer.NIK,
	// 	FullName:           customer.FullName,
	// 	LegalName:          customer.LegalName,
	// 	BirthPlace:         customer.BirthPlace,
	// 	BirthDate:          customer.BirthDate,
	// 	Salary:             customer.Salary,
	// 	KtpUrl:             customer.KtpUrl,
	// 	SelfieUrl:          customer.SelfieUrl,
	// 	VerificationStatus: domain.VerificationPending,
	// }

	customer.VerificationStatus = domain.VerificationPending

	// 5. Simpan ke database
	data, err := p.customerRepository.CreateCustomer(ctx, customer)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to create customer")
		span.RecordError(err)

		p.log.Error("Failed to create customer",
			zap.String("nik", customer.NIK),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "create_failed"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "create_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	p.profilesCreated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("service", "profile"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "create_profile"),
			attribute.String("service", "profile"),
			attribute.String("status", "success"),
		),
	)

	p.log.Info("Customer profile created successfully",
		zap.String("nik", data.NIK),
		zap.String("full_name", data.FullName),
		zap.Uint64("customer_id", data.ID),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer profile created successfully")
	span.SetAttributes(
		attribute.Int64("customer.id", int64(data.ID)),
		attribute.String("verification_status", string(data.VerificationStatus)),
	)

	return data, nil
}

// GetMyLimits implements ProfileUsecases
func (p *profileService) GetMyLimits(ctx context.Context, customerID uint64) ([]dto.LimitDetailResponse, error) {
	ctx, span := p.tracer.Start(ctx, "service.GetMyLimits")
	defer span.End()

	start := time.Now()

	p.log.Debug("Getting customer limits",
		zap.Uint64("customer_id", customerID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "get_limits"),
			attribute.String("service", "profile"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("service", "profile"),
	)

	// 1. Ambil semua limit yang ditetapkan untuk customer
	customerLimits, err := p.limitRepository.FindAllByCustomerID(ctx, customerID)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer limits")
		span.RecordError(err)

		p.log.Error("Failed to fetch customer limits",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_limits"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_limits"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	// 2. Ambil semua data tenor untuk mapping ID ke durasi bulan
	allTenors, err := p.tenorRepository.FindAll(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch tenors")
		span.RecordError(err)

		p.log.Error("Failed to fetch tenors",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_limits"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "tenor_fetch_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_limits"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	tenorMap := make(map[uint]uint8)
	for _, tenor := range allTenors {
		tenorMap[tenor.ID] = tenor.DurationMonths
	}

	// 3. Menyiapkan response
	response := make([]dto.LimitDetailResponse, 0, len(customerLimits))

	for _, limit := range customerLimits {
		// Hitung pemakaian tenor ini
		usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(ctx, customerID, limit.TenorID)
		if err != nil {
			span.SetStatus(codes.Error, fmt.Sprintf("Failed to calculate used amount for tenor %d", limit.TenorID))
			span.RecordError(err)

			p.log.Error("Failed to calculate used amount",
				zap.Uint64("customer_id", customerID),
				zap.Uint("tenor_id", limit.TenorID),
				zap.String("trace_id", span.SpanContext().TraceID().String()),
				zap.Error(err),
			)

			p.errorCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("operation", "get_limits"),
					attribute.String("service", "profile"),
					attribute.String("error_type", "used_amount_calculation_error"),
				),
			)

			duration := float64(time.Since(start).Milliseconds())
			p.operationDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.String("operation", "get_limits"),
					attribute.String("service", "profile"),
					attribute.String("status", "error"),
				),
			)

			return nil, fmt.Errorf("failed to calculate used amount for tenor %d: %w", limit.TenorID, err)
		}

		detail := dto.LimitDetailResponse{
			TenorMonths:    tenorMap[limit.TenorID],
			LimitAmount:    limit.LimitAmount,
			UsedAmount:     usedAmount,
			RemainingLimit: limit.LimitAmount - usedAmount,
		}
		response = append(response, detail)
	}

	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "get_limits"),
			attribute.String("service", "profile"),
			attribute.String("status", "success"),
		),
	)

	p.log.Info("Customer limits retrieved successfully",
		zap.Uint64("customer_id", customerID),
		zap.Int("limits_count", len(response)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer limits retrieved successfully")
	span.SetAttributes(
		attribute.Int("limits.count", len(response)),
	)

	return response, nil
}

// GetMyTransactions implements ProfileUsecases
func (p *profileService) GetMyTransactions(ctx context.Context, customerID uint64, params domain.Params) (*domain.Paginated, error) {
	ctx, span := p.tracer.Start(ctx, "service.GetMyTransactions")
	defer span.End()

	start := time.Now()

	p.log.Debug("Getting customer transactions",
		zap.Uint64("customer_id", customerID),
		zap.Int("page", params.Page),
		zap.Int("limit", params.Limit),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "get_transactions"),
			attribute.String("service", "profile"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.Int("pagination.page", params.Page),
		attribute.Int("pagination.limit", params.Limit),
		attribute.String("service", "profile"),
	)

	transactions, total, err := p.transactionRepository.FindPaginatedByCustomerID(ctx, customerID, params)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer transactions")
		span.RecordError(err)

		p.log.Error("Failed to fetch customer transactions",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_transactions"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_transactions"),
				attribute.String("service", "profile"),
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
		Data:       transactions,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}

	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "get_transactions"),
			attribute.String("service", "profile"),
			attribute.String("status", "success"),
		),
	)

	p.log.Info("Customer transactions retrieved successfully",
		zap.Uint64("customer_id", customerID),
		zap.Int64("total_transactions", total),
		zap.Int("current_page", params.Page),
		zap.Int("total_pages", totalPages),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer transactions retrieved successfully")
	span.SetAttributes(
		attribute.Int64("transactions.total", total),
		attribute.Int("pagination.total_pages", totalPages),
	)

	return result, nil
}

// GetMyProfile implements ProfileUsecases
func (p *profileService) GetMyProfile(ctx context.Context, customerID uint64) (*domain.Customer, error) {
	ctx, span := p.tracer.Start(ctx, "service.GetMyProfile")
	defer span.End()

	start := time.Now()

	p.log.Debug("Getting customer profile",
		zap.Uint64("customer_id", customerID),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "get_profile"),
			attribute.String("service", "profile"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("service", "profile"),
	)

	customer, err := p.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer profile")
		span.RecordError(err)

		p.log.Error("Failed to fetch customer profile",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "repository_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	if customer == nil {
		err := common.ErrCustomerNotFound
		span.SetStatus(codes.Error, "Customer not found")
		span.RecordError(err)

		p.log.Warn("Customer not found",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "get_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "customer_not_found"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "get_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return nil, err
	}

	p.profilesRetrieved.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("service", "profile"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "get_profile"),
			attribute.String("service", "profile"),
			attribute.String("status", "success"),
		),
	)

	p.log.Info("Customer profile retrieved successfully",
		zap.Uint64("customer_id", customerID),
		zap.String("full_name", customer.FullName),
		zap.String("verification_status", string(customer.VerificationStatus)),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer profile retrieved successfully")
	span.SetAttributes(
		attribute.String("customer.full_name", customer.FullName),
		attribute.String("verification_status", string(customer.VerificationStatus)),
	)

	return customer, nil
}

// Update implements ProfileUsecases
func (p *profileService) Update(ctx context.Context, customerID uint64, req domain.Customer) error {
	ctx, span := p.tracer.Start(ctx, "service.UpdateProfile")
	defer span.End()

	start := time.Now()

	p.log.Debug("Updating customer profile",
		zap.Uint64("customer_id", customerID),
		zap.String("full_name", req.FullName),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	p.operationCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("operation", "update_profile"),
			attribute.String("service", "profile"),
		),
	)

	span.SetAttributes(
		attribute.Int64("customer.id", int64(customerID)),
		attribute.String("customer.full_name", req.FullName),
		attribute.String("service", "profile"),
	)

	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		span.SetStatus(codes.Error, "Failed to begin transaction")
		span.RecordError(tx.Error)

		p.log.Error("Failed to begin transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(tx.Error),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "transaction_begin_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return tx.Error
	}
	defer tx.Rollback()

	var customer model.Customer
	if err := tx.First(&customer, customerID).Error; err != nil {
		span.SetStatus(codes.Error, "Failed to fetch customer for update")
		span.RecordError(err)

		var errorType string
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorType = "customer_not_found"
			err = common.ErrCustomerNotFound
		} else {
			errorType = "repository_error"
		}

		p.log.Error("Failed to fetch customer for update",
			zap.Uint64("customer_id", customerID),
			zap.String("error_type", errorType),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", errorType),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	updates := map[string]any{
		"full_name": req.FullName,
		"salary":    req.Salary,
	}

	customer.FullName = req.FullName
	customer.Salary = req.Salary

	if err := tx.Model(&customer).Updates(updates).Error; err != nil {
		span.SetStatus(codes.Error, "Failed to update customer")
		span.RecordError(err)

		p.log.Error("Failed to update customer",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "update_failed"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	if err := tx.Commit().Error; err != nil {
		span.SetStatus(codes.Error, "Failed to commit transaction")
		span.RecordError(err)

		p.log.Error("Failed to commit transaction",
			zap.Uint64("customer_id", customerID),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.Error(err),
		)

		p.errorCount.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("error_type", "transaction_commit_error"),
			),
		)

		duration := float64(time.Since(start).Milliseconds())
		p.operationDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("operation", "update_profile"),
				attribute.String("service", "profile"),
				attribute.String("status", "error"),
			),
		)

		return err
	}

	p.profilesUpdated.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("service", "profile"),
		),
	)

	duration := float64(time.Since(start).Milliseconds())
	p.operationDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("operation", "update_profile"),
			attribute.String("service", "profile"),
			attribute.String("status", "success"),
		),
	)

	p.log.Info("Customer profile updated successfully",
		zap.Uint64("customer_id", customerID),
		zap.String("full_name", req.FullName),
		zap.Float64("duration_ms", duration),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
		zap.String("span_id", span.SpanContext().SpanID().String()),
	)

	span.SetStatus(codes.Ok, "Customer profile updated successfully")

	return nil
}

func NewProfileService(
	db *gorm.DB,
	customerRepository repository.CustomerRepository,
	limitRepository repository.LimitRepository,
	tenorRepository repository.TenorRepository,
	transactionRepository repository.TransactionRepository,
	meter metric.Meter,
	tracer trace.Tracer,
	log *zap.Logger,
) service.ProfileServices {
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

	profilesCreated, _ := meter.Int64Counter(
		"service.profiles.created",
		metric.WithDescription("Number of profiles created"),
		metric.WithUnit("{profile}"),
	)

	profilesRetrieved, _ := meter.Int64Counter(
		"service.profiles.retrieved",
		metric.WithDescription("Number of profiles retrieved"),
		metric.WithUnit("{profile}"),
	)

	profilesUpdated, _ := meter.Int64Counter(
		"service.profiles.updated",
		metric.WithDescription("Number of profiles updated"),
		metric.WithUnit("{profile}"),
	)

	return &profileService{
		db:                    db,
		customerRepository:    customerRepository,
		limitRepository:       limitRepository,
		tenorRepository:       tenorRepository,
		transactionRepository: transactionRepository,

		meter:             meter,
		tracer:            tracer,
		log:               log,
		operationDuration: operationDuration,
		operationCount:    operationCount,
		errorCount:        errorCount,
		profilesCreated:   profilesCreated,
		profilesRetrieved: profilesRetrieved,
		profilesUpdated:   profilesUpdated,
	}
}
