package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	transactionrepo "github.com/fazamuttaqien/multifinance/internal/repository/transaction"
	"github.com/fazamuttaqien/multifinance/pkg/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type TransactionRepositoryTestSuite struct {
	suite.Suite
	db                    *gorm.DB
	ctx                   context.Context
	meter                 metric.Meter
	tracer                trace.Tracer
	log                   *zap.Logger
	transactionRepository repository.TransactionRepository

	// Test data
	customerID uint64
	tenorID    uint
}

func (suite *TransactionRepositoryTestSuite) SetupSuite() {
	// Setup database connection untuk testing
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("DB_USER", "root"),
		common.GetEnv("DB_PASSWORD", ""),
		common.GetEnv("DB_HOST", "localhost"),
		common.GetEnv("DB_PORT", "3306"),
	)

	db, err := sql.Open("mysql", dsn)
	require.NoError(suite.T(), err)

	testDBName := "multifinance_transaction_test"

	// Drop database jika sudah ada
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	require.NoError(suite.T(), err)

	// Buat database untuk testing
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(suite.T(), err)

	db.Close()

	// Connect ke test database
	testDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("DB_USER", "root"),
		common.GetEnv("DB_PASSWORD", ""),
		common.GetEnv("DB_HOST", "localhost"),
		common.GetEnv("DB_PORT", "3306"),
		testDBName,
	)

	gormDB, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silent mode untuk testing
	})
	require.NoError(suite.T(), err)

	suite.db = gormDB
	suite.ctx = context.Background()

	// Auto migrate models
	err = suite.db.AutoMigrate(&model.Customer{}, &model.Tenor{}, &model.CustomerLimit{}, &model.Transaction{})
	require.NoError(suite.T(), err)

	// Initialize repository
	suite.transactionRepository = transactionrepo.NewTransactionRepository(suite.db, suite.meter, suite.tracer, suite.log)
}

func (suite *TransactionRepositoryTestSuite) TearDownSuite() {
	// Drop test database
	testDBName := "multifinance_transaction_test"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("DB_USER", "root"),
		common.GetEnv("DB_PASSWORD", ""),
		common.GetEnv("DB_HOST", "localhost"),
		common.GetEnv("DB_PORT", "3306"),
	)

	db, err := sql.Open("mysql", dsn)
	if err == nil {
		db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
		db.Close()
	}
}

func (suite *TransactionRepositoryTestSuite) SetupTest() {
	// Clean up database sebelum setiap test
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")

	// Setup test data
	suite.setupTestData()
}

func (suite *TransactionRepositoryTestSuite) setupTestData() {
	// Create test customer
	customer := model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Test Customer",
		LegalName:          "Test Customer Legal",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpPhotoUrl:        "https://example.com/ktp.jpg",
		SelfiePhotoUrl:     "https://example.com/selfie.jpg",
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(&customer).Error
	require.NoError(suite.T(), err)
	suite.customerID = customer.ID

	// Create test tenor
	tenor := model.Tenor{
		DurationMonths: 12,
		Description:    "12 Months",
	}
	err = suite.db.Create(&tenor).Error
	require.NoError(suite.T(), err)
	suite.tenorID = tenor.ID
}

func (suite *TransactionRepositoryTestSuite) TestCreateTransaction_Success() {
	// Arrange
	transaction := domain.Transaction{
		ContractNumber:         "CONTRACT001",
		CustomerID:             suite.customerID,
		TenorID:                suite.tenorID,
		AssetName:              "Honda Beat",
		OTRAmount:              15000000,
		AdminFee:               500000,
		TotalInterest:          2000000,
		TotalInstallmentAmount: 17500000,
		Status:                 domain.TransactionPending,
		TransactionDate:        time.Now(),
	}

	// Act
	err := suite.transactionRepository.CreateTransaction(suite.ctx, &transaction)

	// Assert
	assert.NoError(suite.T(), err)

	// Verify data tersimpan di database
	var savedTransaction model.Transaction
	err = suite.db.Where("contract_number = ?", transaction.ContractNumber).First(&savedTransaction).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), transaction.ContractNumber, savedTransaction.ContractNumber)
	assert.Equal(suite.T(), transaction.CustomerID, savedTransaction.CustomerID)
	assert.Equal(suite.T(), transaction.AssetName, savedTransaction.AssetName)
	assert.Equal(suite.T(), transaction.OTRAmount, savedTransaction.OTRAmount)
}

