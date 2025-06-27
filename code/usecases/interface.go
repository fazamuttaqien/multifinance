package usecases

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/models"
)

type Usecases interface {
	Register(ctx context.Context, req *dtos.CustomerRegister) (*models.Customer, error)
	CalculateLimit(ctx context.Context, customerID uint64, tenorMonths uint8) (*dtos.LimitDetailResponse, error)
	SetLimits(ctx context.Context, customerID uint64, req dtos.SetLimitsRequest) error
	CreateTransaction(ctx context.Context, req dtos.CreateTransactionRequest) (*models.Transaction, error)
}

type Media interface {
	Upload(ctx context.Context, file *multipart.FileHeader) (string, error)
}

type ProfileUsecases interface {
	GetProfile(ctx context.Context, customerID uint64) (*models.Customer, error)
	UpdateProfile(ctx context.Context, customerID uint64, req dtos.UpdateProfileRequest) error
	GetMyLimits(ctx context.Context, customerID uint64) ([]dtos.LimitDetailResponse, error)
	GetMyTransactions(ctx context.Context, customerID uint64, params dtos.PaginationParams) (*dtos.PaginatedResponse, error)
}

type PartnerUsecases interface {
	CheckLimit(ctx context.Context, req dtos.CheckLimitRequest) (*dtos.CheckLimitResponse, error)
}

type AdminUsecases interface {
	GetProfile(ctx context.Context, customerID uint64) (*models.Customer, error)
	ListCustomers(ctx context.Context, params dtos.CustomerQueryParams) (*dtos.PaginatedResponse, error)
	VerifyCustomer(ctx context.Context, customerID uint64, req dtos.VerificationRequest) error
}
