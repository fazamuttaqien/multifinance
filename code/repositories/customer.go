package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type customerRepository struct {
	db *gorm.DB
}

// FindPaginated implements CustomerRepository.
func (cr *customerRepository) FindPaginated(ctx context.Context, params dtos.CustomerQueryParams) ([]models.Customer, int64, error) {
	var customers []models.Customer
	var total int64

	query := cr.db.WithContext(ctx).Model(&models.Customer{})
	countQuery := cr.db.WithContext(ctx).Model(&models.Customer{})

	// Filter berdasarkan status
	if params.Status != "" {
		query = query.Where("verification_status = ?", params.Status)
		countQuery = countQuery.Where("verification_status = ?", params.Status)
	}

	// Hitung total sebelum paginasi
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Terapkan paginasi
	offset := (params.Page - 1) * params.Limit
	query = query.Limit(params.Limit).Offset(offset).Order("created_at DESC")

	if err := query.Find(&customers).Error; err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

// UpdateProfile implements CustomerRepository.
func (cr *customerRepository) UpdateProfile(ctx context.Context, customer *models.Customer) error {
	return cr.db.WithContext(ctx).Save(customer).Error
}

// FindByNIKWithLock implements CustomerRepository.
func (cr *customerRepository) FindByNIKWithLock(ctx context.Context, nik string) (*models.Customer, error) {
	var customer models.Customer
	// Menggunakan Clauses(clause.Locking{Strength: "UPDATE"}) untuk SELECT ... FOR UPDATE
	err := cr.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("nik = ?", nik).First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &customer, nil
}

// FindByID implements CustomerRepository.
func (cr *customerRepository) FindByID(ctx context.Context, id uint64) (*models.Customer, error) {
	var customer models.Customer
	if err := cr.db.WithContext(ctx).First(&customer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &customer, nil
}

// FindByNIK implements CustomerRepository.
func (cr *customerRepository) FindByNIK(ctx context.Context, nik string) (*models.Customer, error) {
	var customer models.Customer
	if err := cr.db.WithContext(ctx).Where("nik = ?", nik).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &customer, nil
}

// SaveCustomer implements CustomerRepository.
func (cr *customerRepository) Save(ctx context.Context, customer *models.Customer) error {
	return cr.db.WithContext(ctx).Create(customer).Error
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepository{
		db: db,
	}
}