func (suite *TransactionRepositoryTestSuite) TestFindPaginatedByCustomerID_Success_WithoutFilter() {
	// Arrange
	transactions := []model.Transaction{
		{
			ContractNumber:         "CONTRACT001",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Beat",
			OTRAmount:              15000000,
			AdminFee:               500000,
			TotalInterest:          2000000,
			TotalInstallmentAmount: 17500000,
			Status:                 model.TransactionPending,
			TransactionDate:        time.Now().Add(-2 * time.Hour),
		},
		{
			ContractNumber:         "CONTRACT002",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Vario",
			OTRAmount:              18000000,
			AdminFee:               600000,
			TotalInterest:          2500000,
			TotalInstallmentAmount: 21100000,
			Status:                 model.TransactionApproved,
			TransactionDate:        time.Now().Add(-1 * time.Hour),
		},
		{
			ContractNumber:         "CONTRACT003",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Scoopy",
			OTRAmount:              16000000,
			AdminFee:               550000,
			TotalInterest:          2200000,
			TotalInstallmentAmount: 18750000,
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now(),
		},
	}

	for _, transaction := range transactions {
		err := suite.db.Create(&transaction).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Page:  1,
		Limit: 2,
	}

	// Act
	result, total, err := suite.transactionRepository.FindPaginatedByCustomerID(suite.ctx, suite.customerID, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), total)
	assert.Len(suite.T(), result, 2)
	// Verify ordering (newest first)
	assert.Equal(suite.T(), "CONTRACT003", result[0].ContractNumber)
	assert.Equal(suite.T(), "CONTRACT002", result[1].ContractNumber)
}

func (suite *TransactionRepositoryTestSuite) TestFindPaginatedByCustomerID_Success_WithStatusFilter() {
	// Arrange
	transactions := []model.Transaction{
		{
			ContractNumber:         "CONTRACT001",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Beat",
			OTRAmount:              15000000,
			AdminFee:               500000,
			TotalInterest:          2000000,
			TotalInstallmentAmount: 17500000,
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now().Add(-1 * time.Hour),
		},
		{
			ContractNumber:         "CONTRACT002",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Vario",
			OTRAmount:              18000000,
			AdminFee:               600000,
			TotalInterest:          2500000,
			TotalInstallmentAmount: 21100000,
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now(),
		},
		{
			ContractNumber:         "CONTRACT003",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Scoopy",
			OTRAmount:              16000000,
			AdminFee:               550000,
			TotalInterest:          2200000,
			TotalInstallmentAmount: 18750000,
			Status:                 model.TransactionPending,
			TransactionDate:        time.Now(),
		},
	}

	for _, transaction := range transactions {
		err := suite.db.Create(&transaction).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Status: string(model.TransactionActive),
		Page:   1,
		Limit:  10,
	}

	// Act
	result, total, err := suite.transactionRepository.FindPaginatedByCustomerID(suite.ctx, suite.customerID, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), total)
	assert.Len(suite.T(), result, 2)
	// Verify all results have ACTIVE status
	for _, transaction := range result {
		assert.Equal(suite.T(), domain.TransactionActive, transaction.Status)
	}
}

func (suite *TransactionRepositoryTestSuite) TestFindPaginatedByCustomerID_EmptyResult() {
	// Arrange
	params := domain.Params{
		Page:  1,
		Limit: 10,
	}

	// Act
	result, total, err := suite.transactionRepository.FindPaginatedByCustomerID(suite.ctx, 999999, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), total)
	assert.Len(suite.T(), result, 0)
}

func (suite *TransactionRepositoryTestSuite) TestFindPaginatedByCustomerID_SecondPage() {
	// Arrange
	for i := 0; i < 5; i++ {
		transaction := model.Transaction{
			ContractNumber:         fmt.Sprintf("CONTRACT%03d", i+1),
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              fmt.Sprintf("Asset %d", i+1),
			OTRAmount:              float64(15000000 + i*1000000),
			AdminFee:               500000,
			TotalInterest:          2000000,
			TotalInstallmentAmount: float64(17500000 + i*1000000),
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now().Add(time.Duration(-i) * time.Hour),
		}
		err := suite.db.Create(&transaction).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Page:  2,
		Limit: 2,
	}

	// Act
	result, total, err := suite.transactionRepository.FindPaginatedByCustomerID(suite.ctx, suite.customerID, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), total)
	assert.Len(suite.T(), result, 2)
	// Verify correct page data
	assert.Equal(suite.T(), "CONTRACT003", result[0].ContractNumber)
	assert.Equal(suite.T(), "CONTRACT004", result[1].ContractNumber)
}

