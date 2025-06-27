package repositories

import (
	"context"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/models"
)

type CustomerRepository interface {
	Save(ctx context.Context, customer *models.Customer) error
	FindByNIK(ctx context.Context, nik string) (*models.Customer, error)
	FindByNIKWithLock(ctx context.Context, nik string) (*models.Customer, error)
	FindByID(ctx context.Context, id uint64) (*models.Customer, error)
	FindPaginated(ctx context.Context, params dtos.CustomerQueryParams) ([]models.Customer, int64, error)

	UpdateProfile(ctx context.Context, customer *models.Customer) error
}

type TenorRepository interface {
	FindByDuration(ctx context.Context, durationMonths uint8) (*models.Tenor, error)
	FindAll(ctx context.Context) ([]models.Tenor, error)
}

type LimitRepository interface {
	FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*models.CustomerLimit, error)
	UpsertMany(ctx context.Context, limits []models.CustomerLimit) error
	FindAllByCustomerID(ctx context.Context, customerID uint64) ([]models.CustomerLimit, error)
}

type TransactionRepository interface {
	SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error)
	CreateTransaction(ctx context.Context, tx models.Transaction) error
	FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params dtos.PaginationParams) ([]models.Transaction, int64, error)
}
