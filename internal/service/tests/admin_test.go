package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/dto"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
	"github.com/fazamuttaqien/multifinance/internal/service"
	adminsrv "github.com/fazamuttaqien/multifinance/internal/service/admin"
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

type AdminServiceTestSuite struct {
	suite.Suite
	db                 *gorm.DB
	ctx                context.Context
	adminService       service.AdminServices
	customerRepository repository.CustomerRepository
	meter              metric.Meter
	tracer             trace.Tracer
	log                *zap.Logger
}

func (suite *AdminServiceTestSuite) SetupSuite() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)
	sqlDB, err := sql.Open("mysql", dsn)
	suite.Require().NoError(err)
	defer sqlDB.Close()

	testDbName := fmt.Sprintf("loan_system_test_%d", rand.Intn(1000))

	_, err = sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDbName))
	suite.Require().NoError(err)
	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", testDbName))
	suite.Require().NoError(err)

	testDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
		testDbName,
	)
	gormDB, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	suite.Require().NoError(err)

	suite.db = gormDB
	suite.ctx = context.Background()

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-admin-service-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-admin-service-meter")

	err = suite.db.AutoMigrate(&model.Customer{}, &model.Tenor{}, &model.CustomerLimit{})
	suite.Require().NoError(err)

	suite.customerRepository = customerrepo.NewCustomerRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.adminService = adminsrv.NewAdminService(suite.db, suite.customerRepository, suite.meter, suite.tracer, suite.log)
}

func (suite *AdminServiceTestSuite) TearDownSuite() {
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		suite.Require().NoError(err, "Failed to get underlying sql.DB")
		suite.Require().NoError(sqlDB.Close(), "Failed to close database connection")
	}
}

func (suite *AdminServiceTestSuite) AfterTest(suiteName, testName string) {
	// Dijalankan setelah setiap tes untuk membersihkan data
	suite.db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	suite.db.Exec("TRUNCATE TABLE customer_limits")
	suite.db.Exec("TRUNCATE TABLE tenors")
	suite.db.Exec("TRUNCATE TABLE customers")
	suite.db.Exec("SET FOREIGN_KEY_CHECKS = 1")
}

func (suite *AdminServiceTestSuite) seedCustomer(fullName string, status domain.VerificationStatus) *model.Customer {
	nik := fmt.Sprintf("%016d", rand.Int63n(1e16))

	customer := &model.Customer{
		NIK:                nik,
		FullName:           fullName,
		LegalName:          fullName,
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC),
		Salary:             10000000,
		KtpPhotoUrl:        "http://example.com/ktp.jpg",
		SelfiePhotoUrl:     "http://example.com/selfie.jpg",
		VerificationStatus: model.VerificationStatus(status),
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)
	return customer
}

func (suite *AdminServiceTestSuite) TestListCustomers() {
	suite.T().Run("Success - Get Pending Customers", func(t *testing.T) {
		// Arrange
		suite.seedCustomer("John Smith", domain.VerificationPending)
		suite.seedCustomer("Jane Smith", domain.VerificationPending)

		params := domain.Params{Status: "PENDING", Page: 1, Limit: 10}

		// Act
		result, err := suite.adminService.ListCustomers(suite.ctx, params)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(2), result.Total)
		assert.Len(t, result.Data, 2)
		data, ok := result.Data.([]domain.Customer)
		assert.True(t, ok)

		expectedNames := map[string]bool{
			"John Smith": true,
			"Jane Smith": true,
		}
		foundNames := make(map[string]bool)
		for _, cust := range data {
			foundNames[cust.FullName] = true
		}
		assert.Equal(t, expectedNames, foundNames, "The list of pending customers should match the seeded ones")
	})

	suite.T().Run("Success - Pagination works", func(t *testing.T) {
		// Arrange
		for i := range 15 {
			suite.seedCustomer(fmt.Sprintf("All User %d", i), domain.VerificationPending)
		}
		params := domain.Params{Page: 2, Limit: 5}

		// Act
		result, err := suite.adminService.ListCustomers(suite.ctx, params)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(17), result.Total)
		assert.Equal(t, 4, result.TotalPages)
		assert.Equal(t, 2, result.Page)
		assert.Len(t, result.Data, 5)
	})
}

