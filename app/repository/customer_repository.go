package repository

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type customerRepository struct {
	db *gorm.DB
}

// FindByNIKWithLock implements CustomerRepository.
func (c *customerRepository) FindByNIKWithLock(ctx context.Context, nik string) (*domain.Customer, error) {
	var customer model.Customer
	
	// Menggunakan Clauses(clause.Locking{Strength: "UPDATE"}) untuk SELECT ... FOR UPDATE
	err := c.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("nik = ?", nik).First(&customer).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return model.CustomerToEntity(customer), nil
}

// FindByID implements CustomerRepository.
func (c *customerRepository) FindByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	var customer model.Customer
	if err := c.db.WithContext(ctx).First(&customer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return model.CustomerToEntity(customer), nil
}

// FindByNIK implements CustomerRepository.
func (c *customerRepository) FindByNIK(ctx context.Context, nik string) (*domain.Customer, error) {
	var customer model.Customer

	if err := c.db.WithContext(ctx).Where("nik = ?", nik).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return model.CustomerToEntity(customer), nil
}

// FindPaginated implements CustomerRepository.
func (c *customerRepository) FindPaginated(ctx context.Context, params domain.Params) ([]domain.Customer, int64, error) {
	var customers []model.Customer
	var total int64

	query := c.db.WithContext(ctx).Model(&model.Customer{})
	countQuery := c.db.WithContext(ctx).Model(&model.Customer{})

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

	return model.CustomersToEntity(customers), total, nil
}

// Create implements CustomerRepository.
func (c *customerRepository) CreateCustomer(ctx context.Context, customer *domain.Customer) error {
	data := model.CustomerFromEntity(customer)
	return c.db.WithContext(ctx).Create(&data).Error
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepository{
		db: db,
	}
}
