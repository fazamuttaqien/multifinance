package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/model"
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
		return common.ErrCustomerNotFound
	}

	limitsToUpsert := make([]domain.CustomerLimit, 0, len(req.Limits))
	tenorTx := repository.NewTenorRepository(tx)

	// 2. Loop dan validasi setiap item limit dalam request
	for _, item := range req.Limits {
		if item.LimitAmount < 0 {
			return common.ErrInvalidLimitAmount
		}

		// Cari tenor ID berdasarkan durasi bulan
		tenor, err := tenorTx.FindByDuration(ctx, item.TenorMonths)
		log.Println("Tenor: ", tenor)
		if err != nil {
			return fmt.Errorf("error finding tenor for %d months: %w", item.TenorMonths, err)
		}
		if tenor == nil {
			return fmt.Errorf("%w: for %d months", common.ErrTenorNotFound, item.TenorMonths)
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

// GetCustomerByNIK implements AdminUsecases.
func (a *adminService) GetCustomerByNIK(ctx context.Context, customerID uint64) (*domain.Customer, error) {
	customer, err := a.customerRepository.FindByID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, common.ErrCustomerNotFound
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

	var customer model.Customer
	if err := tx.First(&customer, customerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.ErrCustomerNotFound
		}
		return err
	}

	// Validasi: hanya bisa verifikasi customer yang statusnya PENDING
	if customer.VerificationStatus != model.VerificationPending {
		return fmt.Errorf("customer is not in PENDING state, current state: %s", customer.VerificationStatus)
	}

	customer.VerificationStatus = model.VerificationStatus(req.Status)

	if err := tx.Model(&customer).Update("verification_status", req.Status).Error; err != nil {
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