func (suite *AdminServiceTestSuite) TestGetCustomerByID() {
	suite.T().Run("Success - Customer Found", func(t *testing.T) {
		// Arrange
		seededCustomer := suite.seedCustomer("Annisa", domain.VerificationVerified)

		// Act
		result, err := suite.adminService.GetCustomerByID(suite.ctx, seededCustomer.ID)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, seededCustomer.ID, result.ID)
		assert.Equal(t, "Annisa", result.FullName)
	})

	suite.T().Run("Failure - Customer Not Found", func(t *testing.T) {
		// Act
		result, err := suite.adminService.GetCustomerByID(suite.ctx, 9999)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}

func (suite *AdminServiceTestSuite) TestVerifyCustomer() {
	suite.T().Run("Success - Verifying Pending Customer", func(t *testing.T) {
		// Arrange
		pendingCustomer := suite.seedCustomer("John Doe", domain.VerificationPending)
		req := dto.VerificationRequest{Status: domain.VerificationVerified}

		// Act
		err := suite.adminService.VerifyCustomer(suite.ctx, pendingCustomer.ID, req)

		// Assert
		assert.NoError(t, err)
		var updatedCustomer model.Customer
		err = suite.db.First(&updatedCustomer, pendingCustomer.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, model.VerificationVerified, updatedCustomer.VerificationStatus)
	})

	suite.T().Run("Failure - Verifying Already Verified Customer", func(t *testing.T) {
		// Arrange
		verifiedCustomer := suite.seedCustomer("John Doe", domain.VerificationVerified)
		req := dto.VerificationRequest{Status: domain.VerificationRejected}

		// Act
		err := suite.adminService.VerifyCustomer(suite.ctx, verifiedCustomer.ID, req)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "customer is not in PENDING state")
	})

	suite.T().Run("Failure - Customer Not Found", func(t *testing.T) {
		// Arrange
		req := dto.VerificationRequest{Status: domain.VerificationVerified}

		// Act
		err := suite.adminService.VerifyCustomer(suite.ctx, 9999, req)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}

func (suite *AdminServiceTestSuite) TestSetLimits() {
	// Arrange common data for all sub-tests in this test method
	customer := suite.seedCustomer("Jane Doe", domain.VerificationVerified)
	tenor3 := &model.Tenor{DurationMonths: 3}
	tenor6 := &model.Tenor{DurationMonths: 6}
	suite.db.Create(&[]*model.Tenor{tenor3, tenor6})

	suite.T().Run("Success - Setting New Limits", func(t *testing.T) {
		// Arrange
		req := dto.SetLimits{
			Limits: []dto.LimitItemRequest{
				{TenorMonths: 3, LimitAmount: 1000},
				{TenorMonths: 6, LimitAmount: 2000},
			},
		}

		// Act
		err := suite.adminService.SetLimits(suite.ctx, customer.ID, req)

		// Assert
		assert.NoError(t, err)
		var limits []model.CustomerLimit
		suite.db.Where("customer_id = ?", customer.ID).Order("tenor_id asc").Find(&limits)
		assert.Len(t, limits, 2)
		assert.Equal(t, tenor3.ID, limits[0].TenorID)
		assert.Equal(t, float64(1000), limits[0].LimitAmount)
		assert.Equal(t, tenor6.ID, limits[1].TenorID)
		assert.Equal(t, float64(2000), limits[1].LimitAmount)
	})

	suite.T().Run("Success - Updating Existing Limits", func(t *testing.T) {
		// Arrange: Seed initial limits first
		suite.db.Create(&model.CustomerLimit{CustomerID: customer.ID, TenorID: tenor3.ID, LimitAmount: 500})
		suite.db.Create(&model.CustomerLimit{CustomerID: customer.ID, TenorID: tenor6.ID, LimitAmount: 1000})

		req := dto.SetLimits{
			Limits: []dto.LimitItemRequest{
				{TenorMonths: 3, LimitAmount: 1500}, // Update existing
				{TenorMonths: 6, LimitAmount: 2500}, // Update existing
			},
		}

		// Act
		err := suite.adminService.SetLimits(suite.ctx, customer.ID, req)

		// Assert
		assert.NoError(t, err)
		var updatedLimit3, updatedLimit6 model.CustomerLimit
		suite.db.Where("customer_id = ? AND tenor_id = ?", customer.ID, tenor3.ID).First(&updatedLimit3)
		suite.db.Where("customer_id = ? AND tenor_id = ?", customer.ID, tenor6.ID).First(&updatedLimit6)
		assert.Equal(t, float64(1500), updatedLimit3.LimitAmount)
		assert.Equal(t, float64(2500), updatedLimit6.LimitAmount)
	})

	suite.T().Run("Failure - Tenor not found", func(t *testing.T) {
		// Arrange
		req := dto.SetLimits{
			Limits: []dto.LimitItemRequest{
				{TenorMonths: 99, LimitAmount: 5000}, // 99 months tenor does not exist
			},
		}

		// Act
		err := suite.adminService.SetLimits(suite.ctx, customer.ID, req)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tenor not found: for 99 months")
	})
}

func TestAdminServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AdminServiceTestSuite))
}
