package service_test

import (
	"context"
	"mime/multipart"

	"github.com/fazamuttaqien/multifinance/domain"
)

// Mock Customer Repository
type MockCustomerRepository struct {
	// Fields to control mock behavior
	MockFindPaginatedData     []domain.Customer
	MockFindPaginatedTotal    int64
	MockFindByIDData          *domain.Customer
	MockFindByNIKData         *domain.Customer
	MockFindByNIKWithLockData *domain.Customer
	MockError                 error

	// Fields to capture calls
	FindPaginatedCalledWith     domain.Params
	FindByIDCalledWith          uint64
	FindByNIKCalledWith         string
	FindByNIKWithLockCalledWith string
	UpdateCalledWith            *domain.Customer
	CreateCalledWith            *domain.Customer
}

func NewMockCustomerRepository() *MockCustomerRepository {
	return &MockCustomerRepository{}
}

func (m *MockCustomerRepository) FindPaginated(ctx context.Context, params domain.Params) ([]domain.Customer, int64, error) {
	m.FindPaginatedCalledWith = params
	return m.MockFindPaginatedData, m.MockFindPaginatedTotal, m.MockError
}

func (m *MockCustomerRepository) FindByID(ctx context.Context, id uint64) (*domain.Customer, error) {
	// m.FindByIDCalledWith = id
	// if m.MockFindByIDData != nil && m.MockFindByIDData.ID == id {
	// 	return m.MockFindByIDData, m.MockError
	// }

	// if m.MockError != nil {
	// 	return nil, m.MockError
	// }
	// return nil, nil

	m.FindByIDCalledWith = id
	return m.MockFindByIDData, m.MockError
}

func (m *MockCustomerRepository) CreateCustomer(ctx context.Context, customer *domain.Customer) error {
	m.CreateCalledWith = customer
	return m.MockError
}

// func (m *MockCustomerRepository) Update(ctx context.Context, customer *domain.Customer) error {
// 	m.UpdateCalledWith = customer
// 	return m.MockError
// }

func (m *MockCustomerRepository) FindByNIK(ctx context.Context, nik string) (*domain.Customer, error) {
	return nil, nil
}

func (m *MockCustomerRepository) FindByNIKWithLock(ctx context.Context, nik string) (*domain.Customer, error) {
	m.FindByNIKWithLockCalledWith = nik
	return m.MockFindByNIKWithLockData, m.MockError
}

// Mock Media Repository
type MockMediaRepository struct {
	MockUploadImageURL   string
	MockUploadImageError error

	UploadImageCalledWith *multipart.FileHeader
}

func NewMockMediaRepository() *MockMediaRepository {
	return &MockMediaRepository{}
}

func (m *MockMediaRepository) UploadImage(ctx context.Context, file *multipart.FileHeader) (string, error) {
	m.UploadImageCalledWith = file
	return m.MockUploadImageURL, m.MockUploadImageError
}

// Mock Tenor Repository
type MockTenorRepository struct {
	MockFindAllData        []domain.Tenor
	MockFindByDurationData *domain.Tenor
	MockError              error
}

func NewMockTenorRepository() *MockTenorRepository {
	return &MockTenorRepository{}
}

func (m *MockTenorRepository) FindAll(ctx context.Context) ([]domain.Tenor, error) {
	return m.MockFindAllData, m.MockError
}
func (m *MockTenorRepository) FindByDuration(ctx context.Context, duration uint8) (*domain.Tenor, error) {
	return m.MockFindByDurationData, m.MockError
}

// Mock Limit Repository
type MockLimitRepository struct {
	MockFindAllByCustomerIDData []domain.CustomerLimit
	MockFindByCIDAndTIDData     *domain.CustomerLimit
	MockError                   error
}

func NewMockLimitRepository() *MockLimitRepository {
	return &MockLimitRepository{}
}

func (m *MockLimitRepository) FindAllByCustomerID(ctx context.Context, customerID uint64) ([]domain.CustomerLimit, error) {
	return m.MockFindAllByCustomerIDData, m.MockError
}

func (m *MockLimitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*domain.CustomerLimit, error) {
	return m.MockFindByCIDAndTIDData, m.MockError
}

func (m *MockLimitRepository) UpsertMany(ctx context.Context, limits []domain.CustomerLimit) error {
	return m.MockError
}

// Mock Transaction Repository
type MockTransactionRepository struct {
	MockSumActiveData             float64
	MockFindPaginatedData         []domain.Transaction
	MockFindPaginatedTotal        int64
	MockError                     error

	SumActiveCalledWithCustomerID uint64
	SumActiveCalledWithTenorID    uint

	CreateCalledWith *domain.Transaction
}

func NewMockTransactionRepository() *MockTransactionRepository {
	return &MockTransactionRepository{}
}

func (m *MockTransactionRepository) SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error) {
	m.SumActiveCalledWithCustomerID = customerID
	m.SumActiveCalledWithTenorID = tenorID
	return m.MockSumActiveData, m.MockError
}

func (m *MockTransactionRepository) FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params domain.Params) ([]domain.Transaction, int64, error) {
	return m.MockFindPaginatedData, m.MockFindPaginatedTotal, m.MockError
}

func (m *MockTransactionRepository) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	m.CreateCalledWith = tx
	return m.MockError
}