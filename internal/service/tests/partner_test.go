package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
	limitrepo "github.com/fazamuttaqien/multifinance/internal/repository/limit"
	tenorrepo "github.com/fazamuttaqien/multifinance/internal/repository/tenor"
	transactionrepo "github.com/fazamuttaqien/multifinance/internal/repository/transaction"
	"github.com/fazamuttaqien/multifinance/internal/service"
	partnersrv "github.com/fazamuttaqien/multifinance/internal/service/partner"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/metric"
	noop_metric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	noop_trace "go.opentelemetry.io/otel/trace/noop"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PartnerServiceTestSuite struct {
	suite.Suite
	db  *gorm.DB
	ctx context.Context

	partnerService        service.PartnerServices
	customerRepository    repository.CustomerRepository
	tenorRepository       repository.TenorRepository
	limitRepository       repository.LimitRepository
	transactionRepository repository.TransactionRepository

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *PartnerServiceTestSuite) SetupSuite() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "localhost"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)

	db, err := sql.Open("mysql", dsn)
	suite.Require().NoError(err)

	testDbName := "loan_system_test"

	// Drop database jika sudah ada
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDbName))
	suite.Require().NoError(err)

	// Buat database untuk testing
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDbName))
	suite.Require().NoError(err)

	db.Close()

	// Connect ke test database
	testDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "localhost"),
		common.GetEnv("MYSQL_PORT", "3306"),
		testDbName,
	)

	gormDB, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	suite.Require().NoError(err)

	suite.db = gormDB
	suite.ctx = context.Background()

	// Setup logging and tracing
	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-partner-service-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-partner-service-meter")

	// Auto migrate models
	err = suite.db.AutoMigrate(
		&model.Customer{},
		&model.Tenor{},
		&model.CustomerLimit{},
		&model.Transaction{},
	)
	suite.Require().NoError(err)

	// Initialize repositories
	suite.customerRepository = customerrepo.NewCustomerRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.tenorRepository = tenorrepo.NewTenorRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.limitRepository = limitrepo.NewLimitRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.transactionRepository = transactionrepo.NewTransactionRepository(suite.db, suite.meter, suite.tracer, suite.log)

	// Initialize service
	suite.partnerService = partnersrv.NewPartnerService(
		suite.db,
		suite.customerRepository,
		suite.tenorRepository,
		suite.limitRepository,
		suite.transactionRepository,
		suite.meter,
		suite.tracer,
		suite.log,
	)
}

