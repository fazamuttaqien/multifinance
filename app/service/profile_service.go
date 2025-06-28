package service

import (
	"context"
	"errors"
	"fmt"
	"math"

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

// CreateProfile implements ProfileUsecases.
func (p *profileService) CreateProfile(ctx context.Context, req *domain.Customer) (*domain.Customer, error) {
	// 1. Cek duplikasi NIK
	existingCustomer, err := p.customerRepository.FindByNIK(ctx, req.NIK)
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
	if err := p.customerRepository.CreateCustomer(ctx, newCustomer); err != nil {
		return nil, err
	}

	return newCustomer, nil
}

// GetMyLimits implements ProfileUsecases.
func (p *profileService) GetMyLimits(ctx context.Context, customerID uint64) ([]dto.LimitDetailResponse, error) {
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
	response := make([]dto.LimitDetailResponse, 0, len(customerLimits))

	for _, limit := range customerLimits {
		// Hitung pemakaian tenor ini
		usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(ctx, customerID, limit.TenorID)
		if err != nil {
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

// GetMyProfile implements ProfileUsecases.
func (p *profileService) GetMyProfile(ctx context.Context, customerID uint64) (*domain.Customer, error) {
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
