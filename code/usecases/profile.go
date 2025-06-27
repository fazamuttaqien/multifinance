package usecases

import (
	"context"
	"fmt"
	"math"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"github.com/fazamuttaqien/xyz-multifinance/repositories"
	"gorm.io/gorm"
)

type profileUsecases struct {
	db                    *gorm.DB
	customerRepository    repositories.CustomerRepository
	limitRepository       repositories.LimitRepository
	tenorRepository       repositories.TenorRepository
	transactionRepository repositories.TransactionRepository
}

// GetMyLimits implements ProfileUsecases.
func (p *profileUsecases) GetMyLimits(ctx context.Context, customerID uint64) ([]dtos.LimitDetailResponse, error) {
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
	response := make([]dtos.LimitDetailResponse, 0, len(customerLimits))

	for _, limit := range customerLimits {
		// Hitung pemakaian tenor ini
		usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(ctx, customerID, limit.TenorID)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate used amount for tenor %d: %w", limit.TenorID, err)
		}

		detail := dtos.LimitDetailResponse{
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
func (p *profileUsecases) GetMyTransactions(ctx context.Context, customerID uint64, params dtos.PaginationParams) (*dtos.PaginatedResponse, error) {
	transactions, total, err := p.transactionRepository.FindPaginatedByCustomerID(ctx, customerID, params)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return &dtos.PaginatedResponse{
		Data:       transactions,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetProfile implements ProfileUsecases.
func (p *profileUsecases) GetProfile(ctx context.Context, customerID uint64) (*models.Customer, error) {
	customer, err := p.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, helper.ErrCustomerNotFound
	}

	return customer, nil
}

// UpdateProfile implements ProfileUsecases.
func (p *profileUsecases) UpdateProfile(ctx context.Context, customerID uint64, req dtos.UpdateProfileRequest) error {
	tx := p.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	customerTx := repositories.NewCustomerRepository(tx)
	customer, err := customerTx.FindByID(ctx, customerID)
	if err != nil {
		return err
	}
	if customer == nil {
		return helper.ErrCustomerNotFound
	}

	customer.FullName = req.FullName
	customer.Salary = req.Salary

	if err := customerTx.UpdateProfile(ctx, customer); err != nil {
		return err
	}

	return tx.Commit().Error
}

func NewProfileUsecases(
	db *gorm.DB,
	customerRepository repositories.CustomerRepository,
	limitRepository repositories.LimitRepository,
	tenorRepository repositories.TenorRepository,
	transactionRepository repositories.TransactionRepository,
) ProfileUsecases {
	return &profileUsecases{
		db:                    db,
		customerRepository:    customerRepository,
		limitRepository:       limitRepository,
		tenorRepository:       tenorRepository,
		transactionRepository: transactionRepository,
	}
}
