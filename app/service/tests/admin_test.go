package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/repository"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

// UNIT TESTS
func TestListCustomers(t *testing.T) {
	// Arrange
	mockRepository := &MockCustomerRepository{}
	// Kita set db dan repo lain ke nil karena service ini tidak menggunakannya
	service := service.NewAdminService(
		nil,
		mockRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// Skenario 1: Sukses mendapatkan data
	t.Run("Success", func(t *testing.T) {
		// Konfigurasi mock
		expectedCustomers := []domain.Customer{{ID: 2, FullName: "Budi"}}
		mockRepository.MockFindPaginatedData = expectedCustomers
		mockRepository.MockFindPaginatedTotal = 1
		mockRepository.MockError = nil

		params := domain.Params{Status: "PENDING", Page: 1, Limit: 10}

		// Act
		result, err := service.ListCustomers(context.Background(), params)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(1), result.Total)
		assert.Equal(t, 1, result.TotalPages)
		assert.Equal(t, params, mockRepository.FindPaginatedCalledWith)
		data, ok := result.Data.([]domain.Customer)
		assert.True(t, ok)
		assert.Equal(t, "Budi", data[0].FullName)
	})

	// Skenario 2: Error dari repository
	t.Run("Repository Error", func(t *testing.T) {
		// Konfigurasi mock
		mockRepository.MockFindPaginatedData = nil
		mockRepository.MockFindPaginatedTotal = 0
		mockRepository.MockError = errors.New("database connection lost")

		params := domain.Params{}

		// Act
		result, err := service.ListCustomers(context.Background(), params)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "database connection lost", err.Error())
	})
}

func TestGetCustomerByID_Admin(t *testing.T) {
	// Arrange
	mockRepository := &MockCustomerRepository{}
	service := service.NewAdminService(
		nil,
		mockRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// Skenario 1: Customer ditemukan
	t.Run("Customer Found", func(t *testing.T) {
		// Konfigurasi mock
		expectedCustomer := &domain.Customer{ID: 5, FullName: "Annisa"}
		mockRepository.MockFindByIDData = expectedCustomer
		mockRepository.MockError = nil

		// Act
		// Service GetProfile digunakan oleh handler GetCustomerByID
		result, err := service.GetCustomerByID(context.Background(), 5)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint64(5), mockRepository.FindByIDCalledWith)
		assert.Equal(t, "Annisa", result.FullName)
	})

	// Skenario 2: Customer tidak ditemukan
	t.Run("Customer Not Found", func(t *testing.T) {
		// Konfigurasi mock
		mockRepository.MockFindByIDData = nil
		mockRepository.MockError = nil

		// Act
		result, err := service.GetCustomerByID(context.Background(), 99)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}

func TestVerifyCustomer(t *testing.T) {
	// Arrange (menggunakan DB sungguhan, tapi in-memory)
	db := SetupTestDB(t)
	customerRepository := repository.NewCustomerRepository(
		db,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	service := service.NewAdminService(
		db,
		customerRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// Buat data customer yang PENDING
	pendingCustomer := &domain.Customer{
		ID: 2, NIK: "111", FullName: "Pending User", VerificationStatus: domain.VerificationPending,
		BirthDate: time.Now(), KtpUrl: "url", SelfieUrl: "url",
	}
	db.Create(pendingCustomer)

	// Skenario 1: Sukses verifikasi customer PENDING
	t.Run("Success Verifying Pending Customer", func(t *testing.T) {
		req := dto.VerificationRequest{Status: domain.VerificationVerified}

		// Act
		err := service.VerifyCustomer(context.Background(), 2, req)

		// Assert
		assert.NoError(t, err)

		// Cek langsung ke DB in-memory
		var updatedCustomer domain.Customer
		db.First(&updatedCustomer, 2)
		assert.Equal(t, domain.VerificationVerified, updatedCustomer.VerificationStatus)
	})

	// Skenario 2: Gagal verifikasi customer yang sudah VERIFIED
	t.Run("Fail Verifying Already Verified Customer", func(t *testing.T) {
		req := dto.VerificationRequest{Status: domain.VerificationRejected}

		// Act
		err := service.VerifyCustomer(context.Background(), 2, req)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "customer is not in PENDING state")
	})
}

func TestSetLimits(t *testing.T) {
	// Arrange
	db := SetupTestDB(t)

	customerRepository := repository.NewCustomerRepository(
		db,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	service := service.NewAdminService(
		db,
		customerRepository,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)

	// Buat data customer dan tenor
	db.Create(&domain.Customer{
		ID:        3,
		NIK:       "222",
		FullName:  "Limit User",
		BirthDate: time.Now(),
		KtpUrl:    "url",
		SelfieUrl: "url"},
	)
	db.Create(&domain.Tenor{ID: 1, DurationMonths: 3})
	db.Create(&domain.Tenor{ID: 2, DurationMonths: 6})

	t.Run("Success Setting New Limits", func(t *testing.T) {
		req := dto.SetLimits{
			Limits: []dto.LimitItemRequest{
				{TenorMonths: 3, LimitAmount: 1000},
				{TenorMonths: 6, LimitAmount: 2000},
			},
		}

		// Act
		err := service.SetLimits(context.Background(), 3, req)

		// Assert
		assert.NoError(t, err)

		// Cek DB
		var limits []domain.CustomerLimit
		db.Where("customer_id = ?", 3).Find(&limits)
		assert.Len(t, limits, 2)
		assert.Equal(t, float64(1000), limits[0].LimitAmount)
	})

	t.Run("Success Updating Existing Limits", func(t *testing.T) {
		req := dto.SetLimits{
			Limits: []dto.LimitItemRequest{
				{TenorMonths: 3, LimitAmount: 1500},
			},
		}

		// Act
		err := service.SetLimits(context.Background(), 3, req)

		// Assert
		assert.NoError(t, err)

		// Cek DB
		var updatedLimit domain.CustomerLimit
		db.Where("customer_id = ? AND tenor_id = ?", 3, 1).First(&updatedLimit)
		assert.Equal(t, float64(1500), updatedLimit.LimitAmount)
	})
}
