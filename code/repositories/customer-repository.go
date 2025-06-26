package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
)

type customerRepository struct {
	db *databases.Database
}

// FindByID implements CustomerRepository.
func (cr *customerRepository) FindByID(ctx context.Context, id uint64) (*models.Customer, error) {
	var customer models.Customer
	if err := cr.db.Gorm.WithContext(ctx).First(&customer, id).Error; err != nil {
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
	if err := cr.db.Gorm.WithContext(ctx).Where("nik = ?", nik).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &customer, nil
}

// SaveCustomer implements CustomerRepository.
func (cr *customerRepository) Save(ctx context.Context, customer *models.Customer) error {
	return cr.db.Gorm.WithContext(ctx).Create(customer).Error
}

func NewCustomerRepository(database *databases.Database) CustomerRepository {
	return &customerRepository{
		db: database,
	}
}
