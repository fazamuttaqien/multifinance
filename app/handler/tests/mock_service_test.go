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
	MockGetMyLimitsResult       []dto.LimitDetailResponse
	MockGetMyTransactionsResult *domain.Paginated
	MockError                   error
}

func (m *mockProfileService) Create(ctx context.Context, customer *domain.Customer) (*domain.Customer, error) {
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

func (m *mockProfileService) Update(ctx context.Context, id uint64, customer domain.Customer) error {
	return m.MockError
}

func (m *mockProfileService) GetMyLimits(ctx context.Context, id uint64) ([]dto.LimitDetailResponse, error) {
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

type mockAdminService struct {
	MockListCustomersResult   *domain.Paginated
	MockGetCustomerByIDResult *domain.Customer
	MockError                 error
}

func (m *mockAdminService) ListCustomers(ctx context.Context, params domain.Params) (*domain.Paginated, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockListCustomersResult, nil
}

func (m *mockAdminService) GetCustomerByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockGetCustomerByIDResult, nil
}

func (m *mockAdminService) VerifyCustomer(ctx context.Context, id uint64, req dto.VerificationRequest) error {
	return m.MockError
}

func (m *mockAdminService) SetLimits(ctx context.Context, id uint64, req dto.SetLimits) error {
	return m.MockError
}

type mockPartnerService struct {
	MockCheckLimitResult        *dto.CheckLimitResponse
	MockCreateTransactionResult *domain.Transaction
	MockError                   error
}

func (m *mockPartnerService) CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockCheckLimitResult, nil
}

func (m *mockPartnerService) CreateTransaction(ctx context.Context, req dto.CreateTransactionRequest) (*domain.Transaction, error) {
	if m.MockError != nil {
		return nil, m.MockError
	}
	return m.MockCreateTransactionResult, nil
}