func (suite *TransactionRepositoryTestSuite) TestSumActivePrincipalByCustomerIDAndTenorID_Success() {
	// Arrange
	transactions := []model.Transaction{
		{
			ContractNumber:         "CONTRACT001",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Beat",
			OTRAmount:              15000000,
			AdminFee:               500000,
			TotalInterest:          2000000,
			TotalInstallmentAmount: 17500000,
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now(),
		},
		{
			ContractNumber:         "CONTRACT002",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Vario",
			OTRAmount:              18000000,
			AdminFee:               600000,
			TotalInterest:          2500000,
			TotalInstallmentAmount: 21100000,
			Status:                 model.TransactionActive,
			TransactionDate:        time.Now(),
		},
		{
			ContractNumber:         "CONTRACT003",
			CustomerID:             suite.customerID,
			TenorID:                suite.tenorID,
			AssetName:              "Honda Scoopy",
			OTRAmount:              16000000,
			AdminFee:               550000,
			TotalInterest:          2200000,
			TotalInstallmentAmount: 18750000,
			Status:                 model.TransactionPending, // Should not be included
			TransactionDate:        time.Now(),
		},
	}

	for _, transaction := range transactions {
		err := suite.db.Create(&transaction).Error
		require.NoError(suite.T(), err)
	}

	// Act
	totalUsed, err := suite.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(suite.ctx, suite.customerID, suite.tenorID)

	// Assert
	assert.NoError(suite.T(), err)
	// Expected: (15000000 + 500000) + (18000000 + 600000) = 34100000
	assert.Equal(suite.T(), float64(34100000), totalUsed)
}

func (suite *TransactionRepositoryTestSuite) TestSumActivePrincipalByCustomerIDAndTenorID_NoActiveTransactions() {
	// Arrange
	transaction := model.Transaction{
		ContractNumber:         "CONTRACT001",
		CustomerID:             suite.customerID,
		TenorID:                suite.tenorID,
		AssetName:              "Honda Beat",
		OTRAmount:              15000000,
		AdminFee:               500000,
		TotalInterest:          2000000,
		TotalInstallmentAmount: 17500000,
		Status:                 model.TransactionPending,
		TransactionDate:        time.Now(),
	}
	err := suite.db.Create(&transaction).Error
	require.NoError(suite.T(), err)

	// Act
	totalUsed, err := suite.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(suite.ctx, suite.customerID, suite.tenorID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), float64(0), totalUsed)
}

func (suite *TransactionRepositoryTestSuite) TestSumActivePrincipalByCustomerIDAndTenorID_DifferentCustomerAndTenor() {
	// Arrange
	// Create another customer and tenor
	customer2 := model.Customer{
		NIK:                "9876543210987654",
		FullName:           "Another Customer",
		LegalName:          "Another Customer Legal",
		BirthPlace:         "Bandung",
		BirthDate:          time.Date(1992, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             6000000,
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(&customer2).Error
	require.NoError(suite.T(), err)

	tenor2 := model.Tenor{
		DurationMonths: 24,
		Description:    "24 Months",
	}
	err = suite.db.Create(&tenor2).Error
	require.NoError(suite.T(), err)

	// Create transactions for different combinations
	transactions := []model.Transaction{
		{
			ContractNumber:  "CONTRACT001",
			CustomerID:      suite.customerID,
			TenorID:         suite.tenorID,
			AssetName:       "Honda Beat",
			OTRAmount:       15000000,
			AdminFee:        500000,
			Status:          model.TransactionActive,
			TransactionDate: time.Now(),
		},
		{
			ContractNumber:  "CONTRACT002",
			CustomerID:      customer2.ID,
			TenorID:         suite.tenorID,
			AssetName:       "Honda Vario",
			OTRAmount:       18000000,
			AdminFee:        600000,
			Status:          model.TransactionActive,
			TransactionDate: time.Now(),
		},
		{
			ContractNumber:  "CONTRACT003",
			CustomerID:      suite.customerID,
			TenorID:         tenor2.ID,
			AssetName:       "Honda Scoopy",
			OTRAmount:       16000000,
			AdminFee:        550000,
			Status:          model.TransactionActive,
			TransactionDate: time.Now(),
		},
	}

	for _, transaction := range transactions {
		err := suite.db.Create(&transaction).Error
		require.NoError(suite.T(), err)
	}

	// Act
	totalUsed, err := suite.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(suite.ctx, suite.customerID, suite.tenorID)

	// Assert
	assert.NoError(suite.T(), err)
	// Only CONTRACT001 should be included: 15000000 + 500000 = 15500000
	assert.Equal(suite.T(), float64(15500000), totalUsed)
}

func (suite *TransactionRepositoryTestSuite) TestCreateTransaction_ValidationError() {
	// Arrange - Create transaction with invalid customer ID
	transaction := domain.Transaction{
		ContractNumber:         "CONTRACT001",
		CustomerID:             999999, // Non-existent customer
		TenorID:                suite.tenorID,
		AssetName:              "Honda Beat",
		OTRAmount:              15000000,
		AdminFee:               500000,
		TotalInterest:          2000000,
		TotalInstallmentAmount: 17500000,
		Status:                 domain.TransactionPending,
		TransactionDate:        time.Now(),
	}

	// Act
	err := suite.transactionRepository.CreateTransaction(suite.ctx, &transaction)

	// Assert
	// Should return foreign key constraint error
	assert.Error(suite.T(), err)
}

// Test runner function
func TestTransactionRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(TransactionRepositoryTestSuite))
}

