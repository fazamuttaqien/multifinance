package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/repository"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

// UNIT TESTS
func TestRegister(t *testing.T) {
	// Arrange
	mockCustomerRepository := &MockCustomerRepository{}
	mockMediaRepository := &MockMediaRepository{}

	service := service.NewProfileService(
		nil,
		mockCustomerRepository,
		nil, nil, nil,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	birthDate, _ := time.Parse("2006-01-02", "2000-01-01")
	req := domain.Customer{
		NIK: "1234567890123456", FullName: "Test User", BirthDate: birthDate,
	}

	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		mockCustomerRepository.MockFindByIDData = nil // NIK tidak ditemukan
		mockMediaRepository.MockUploadImageURL = "http://cloudinary.com/image.jpg"

		// Act
		customer, err := service.Create(context.Background(), &req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, customer)
		assert.Equal(t, req.NIK, customer.NIK)
		assert.Equal(t, domain.VerificationPending, customer.VerificationStatus)
		assert.NotNil(t, mockCustomerRepository.CreateCalledWith)
	})

	t.Run("NIK Exists", func(t *testing.T) {
		// Konfigurasi mock
		mockCustomerRepository.MockFindByNIKData = &domain.Customer{} // NIK ditemukan

		// Act
		_, err := service.Create(context.Background(), &req)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, common.ErrNIKExists)
	})

	t.Run("Upload KTP Fails", func(t *testing.T) {
		// Konfigurasi mock
		mockCustomerRepository.MockFindByNIKData = nil
		mockMediaRepository.MockUploadImageError = errors.New("upload failed")

		// Act
		_, err := service.Create(context.Background(), &req)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to upload")
	})
}

func TestGetMyLimits(t *testing.T) {
	// Arrange
	mockLimitRepository := &MockLimitRepository{}
	mockTenorRepository := &MockTenorRepository{}
	mockTxnRepository := &MockTransactionRepository{}

	service := service.NewProfileService(
		nil, nil,
		mockLimitRepository,
		mockTenorRepository,
		mockTxnRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	t.Run("Success with calculated remaining limit", func(t *testing.T) {
		// Konfigurasi mock
		customerID := uint64(10)
		mockLimitRepository.MockFindAllByCustomerIDData = []domain.CustomerLimit{
			{CustomerID: customerID, TenorID: 1, LimitAmount: 1000},
			{CustomerID: customerID, TenorID: 2, LimitAmount: 5000},
		}
		mockTenorRepository.MockFindAllData = []domain.Tenor{
			{ID: 1, DurationMonths: 3},
			{ID: 2, DurationMonths: 6},
		}
		mockTxnRepository.MockSumActiveData = 250.0

		// Act
		limits, err := service.GetMyLimits(context.Background(), customerID)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, limits, 2)
		// Cek item pertama
		assert.Equal(t, uint8(3), limits[0].TenorMonths)
		assert.Equal(t, float64(1000), limits[0].LimitAmount)
		assert.Equal(t, float64(250), limits[0].UsedAmount)
		assert.Equal(t, float64(750), limits[0].RemainingLimit) // 1000 - 250
		// Cek item kedua
		assert.Equal(t, uint8(6), limits[1].TenorMonths)
		assert.Equal(t, float64(5000), limits[1].LimitAmount)
		assert.Equal(t, float64(4750), limits[1].RemainingLimit) // 5000 - 250
	})
}

func TestGetMyTransactions(t *testing.T) {
	// Arrange
	mockTxnRepository := &MockTransactionRepository{}
	service := service.NewProfileService(
		nil, nil, nil, nil,
		mockTxnRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	t.Run("Success with pagination", func(t *testing.T) {
		// Konfigurasi mock
		mockTxnRepository.MockFindPaginatedData = []domain.Transaction{{ID: 1, AssetName: "Laptop"}}
		mockTxnRepository.MockFindPaginatedTotal = 11 // Total ada 11 data

		params := domain.Params{Page: 2, Limit: 5}

		// Act
		result, err := service.GetMyTransactions(context.Background(), 10, params)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(11), result.Total)
		assert.Equal(t, 2, result.Page)
		assert.Equal(t, 5, result.Limit)
		assert.Equal(t, 3, result.TotalPages) // math.Ceil(11 / 5) = 3
	})
}

func TestUpdateProfile(t *testing.T) {
	// Arrange
	db := SetupTestDB(t)

	customerRepository := repository.NewCustomerRepository(
		db,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	service := service.NewProfileService(
		db,
		customerRepository,
		nil, nil, nil,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// Buat data customer untuk diupdate
	testCustomer := &domain.Customer{
		ID:        10,
		NIK:       "333",
		FullName:  "Old Name",
		Salary:    5000,
		BirthDate: time.Now(),
	}
	db.Create(testCustomer)

	t.Run("Success updating profile", func(t *testing.T) {
		req := domain.Customer{
			FullName: "New Name",
			Salary:   10000,
		}

		// Act
		err := service.Update(context.Background(), 10, req)

		// Assert
		assert.NoError(t, err)

		// Cek langsung ke DB in-memory
		var updatedCustomer domain.Customer
		db.First(&updatedCustomer, 10)
		assert.Equal(t, "New Name", updatedCustomer.FullName)
		assert.Equal(t, float64(10000), updatedCustomer.Salary)
	})

	t.Run("Fail updating non-existent customer", func(t *testing.T) {
		req := domain.Customer{FullName: "New Name", Salary: 10000}

		// Act
		err := service.Update(context.Background(), 99, req) // ID 99 tidak ada

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}
