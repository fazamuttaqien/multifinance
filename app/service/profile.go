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
)

type profileService struct {
	db                    *gorm.DB
	customerRepository    repository.CustomerRepository
	limitRepository       repository.LimitRepository
	tenorRepository       repository.TenorRepository
	transactionRepository repository.TransactionRepository
}

// CreateTransaction implements ProfileUsecases.
func (p *profileService) CreateTransaction(ctx context.Context, req dto.Transaction) (*domain.Transaction, error) {
	// Start Transaction
	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	// 1. Mendapatkan Customer berdasarkan NIK dan KUNCI barisnya untuk mencegah race condition
	customerTx := repository.NewCustomerRepository(tx)
	lockedCustomer, err := customerTx.FindByNIK(ctx, req.NIK, true)
	if err != nil {
		return nil, fmt.Errorf("error finding customer: %w", err)
	}
	if lockedCustomer == nil {
		return nil, common.ErrCustomerNotFound
	}

	// Memastikan costumer sudah terverifikasi
	if lockedCustomer.VerificationStatus != domain.VerificationVerified {
		return nil, fmt.Errorf("customer with NIK %s is not verified", req.NIK)
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

	// 5. Generate Nomor Kontrak
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
		Status:                 domain.TransactionActive, // Langsung aktif
	}

	// 7. Simpan transaksi baru ke DB
	repoTx := repository.NewTransactionRepository(tx)
	if err := repoTx.CreateTransaction(ctx, newTransaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction record: %w", err)
	}

	// 8. Jika semua berhasil, commit transaksi
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &newTransaction, nil
}

// Register implements ProfileUsecases.
func (p *profileService) Register(ctx context.Context, req *domain.Customer) (*domain.Customer, error) {
	// 1. Cek duplikasi NIK
	existingCustomer, err := p.customerRepository.FindByNIK(ctx, req.NIK, false)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existingCustomer != nil {
		return nil, errors.New("customer already registered")
	}

	// 4. Buat entitas customer baru
	newCustomer := &domain.Customer{
		NIK:                req.NIK,
		FullName:           req.FullName,
		LegalName:          req.LegalName,
		BirthPlace:         req.BirthPlace,
		BirthDate:          req.BirthDate,
		Salary:             req.Salary,
		KtpUrl:             req.KtpUrl,
		SelfieUrl:          req.SelfieUrl,
		VerificationStatus: domain.VerificationPending,
	}

	// 5. Simpan ke database
	if err := p.customerRepository.Save(ctx, newCustomer); err != nil {
		return nil, err
	}

	return newCustomer, nil
}

// GetMyLimits implements ProfileUsecases.
func (p *profileService) GetMyLimits(ctx context.Context, customerID uint64) ([]dto.LimitDetail, error) {
	// 1. Ambil semua limit yang ditetapkan untuk customer
	customerLimits, err := p.limitRepository.FindAllByCustomerID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	// 2. Ambil semua data tenor untuk mapping ID ke durasi bulan
	allTenors, err := p.tenorRepository.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	tenorMap := make(map[uint]uint8)
	for _, tenor := range allTenors {
		tenorMap[tenor.ID] = tenor.DurationMonths
	}

	// 3. Menyiapkan response
	response := make([]dto.LimitDetail, 0, len(customerLimits))

	for _, limit := range customerLimits {
		// Hitung pemakaian tenor ini
		usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(ctx, customerID, limit.TenorID)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate used amount for tenor %d: %w", limit.TenorID, err)
		}

		detail := dto.LimitDetail{
			TenorMonths:    tenorMap[limit.TenorID],
			LimitAmount:    limit.LimitAmount,
			UsedAmount:     usedAmount,
			RemainingLimit: limit.LimitAmount - usedAmount,
		}
		response = append(response, detail)
	}

	return response, nil
}

// GetMyTransactions implements ProfileUsecases.
func (p *profileService) GetMyTransactions(ctx context.Context, customerID uint64, params domain.Params) (*domain.Paginated, error) {
	transactions, total, err := p.transactionRepository.FindPaginatedByCustomerID(ctx, customerID, params)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return &domain.Paginated{
		Data:       transactions,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetProfile implements ProfileUsecases.
func (p *profileService) GetProfile(ctx context.Context, customerID uint64) (*domain.Customer, error) {
	customer, err := p.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, common.ErrCustomerNotFound
	}

	return customer, nil
}

// UpdateProfile implements ProfileUsecases.
func (p *profileService) UpdateProfile(ctx context.Context, customerID uint64, req domain.Customer) error {
	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	var customer model.Customer
	if err := tx.First(&customer, customerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrCustomerNotFound
		}
		return err
	}

	updates := map[string]any{
		"full_name": req.FullName,
		"salary":    req.Salary,
	}

	customer.FullName = req.FullName
	customer.Salary = req.Salary

	if err := tx.Model(&customer).Updates(updates).Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func NewProfileService(
	db *gorm.DB,
	customerRepository repository.CustomerRepository,
	limitRepository repository.LimitRepository,
	tenorRepository repository.TenorRepository,
	transactionRepository repository.TransactionRepository,
) ProfileServices {
	return &profileService{
		db:                    db,
		customerRepository:    customerRepository,
		limitRepository:       limitRepository,
		tenorRepository:       tenorRepository,
		transactionRepository: transactionRepository,
	}
}
