package handler_test

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
)

type mockProfileService struct {
	MockRegisterResult          *domain.Customer
	MockGetMyProfileResult      *domain.Customer
	MockGetMyLimitsResult       []dto.LimitDetail
	MockGetMyTransactionsResult *domain.Paginated
	MockError                   error
}

func (m *mockProfileService) Register(ctx context.Context, customer *domain.Customer) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockRegisterResult, nil
}

func (m *mockProfileService) GetMyProfile(ctx context.Context, id uint64) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyProfileResult, nil
}

func (m *mockProfileService) UpdateProfile(ctx context.Context, id uint64, customer domain.Customer) error {
	return m.MockError
}

func (m *mockProfileService) GetMyLimits(ctx context.Context, id uint64) ([]dto.LimitDetail, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyLimitsResult, nil
}

func (m *mockProfileService) GetMyTransactions(ctx context.Context, id uint64, params domain.Params) (*domain.Paginated, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyTransactionsResult, nil
}

type mockCloudinaryService struct {
	MockUploadURL   string
	MockUploadError error
}

func (m *mockCloudinaryService) UploadImage(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	if m.MockUploadError != nil {
		return "", m.MockUploadError
	}
	return m.MockUploadURL, nil
}

