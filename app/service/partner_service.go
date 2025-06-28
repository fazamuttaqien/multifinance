package service

import (
	"context"
	"fmt"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/repository"
	"gorm.io/gorm"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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

	// operationDuration metric.Float64Histogram
	// operationCount    metric.Int64Counter
	// errorCount        metric.Int64Counter
	// profilesCreated   metric.Int64Counter
	// profilesRetrieved metric.Int64Counter
	// profilesUpdated   metric.Int64Counter
}

// CreateTransaction implements PartnerServices.
func (p *partnerService) CreateTransaction(ctx context.Context, req dto.CreateTransactionRequest) (*domain.Transaction, error) {
	// Start Transaction
	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	// 1. Mendapatkan Customer berdasarkan NIK dan KUNCI barisnya untuk mencegah race condition
	customerTx := repository.NewCustomerRepository(tx, p.meter, p.tracer, p.log)
	lockedCustomer, err := customerTx.FindByNIKWithLock(ctx, req.CustomerNIK)
	if err != nil {
		return nil, fmt.Errorf("error finding customer: %w", err)
	}
	if lockedCustomer == nil {
		return nil, common.ErrCustomerNotFound
	}

	// Memastikan costumer sudah terverifikasi
	if lockedCustomer.VerificationStatus != domain.VerificationVerified {
		return nil, fmt.Errorf("customer with NIK %s is not verified", req.CustomerNIK)
	}

	// 2. Mendapatkan Tenor
	tenorTx := repository.NewTenorRepository(tx)
	tenor, err := tenorTx.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		return nil, err
	}
	if tenor == nil {
		return nil, common.ErrTenorNotFound
	}

	// 3. Validasi ulang limit di dalam transanksi yang terkunci
	limitTx := repository.NewLimitRepository(tx)
	limit, err := limitTx.FindByCustomerIDAndTenorID(ctx, lockedCustomer.ID, tenor.ID)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		return nil, common.ErrLimitNotSet
	}
	totalLimit := limit.LimitAmount

	transactionTx := repository.NewTransactionRepository(tx)
	usedAmount, err := transactionTx.SumActivePrincipalByCustomerIDAndTenorID(ctx, lockedCustomer.ID, tenor.ID)
	if err != nil {
		return nil, err
	}

	remainingLimit := totalLimit - usedAmount
	transactionPrincipal := req.OTRAmount + req.AdminFee

	if remainingLimit < transactionPrincipal {
		return nil, common.ErrInsufficientLimit
	}

	// 4. Hitung komponen finansial lainnya (business logic)
	// Aturan bunga sederhana: 2% dari OTR per bulan tenor
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
	repoTx := repository.NewTransactionRepository(tx)
	if err := repoTx.CreateTransaction(ctx, &newTransaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	// 8. Jika semua berhasil, commit transaksi
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &newTransaction, nil
}

// CheckLimit implements PartnerUsecases.
func (p *partnerService) CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error) {
	// 1. Validasi Customer & Tenor
	cust, err := p.customerRepository.FindByNIK(ctx, req.NIK)
	if err != nil {
		return nil, err
	}
	if cust == nil {
		return nil, common.ErrCustomerNotFound
	}
	if cust.VerificationStatus != domain.VerificationVerified {
		return nil, fmt.Errorf("customer %s is not verified", req.NIK)
	}

	tenor, err := p.tenorRepository.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		return nil, err
	}
	if tenor == nil {
		return nil, common.ErrTenorNotFound
	}

	// 2. Hitung Sisa Limit
	limit, err := p.limitRepository.FindByCustomerIDAndTenorID(ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		return nil, common.ErrLimitNotSet
	}

	usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(
		ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}

	remainingLimit := limit.LimitAmount - usedAmount

	// 3. Buat Response
	if remainingLimit >= req.TransactionAmount {
		return &dto.CheckLimitResponse{
			Status:         "approved",
			Message:        "Limit is sufficient.",
			RemainingLimit: remainingLimit,
		}, nil
	}

	return &dto.CheckLimitResponse{
		Status:         "rejected",
		Message:        "Insufficient limit for this transaction.",
		RemainingLimit: remainingLimit,
	}, nil
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
) PartnerServices {
	return &partnerService{
		db:                    db,
		customerRepository:    customerRepository,
		tenorRepository:       tenorRepository,
		limitRepository:       limitRepository,
		transactionRepository: transactionRepository,

		meter:  meter,
		tracer: tracer,
		log:    log,
		// operationDuration: operationDuration,
		// operationCount:    operationCount,
		// errorCount:        errorCount,
		// profilesCreated:   profilesCreated,
		// profilesRetrieved: profilesRetrieved,
		// profilesUpdated:   profilesUpdated,
	}
}