func (suite *PartnerServiceTestSuite) TearDownSuite() {
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func (suite *PartnerServiceTestSuite) SetupTest() {
	// Clean up database sebelum setiap test
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")
}

func (suite *PartnerServiceTestSuite) seedTestData() (customer *model.Customer, tenor *model.Tenor, limit *model.CustomerLimit) {
	// Create test customer
	customer = &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Customer1",
		LegalName:          "Customer1",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpPhotoUrl:        "https://example.com/ktp.jpg",
		SelfiePhotoUrl:     "https://example.com/selfie.jpg",
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	// Create test tenor
	tenor = &model.Tenor{
		DurationMonths: 6,
		Description:    "Create test tenor for Customer1",
	}
	err = suite.db.Create(tenor).Error
	suite.Require().NoError(err)

	// Create test limit
	limit = &model.CustomerLimit{
		CustomerID:  customer.ID,
		TenorID:     tenor.ID,
		LimitAmount: 50000,
	}
	err = suite.db.Create(limit).Error
	suite.Require().NoError(err)

	return customer, tenor, limit
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Success_Approved() {
	// Arrange
	customer, tenor, _ := suite.seedTestData()

	req := dto.CheckLimitRequest{
		CustomerNIK:       customer.NIK,
		TenorMonths:       tenor.DurationMonths,
		TransactionAmount: 30000,
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "approved", result.Status)
	assert.Equal(suite.T(), float64(50000), result.RemainingLimit)
	assert.Equal(suite.T(), "Limit is sufficient.", result.Message)
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Success_Rejected_InsufficientLimit() {
	// Arrange
	customer, tenor, _ := suite.seedTestData()

	// Generate contract number
	contractNumber := fmt.Sprintf("KTR-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%100000)

	// Create existing transaction to reduce available limit
	existingTransaction := &model.Transaction{
		CustomerID:             customer.ID,
		TenorID:                tenor.ID,
		AssetName:              "Existing Asset",
		OTRAmount:              40000,
		AdminFee:               1000,
		TotalInstallmentAmount: 7000,
		TotalInterest:          2000,
		ContractNumber:         contractNumber,
		TransactionDate:        time.Now(),
		Status:                 model.TransactionActive,
	}
	err := suite.db.Create(existingTransaction).Error
	suite.Require().NoError(err)

	req := dto.CheckLimitRequest{
		CustomerNIK:       customer.NIK,
		TenorMonths:       tenor.DurationMonths,
		TransactionAmount: 15000, // Total would be 55000, exceeding limit of 50000
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "rejected", result.Status)
	assert.True(suite.T(), result.RemainingLimit < req.TransactionAmount)
	assert.Equal(suite.T(), "Insufficient limit for this transaction.", result.Message)
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Failure_CustomerNotFound() {
	// Arrange
	req := dto.CheckLimitRequest{
		CustomerNIK:       "nonexistent",
		TenorMonths:       6,
		TransactionAmount: 30000,
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrCustomerNotFound)
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Failure_CustomerNotVerified() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Unverified Customer",
		LegalName:          "Unverified Customer Legal",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		VerificationStatus: model.VerificationPending, // Not verified
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	req := dto.CheckLimitRequest{
		CustomerNIK:       customer.NIK,
		TenorMonths:       6,
		TransactionAmount: 30000,
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "not verified")
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Failure_TenorNotFound() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Test Customer",
		BirthDate:          time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC),
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	req := dto.CheckLimitRequest{
		CustomerNIK:       customer.NIK,
		TenorMonths:       99, // Nonexistent tenor
		TransactionAmount: 30000,
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrTenorNotFound)
}

func (suite *PartnerServiceTestSuite) TestCheckLimit_Failure_LimitNotSet() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Test Customer",
		BirthDate:          time.Date(1992, 1, 1, 0, 0, 0, 0, time.UTC),
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	tenor := &model.Tenor{
		DurationMonths: 6,
	}
	err = suite.db.Create(tenor).Error
	suite.Require().NoError(err)

	// No limit created for this customer-tenor combination

	req := dto.CheckLimitRequest{
		CustomerNIK:       customer.NIK,
		TenorMonths:       tenor.DurationMonths,
		TransactionAmount: 30000,
	}

	// Act
	result, err := suite.partnerService.CheckLimit(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrLimitNotSet)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Success() {
	// Arrange
	customer, tenor, _ := suite.seedTestData()

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: tenor.DurationMonths,
		AssetName:   "Test Asset",
		OTRAmount:   40000,
		AdminFee:    1000,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), customer.ID, result.CustomerID)
	assert.Equal(suite.T(), tenor.ID, result.TenorID)
	assert.Equal(suite.T(), "Test Asset", result.AssetName)
	assert.Equal(suite.T(), float64(40000), result.OTRAmount)
	assert.Equal(suite.T(), float64(1000), result.AdminFee)
	assert.Equal(suite.T(), domain.TransactionActive, result.Status)

	// Verify transaction is saved in database
	var savedTransaction model.Transaction
	err = suite.db.First(&savedTransaction, result.ID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), result.AssetName, savedTransaction.AssetName)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Failure_InsufficientLimit() {
	// Arrange
	customer, tenor, _ := suite.seedTestData()

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: tenor.DurationMonths,
		AssetName:   "Expensive Asset",
		OTRAmount:   60000, // Exceeds limit of 50000
		AdminFee:    0,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrInsufficientLimit)

	// Verify no transaction was created (rollback successful)
	var count int64
	suite.db.Model(&model.Transaction{}).Count(&count)
	assert.Equal(suite.T(), int64(0), count)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Failure_CustomerNotFound() {
	// Arrange
	req := dto.CreateTransactionRequest{
		CustomerNIK: "nonexistent",
		TenorMonths: 6,
		AssetName:   "Test Asset",
		OTRAmount:   40000,
		AdminFee:    1000,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrCustomerNotFound)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Failure_CustomerNotVerified() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Unverified Customer",
		BirthDate:          time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
		VerificationStatus: model.VerificationPending,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: 6,
		AssetName:   "Test Asset",
		OTRAmount:   40000,
		AdminFee:    1000,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "not verified")
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Failure_TenorNotFound() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Test Customer",
		BirthDate:          time.Date(1993, 1, 1, 0, 0, 0, 0, time.UTC),
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: 99, // Nonexistent tenor
		AssetName:   "Test Asset",
		OTRAmount:   40000,
		AdminFee:    1000,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrTenorNotFound)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Failure_LimitNotSet() {
	// Arrange
	customer := &model.Customer{
		NIK:                "1234567890123456",
		FullName:           "Test Customer",
		BirthDate:          time.Date(1994, 1, 1, 0, 0, 0, 0, time.UTC),
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)

	tenor := &model.Tenor{
		DurationMonths: 6,
	}
	err = suite.db.Create(tenor).Error
	suite.Require().NoError(err)

	// No limit created for this customer-tenor combination

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: tenor.DurationMonths,
		AssetName:   "Test Asset",
		OTRAmount:   40000,
		AdminFee:    1000,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, common.ErrLimitNotSet)
}

func (suite *PartnerServiceTestSuite) TestCreateTransaction_Success_WithExistingTransactions() {
	// Arrange
	customer, tenor, _ := suite.seedTestData()

	// Create existing transaction
	existingTx := &model.Transaction{
		CustomerID: customer.ID,
		TenorID:    tenor.ID,
		AssetName:  "Existing Asset",
		OTRAmount:  20000,
		AdminFee:   500,
		// InstallmentAmount: 3500,
		// InterestAmount:    1000,
		Status: model.TransactionActive,
	}
	err := suite.db.Create(existingTx).Error
	suite.Require().NoError(err)

	req := dto.CreateTransactionRequest{
		CustomerNIK: customer.NIK,
		TenorMonths: tenor.DurationMonths,
		AssetName:   "New Asset",
		OTRAmount:   25000, // Total would be 45000, still within limit of 50000
		AdminFee:    500,
	}

	// Act
	result, err := suite.partnerService.CreateTransaction(suite.ctx, req)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "New Asset", result.AssetName)
	assert.Equal(suite.T(), float64(25000), result.OTRAmount)

	// Verify both transactions exist
	var count int64
	suite.db.Model(&model.Transaction{}).Count(&count)
	assert.Equal(suite.T(), int64(2), count)
}

// Test runner function
func TestPartnerServiceTestSuite(t *testing.T) {
	suite.Run(t, new(PartnerServiceTestSuite))
}