// Benchmark tests
func BenchmarkTransactionRepository_CreateTransaction(b *testing.B) {
	// Setup database untuk benchmark
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/multifinance_transaction_test?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("DB_USER", "root"),
		common.GetEnv("DB_PASSWORD", ""),
		common.GetEnv("DB_HOST", "localhost"),
		common.GetEnv("DB_PORT", "3306"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}

	repo := transactionrepo.NewTransactionRepository(
		db,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L(),
	)
	ctx := context.Background()

	// Setup test data
	customer := model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Benchmark Customer",
		LegalName:          "Benchmark Customer Legal",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	db.Create(&customer)

	tenor := model.Tenor{
		DurationMonths: 12,
		Description:    "12 Months",
	}
	db.Create(&tenor)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			transaction := domain.Transaction{
				ContractNumber:         fmt.Sprintf("BENCH%010d", i),
				CustomerID:             customer.ID,
				TenorID:                tenor.ID,
				AssetName:              fmt.Sprintf("Benchmark Asset %d", i),
				OTRAmount:              15000000,
				AdminFee:               500000,
				TotalInterest:          2000000,
				TotalInstallmentAmount: 17500000,
				Status:                 domain.TransactionPending,
				TransactionDate:        time.Now(),
			}
			repo.CreateTransaction(ctx, &transaction)
			i++
		}
	})
}

func BenchmarkTransactionRepository_SumActivePrincipal(b *testing.B) {
	// Setup database untuk benchmark
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/multifinance_transaction_test?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("DB_USER", "root"),
		common.GetEnv("DB_PASSWORD", ""),
		common.GetEnv("DB_HOST", "localhost"),
		common.GetEnv("DB_PORT", "3306"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}

	repo := transactionrepo.NewTransactionRepository(
		db,
		otel.GetMeterProvider().Meter(""),
		otel.GetTracerProvider().Tracer(""),
		zap.L())
	ctx := context.Background()

	// Setup test data
	customer := model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Benchmark Customer",
		LegalName:          "Benchmark Customer Legal",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	db.Create(&customer)

	tenor := model.Tenor{
		DurationMonths: 12,
		Description:    "12 Months",
	}
	db.Create(&tenor)

	// Create some active transactions
	for i := range 10 {
		transaction := model.Transaction{
			ContractNumber:  fmt.Sprintf("SETUP%010d", i),
			CustomerID:      customer.ID,
			TenorID:         tenor.ID,
			AssetName:       fmt.Sprintf("Setup Asset %d", i),
			OTRAmount:       15000000,
			AdminFee:        500000,
			Status:          model.TransactionActive,
			TransactionDate: time.Now(),
		}
		db.Create(&transaction)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			repo.SumActivePrincipalByCustomerIDAndTenorID(ctx, customer.ID, tenor.ID)
		}
	})
}
