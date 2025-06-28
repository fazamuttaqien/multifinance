package service_test

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/multifinance/domain"
)

type mockCustomerRepository struct {
	// Fields to control mock behavior
	MockFindPaginatedData  []domain.Customer
	MockFindPaginatedTotal int64
	MockFindByIDData       *domain.Customer
	MockError              error

	// Fields to capture calls
	FindPaginatedCalledWith domain.Params
	FindByIDCalledWith      uint64
	UpdateCalledWith        *domain.Customer
}

func (m *mockCustomerRepository) FindPaginated(ctx context.Context, params domain.Params) ([]domain.Customer, int64, error) {
	m.FindPaginatedCalledWith = params
	return m.MockFindPaginatedData, m.MockFindPaginatedTotal, m.MockError
}

func (m *mockCustomerRepository) FindByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	m.FindByIDCalledWith = id
	if m.MockFindByIDData != nil && m.MockFindByIDData.ID == id {
		return m.MockFindByIDData, m.MockError
	}

	if m.MockError != nil {
		return nil, m.MockError
	}
	return nil, nil
}

func (m *mockCustomerRepository) Update(ctx context.Context, customer *domain.Customer) error {
	m.UpdateCalledWith = customer
	return m.MockError
}

func (m *mockCustomerRepository) FindByNIK(ctx context.Context, nik string, lock bool) (*domain.Customer, error) {
	return nil, nil
}

func (m *mockCustomerRepository) Save(ctx context.Context, customer *domain.Customer) error {
	return nil
}

type mockMediaRepository struct {
	MockUploadImageURL   string
	MockUploadImageError error

	UploadImageCalledWith *multipart.FileHeader
}

func (m *mockMediaRepository) UploadImage(ctx context.Context, file *multipart.FileHeader) (string, error) {
	m.UploadImageCalledWith = file
	return m.MockUploadImageURL, m.MockUploadImageError
}

type mockTenorRepository struct {
	MockFindAllData        []domain.Tenor
	MockFindByDurationData *domain.Tenor
	MockError              error
}

func (m *mockTenorRepository) FindAll(ctx context.Context) ([]domain.Tenor, error) {
	return m.MockFindAllData, m.MockError
}
func (m *mockTenorRepository) FindByDuration(ctx context.Context, duration uint8) (*domain.Tenor, error) {
	return m.MockFindByDurationData, m.MockError
}

type mockLimitRepository struct {
	MockFindAllByCustomerIDData []domain.CustomerLimit
	MockError                   error
}

func (m *mockLimitRepository) FindAllByCustomerID(ctx context.Context, customerID uint64) ([]domain.CustomerLimit, error) {
	return m.MockFindAllByCustomerIDData, m.MockError
}

// Metode lain (tidak digunakan dalam tes ini)
func (m *mockLimitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*domain.CustomerLimit, error) {
	return nil, nil
}
func (m *mockLimitRepository) UpsertMany(ctx context.Context, limits []domain.CustomerLimit) error {
	return nil
}

// mockTransactionRepository
type mockTransactionRepository struct {
	MockSumActiveData             float64
	MockFindPaginatedData         []domain.Transaction
	MockFindPaginatedTotal        int64
	MockError                     error
	SumActiveCalledWithCustomerID uint64
	SumActiveCalledWithTenorID    uint
}

func (m *mockTransactionRepository) SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error) {
	m.SumActiveCalledWithCustomerID = customerID
	m.SumActiveCalledWithTenorID = tenorID
	return m.MockSumActiveData, m.MockError
}
func (m *mockTransactionRepository) FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params domain.Params) ([]domain.Transaction, int64, error) {
	return m.MockFindPaginatedData, m.MockFindPaginatedTotal, m.MockError
}
