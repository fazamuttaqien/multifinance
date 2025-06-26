package repositories

import (
	"context"

	"github.com/fazamuttaqien/xyz-multifinance/models"
)

type CustomerRepository interface {
	Save(ctx context.Context, customer *models.Customer) error
	FindByNIK(ctx context.Context, nik string) (*models.Customer, error)
	FindByID(ctx context.Context, id uint64) (*models.Customer, error)
}

type TenorRepository interface {
	FindByDuration(ctx context.Context, durationMonths uint8) (*models.Tenor, error)
}

type LimitRepository interface {
	FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*models.CustomerLimit, error)
}

type TransactionRepository interface {
	SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error)
}
