package handler_test

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
)

type MockProfileService struct {
	MockRegisterResult          *domain.Customer
	MockGetMyProfileResult      *domain.Customer
	MockGetMyLimitsResult       []dto.LimitDetailResponse
	MockGetMyTransactionsResult *domain.Paginated
	MockError                   error
}

func (m *MockProfileService) Create(ctx context.Context, customer *domain.Customer) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockRegisterResult, nil
}

func (m *MockProfileService) GetMyProfile(ctx context.Context, id uint64) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyProfileResult, nil
}

func (m *MockProfileService) Update(ctx context.Context, id uint64, customer domain.Customer) error {
	return m.MockError
}

func (m *MockProfileService) GetMyLimits(ctx context.Context, id uint64) ([]dto.LimitDetailResponse, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyLimitsResult, nil
}

func (m *MockProfileService) GetMyTransactions(ctx context.Context, id uint64, params domain.Params) (*domain.Paginated, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetMyTransactionsResult, nil
}

type MockCloudinaryService struct {
	MockUploadURL   string
	MockUploadError error
}

func (m *MockCloudinaryService) UploadImage(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	if m.MockUploadError != nil {
		return "", m.MockUploadError
	}
	return m.MockUploadURL, nil
}

type MockAdminService struct {
	MockListCustomersResult   *domain.Paginated
	MockGetCustomerByIDResult *domain.Customer
	MockError                 error
}

func (m *MockAdminService) ListCustomers(ctx context.Context, params domain.Params) (*domain.Paginated, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockListCustomersResult, nil
}

func (m *MockAdminService) GetCustomerByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetCustomerByIDResult, nil
}

func (m *MockAdminService) VerifyCustomer(ctx context.Context, id uint64, req dto.VerificationRequest) error {
	return m.MockError
}

func (m *MockAdminService) SetLimits(ctx context.Context, id uint64, req dto.SetLimits) error {
	return m.MockError
}

type MockPartnerService struct {
	MockCheckLimitResult        *dto.CheckLimitResponse
	MockCreateTransactionResult *domain.Transaction
	MockError                   error
}

func (m *MockPartnerService) CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockCheckLimitResult, nil
}

func (m *MockPartnerService) CreateTransaction(ctx context.Context, req dto.CreateTransactionRequest) (*domain.Transaction, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockCreateTransactionResult, nil
}
