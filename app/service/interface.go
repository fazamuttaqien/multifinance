package service

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
)

type Media interface {
	Upload(ctx context.Context, file *multipart.FileHeader) (string, error)
}

type ProfileServices interface {
	Create(ctx context.Context, req *domain.Customer) (*domain.Customer, error)
	Update(ctx context.Context, customerID uint64, req domain.Customer) error
	GetMyProfile(ctx context.Context, customerID uint64) (*domain.Customer, error)
	GetMyLimits(ctx context.Context, customerID uint64) ([]dto.LimitDetailResponse, error)
	GetMyTransactions(ctx context.Context, customerID uint64, params domain.Params) (*domain.Paginated, error)
}

type PartnerServices interface {
	CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error)
	CreateTransaction(ctx context.Context, req dto.CreateTransactionRequest) (*domain.Transaction, error)
}

type AdminServices interface {
	SetLimits(ctx context.Context, customerID uint64, req dto.SetLimits) error
	GetCustomerByID(ctx context.Context, customerID uint64) (*domain.Customer, error)
	ListCustomers(ctx context.Context, params domain.Params) (*domain.Paginated, error)
	VerifyCustomer(ctx context.Context, customerID uint64, req dto.VerificationRequest) error
}

type CloudinaryService interface {
	UploadImage(ctx context.Context, file *multipart.FileHeader, folder string) (string, error)
}
