package service

import (
	"context"
	"fmt"
	"math"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	helper_error "github.com/fazamuttaqien/multifinance/helper/error"
	"github.com/fazamuttaqien/multifinance/repository"
	"gorm.io/gorm"
)

type adminService struct {
	db                 *gorm.DB
	customerRepository repository.CustomerRepository
}

// SetLimits implements AdminUsecases.
func (a *adminService) SetLimits(ctx context.Context, customerID uint64, req dto.SetLimits) error {
	// Start transaction
	tx := a.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	// 1. Validasi customer
	customerTx := repository.NewCustomerRepository(tx)
	customer, err := customerTx.FindByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("error finding customer: %w", err)
	}
	if customer == nil {
		return helper_error.ErrCustomerNotFound
	}

	limitsToUpsert := make([]domain.CustomerLimit, 0, len(req.Limits))
	tenorTx := repository.NewTenorRepository(tx)

	// 2. Loop dan validasi setiap item limit dalam request
	for _, item := range req.Limits {
		if item.LimitAmount < 0 {
			return helper_error.ErrInvalidLimitAmount
		}

		// Cari tenor ID berdasarkan durasi bulan
		tenor, err := tenorTx.FindByDuration(ctx, item.TenorMonths)
		if err != nil {
			return fmt.Errorf("error finding tenor for %d months: %w", item.TenorMonths, err)
		}
		if tenor == nil {
			return fmt.Errorf("%w: for %d months", helper_error.ErrTenorNotFound, item.TenorMonths)
		}

		// Menyiapkan data untuk di upsert
		limitsToUpsert = append(limitsToUpsert, domain.CustomerLimit{
			CustomerID:  customerID,
			TenorID:     tenor.ID,
			LimitAmount: item.LimitAmount,
		})
	}

	// 3. Melakukan operasi upsert massal
	if len(limitsToUpsert) > 0 {
		limitTx := repository.NewLimitRepository(tx)
		if err := limitTx.UpsertMany(ctx, limitsToUpsert); err != nil {
			return fmt.Errorf("failed to upsert limits: %w", err)
		}
	}

	// 4. Jika semua berhasil, commit transaksi
	return tx.Commit().Error
}

// GetProfile implements AdminUsecases.
func (a *adminService) GetProfile(ctx context.Context, customerID uint64) (*domain.Customer, error) {
	customer, err := a.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, helper_error.ErrCustomerNotFound
	}

	return customer, nil
}

// ListCustomers implements AdminUsecases.
func (a *adminService) ListCustomers(ctx context.Context, params domain.Params) (*domain.Paginated, error) {
	customers, total, err := a.customerRepository.FindPaginated(ctx, params)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return &domain.Paginated{
		Data:       customers,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// VerifyCustomer implements AdminUsecases.
func (a *adminService) VerifyCustomer(ctx context.Context, customerID uint64, req dto.Verification) error {
	tx := a.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	customerTx := repository.NewCustomerRepository(tx)
	customer, err := customerTx.FindByID(ctx, customerID)
	if err != nil {
		return err
	}
	if customer == nil {
		return helper_error.ErrCustomerNotFound
	}

	// Validasi: hanya bisa verifikasi customer yang statusnya PENDING
	if customer.VerificationStatus != domain.VerificationPending {
		return fmt.Errorf("customer is not in PENDING state, current state: %s", customer.VerificationStatus)
	}

	customer.VerificationStatus = req.Status
	// Di sini bisa ditambahkan logika untuk menyimpan `req.Reason` jika diperlukan

	if err := customerTx.Save(ctx, customer); err != nil {
		return err
	}

	return tx.Commit().Error
}

func NewAdminService(
	db *gorm.DB,
	customerRepository repository.CustomerRepository,
) AdminServices {
	return &adminService{
		db:                 db,
		customerRepository: customerRepository,
	}
}
