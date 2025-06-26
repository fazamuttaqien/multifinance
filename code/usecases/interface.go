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
}

type Media interface {
	Upload(ctx context.Context, file *multipart.FileHeader) (string, error)
}
