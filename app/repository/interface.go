package repository

import (
	"context"

	"github.com/fazamuttaqien/multifinance/domain"
)

type CustomerRepository interface {
	CreateCustomer(ctx context.Context, customer domain.Customer) error
	FindByNIK(ctx context.Context, nik string) (*domain.Customer, error)
	FindByNIKWithLock(ctx context.Context, nik string) (*domain.Customer, error)
	FindByID(ctx context.Context, id uint64) (*domain.Customer, error)
	FindPaginated(ctx context.Context, params domain.Params) ([]domain.Customer, int64, error)
}

type TenorRepository interface {
	FindByDuration(ctx context.Context, durationMonths uint8) (*domain.Tenor, error)
	FindAll(ctx context.Context) ([]domain.Tenor, error)
}

type LimitRepository interface {
	FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*domain.CustomerLimit, error)
	UpsertMany(ctx context.Context, limits []domain.CustomerLimit) error
	FindAllByCustomerID(ctx context.Context, customerID uint64) ([]domain.CustomerLimit, error)
}

type TransactionRepository interface {
	SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error)
	CreateTransaction(ctx context.Context, tx *domain.Transaction) error
	FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params domain.Params) ([]domain.Transaction, int64, error)
}
