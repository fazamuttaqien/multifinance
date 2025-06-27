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

type adminUsecase struct {
	db                 *gorm.DB
	customerRepository repositories.CustomerRepository
}

// ListCustomers implements AdminUsecases.
func (s *adminUsecase) ListCustomers(ctx context.Context, params dtos.CustomerQueryParams) (*dtos.PaginatedResponse, error) {
	customers, total, err := s.customerRepository.FindPaginated(ctx, params)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return &dtos.PaginatedResponse{
		Data:       customers,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// VerifyCustomer implements AdminUsecases.
func (s *adminUsecase) VerifyCustomer(ctx context.Context, customerID uint64, req dtos.VerificationRequest) error {
	tx := s.db.WithContext(ctx).Begin()
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

	// Validasi: hanya bisa verifikasi customer yang statusnya PENDING
	if customer.VerificationStatus != models.VerificationPending {
		return fmt.Errorf("customer is not in PENDING state, current state: %s", customer.VerificationStatus)
	}

	customer.VerificationStatus = req.Status
	// Di sini bisa ditambahkan logika untuk menyimpan `req.Reason` jika diperlukan

	if err := customerTx.UpdateProfile(ctx, customer); err != nil {
		return err
	}

	return tx.Commit().Error
}

func (p *adminUsecase) GetProfile(ctx context.Context, customerID uint64) (*models.Customer, error) {
	customer, err := p.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, helper.ErrCustomerNotFound
	}

	return customer, nil
}

func NewAdminUsecases(
	db *gorm.DB,
	cr repositories.CustomerRepository,
) AdminUsecases {
	return &adminUsecase{
		db:                 db,
		customerRepository: cr,
	}
}
