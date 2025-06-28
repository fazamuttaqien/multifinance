package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	"github.com/fazamuttaqien/multifinance/helper/common"
	"github.com/fazamuttaqien/multifinance/repository"
	"github.com/fazamuttaqien/multifinance/service"
	"github.com/stretchr/testify/assert"
)

func TestCheckLimit(t *testing.T) {
	// Arrange - buat instance dari semua mock
	mockCustomerRepo := &mockCustomerRepository{}
	mockTenorRepo := &mockTenorRepository{}
	mockLimitRepo := &mockLimitRepository{}
	mockTxnRepo := &mockTransactionRepository{}

	// Buat instance service dengan semua mock
	service := service.NewPartnerService(mockCustomerRepo, mockTenorRepo, mockLimitRepo, mockTxnRepo)

	// --- Skenario 1: Sukses - Limit Cukup (Approved) ---
	t.Run("Success - Limit Approved", func(t *testing.T) {
		// Konfigurasi mock untuk happy path
		mockCustomerRepo.MockFindByNIKData = &domain.Customer{ID: 2, VerificationStatus: domain.VerificationVerified}
		mockTenorRepo.MockFindByDurationData = &domain.Tenor{ID: 4}
		mockLimitRepo.MockFindByCIDAndTIDData = &domain.CustomerLimit{LimitAmount: 10000}
		mockTxnRepo.MockSumActiveData = 2000 // Pemakaian 2000, sisa 8000

		req := dto.CheckLimitRequest{
			TransactionAmount: 5000, // Amount yang diminta (5000) < sisa limit (8000)
		}

		// Act
		res, err := service.CheckLimit(context.Background(), req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "approved", res.Status)
		assert.Equal(t, float64(8000), res.RemainingLimit)
		assert.Equal(t, "Limit is sufficient.", res.Message)
	})

	// --- Skenario 2: Sukses - Limit Tidak Cukup (Rejected) ---
	t.Run("Success - Limit Insufficient (Rejected)", func(t *testing.T) {
		// Konfigurasi mock
		mockCustomerRepo.MockFindByNIKData = &domain.Customer{ID: 2, VerificationStatus: domain.VerificationVerified}
		mockTenorRepo.MockFindByDurationData = &domain.Tenor{ID: 4}
		mockLimitRepo.MockFindByCIDAndTIDData = &domain.CustomerLimit{LimitAmount: 10000}
		mockTxnRepo.MockSumActiveData = 8000 // Pemakaian 8000, sisa 2000

		req := dto.CheckLimitRequest{
			TransactionAmount: 3000, // Amount yang diminta (3000) > sisa limit (2000)
		}

		// Act
		res, err := service.CheckLimit(context.Background(), req)

		// Assert
		assert.NoError(t, err) // Ini bukan error sistem, tapi hasil bisnis
		assert.NotNil(t, res)
		assert.Equal(t, "rejected", res.Status)
		assert.Equal(t, float64(2000), res.RemainingLimit)
		assert.Equal(t, "Insufficient limit for this transaction.", res.Message)
	})

	// --- Skenario 3: Gagal - Customer Tidak Ditemukan ---
	t.Run("Failure - Customer Not Found", func(t *testing.T) {
		mockCustomerRepo.MockFindByNIKData = nil // Simulasikan customer tidak ada

		// Act
		res, err := service.CheckLimit(context.Background(), dto.CheckLimitRequest{})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})

	// --- Skenario 4: Gagal - Customer Belum Terverifikasi ---
	t.Run("Failure - Customer Not Verified", func(t *testing.T) {
		mockCustomerRepo.MockFindByNIKData = &domain.Customer{ID: 2, VerificationStatus: domain.VerificationPending}

		// Act
		res, err := service.CheckLimit(context.Background(), dto.CheckLimitRequest{})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "not verified")
	})

	// --- Skenario 5: Gagal - Tenor Tidak Ditemukan ---
	t.Run("Failure - Tenor Not Found", func(t *testing.T) {
		mockCustomerRepo.MockFindByNIKData = &domain.Customer{ID: 2, VerificationStatus: domain.VerificationVerified} // Customer OK
		mockTenorRepo.MockFindByDurationData = nil                                                                    // Tenor tidak ada

		// Act
		res, err := service.CheckLimit(context.Background(), dto.CheckLimitRequest{})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, common.ErrTenorNotFound)
	})

	// --- Skenario 6: Gagal - Limit Belum Ditetapkan ---
	t.Run("Failure - Limit Not Set", func(t *testing.T) {
		mockCustomerRepo.MockFindByNIKData = &domain.Customer{ID: 2, VerificationStatus: domain.VerificationVerified} // Customer OK
		mockTenorRepo.MockFindByDurationData = &domain.Tenor{ID: 4}                                                   // Tenor OK
		mockLimitRepo.MockFindByCIDAndTIDData = nil                                                                   // Limit tidak ada

		// Act
		res, err := service.CheckLimit(context.Background(), dto.CheckLimitRequest{})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, common.ErrLimitNotSet)
	})

	// --- Skenario 7: Gagal - Error dari Repository ---
	t.Run("Failure - Repository Error", func(t *testing.T) {
		expectedErr := errors.New("database connection failed")
		mockCustomerRepo.MockFindByNIKData = nil
		mockCustomerRepo.MockError = expectedErr // Simulasikan error dari DB

		// Act
		res, err := service.CheckLimit(context.Background(), dto.CheckLimitRequest{})

		// Assert
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestCreateTransaction(t *testing.T) {
	// Arrange - Siapkan semua dependensi yang diperlukan
	// Kita akan membuat ulang DB untuk setiap sub-test untuk memastikan isolasi

	// --- Skenario 1: Sukses - Transaksi berhasil dibuat ---
	t.Run("Success - Create Transaction", func(t *testing.T) {
		// Arrange
		db := setupTestDB(t)

		customerRepo := repository.NewCustomerRepository(db)
		tenorRepo := repository.NewTenorRepository(db)
		limitRepo := repository.NewLimitRepository(db)
		customerTxnRepo := repository.NewCustomerRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)

		service := service.NewPartnerService(db, customerRepo, tenorRepo, limitRepo, customerTxnRepo, transactionRepo)

		// Seed data yang diperlukan
		testCustomer := &domain.Customer{ID: 2, NIK: "1234567890123456", VerificationStatus: domain.VerificationVerified, FullName: "Test", BirthDate: time.Now(), KTPPhotoURL: "url", SelfiePhotoURL: "url"}
		testTenor := &domain.Tenor{ID: 1, DurationMonths: 6}
		testLimit := &domain.CustomerLimit{CustomerID: 2, TenorID: 1, LimitAmount: 50000}

		db.Create(testCustomer)
		db.Create(testTenor)
		db.Create(testLimit)

		req := dto.Transaction{
			NIK:         "1234567890123456",
			TenorMonths: 6,
			AssetName:   "Test Asset",
			OTRAmount:   40000,
			AdminFee:    1000,
		}

		// Act
		createdTx, err := service.CreateTransaction(context.Background(), req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, createdTx)
		assert.Equal(t, uint64(2), createdTx.CustomerID)
		assert.Equal(t, float64(40000), createdTx.OTRAmount)
		assert.Equal(t, domain.TransactionActive, createdTx.Status)

		// Verifikasi bahwa data benar-benar tersimpan di DB
		var txRecord domain.Transaction
		err = db.First(&txRecord, createdTx.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, "Test Asset", txRecord.AssetName)
	})

	// --- Skenario 2: Gagal - Limit tidak cukup ---
	t.Run("Failure - Insufficient Limit", func(t *testing.T) {
		// Arrange
		db := setupTestDB(t)
		customerRepo := repository.NewCustomerRepository(db)
		tenorRepo := repository.NewTenorRepository(db)
		limitRepo := repository.NewLimitRepository(db)
		customerTxnRepo := repository.NewCustomerRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)

		service := service.NewPartnerService(db, customerRepo, tenorRepo, limitRepo, customerTxnRepo, transactionRepo)

		// Seed data dengan limit yang kecil
		db.Create(&domain.Customer{ID: 2, NIK: "1234567890123456", VerificationStatus: domain.VerificationVerified, FullName: "Test", BirthDate: time.Now(), KTPPhotoURL: "url", SelfiePhotoURL: "url"})
		db.Create(&domain.Tenor{ID: 1, DurationMonths: 6})
		db.Create(&domain.CustomerLimit{CustomerID: 2, TenorID: 1, LimitAmount: 20000})

		req := dto.Transaction{
			NIK:         "1234567890123456",
			TenorMonths: 6,
			AssetName:   "Expensive Asset",
			OTRAmount:   25000, // Melebihi limit
			AdminFee:    0,
		}

		// Act
		createdTx, err := service.CreateTransaction(context.Background(), req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, createdTx)
		assert.ErrorIs(t, err, common.ErrInsufficientLimit)

		// Verifikasi bahwa tidak ada transaksi yang dibuat (ROLLBACK berhasil)
		var count int64
		db.Model(&domain.Transaction{}).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	// --- Skenario 3: Gagal - Customer tidak terverifikasi ---
	t.Run("Failure - Customer Not Verified", func(t *testing.T) {
		// Arrange
		db := setupTestDB(t)

		customerRepo := repository.NewCustomerRepository(db)
		tenorRepo := repository.NewTenorRepository(db)
		limitRepo := repository.NewLimitRepository(db)
		customerTxnRepo := repository.NewCustomerRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)

		service := service.NewPartnerService(db, customerRepo, tenorRepo, limitRepo, customerTxnRepo, transactionRepo)

		// Seed customer dengan status PENDING
		db.Create(&domain.Customer{ID: 2, NIK: "1234567890123456", VerificationStatus: domain.VerificationPending, FullName: "Test", BirthDate: time.Now(), KTPPhotoURL: "url", SelfiePhotoURL: "url"})

		req := dto.Transaction{NIK: "1234567890123456"}

		// Act
		_, err := service.CreateTransaction(context.Background(), req)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not verified")
	})

	// --- Skenario 4: Gagal - Customer tidak ditemukan ---
	t.Run("Failure - Customer Not Found", func(t *testing.T) {
		// Arrange
		db := setupTestDB(t)
		// ... (inisialisasi repo dan service) ...
		customerRepo := repository.NewCustomerRepository(db)
		tenorRepo := repository.NewTenorRepository(db)
		limitRepo := repository.NewLimitRepository(db)
		customerTxnRepo := repository.NewCustomerRepository(db)
		transactionRepo := repository.NewTransactionRepository(db)
		service := service.NewPartnerService(db, customerRepo, tenorRepo, limitRepo, customerTxnRepo, transactionRepo)

		// Tidak ada customer yang di-seed

		req := dto.Transaction{NIK: "0000"}

		// Act
		_, err := service.CreateTransaction(context.Background(), req)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}
