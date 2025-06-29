package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
	limitrepo "github.com/fazamuttaqien/multifinance/internal/repository/limit"
	tenorrepo "github.com/fazamuttaqien/multifinance/internal/repository/tenor"
	transactionrepo "github.com/fazamuttaqien/multifinance/internal/repository/transaction"
	"github.com/fazamuttaqien/multifinance/internal/service"
	profilesrv "github.com/fazamuttaqien/multifinance/internal/service/profile"
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

type ProfileServiceTestSuite struct {
	suite.Suite
	db  *gorm.DB
	ctx context.Context

	profileService        service.ProfileServices
	customerRepository    repository.CustomerRepository
	tenorRepository       repository.TenorRepository
	limitRepository       repository.LimitRepository
	transactionRepository repository.TransactionRepository

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *ProfileServiceTestSuite) SetupSuite() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "localhost"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)
	sqlDB, err := sql.Open("mysql", dsn)
	suite.Require().NoError(err)
	defer sqlDB.Close()

	testDbName := "loan_system_test"

	_, err = sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDbName))
	suite.Require().NoError(err)
	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", testDbName))
	suite.Require().NoError(err)

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

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-profile-service-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-profile-service-meter")

	err = suite.db.AutoMigrate(&model.Customer{}, &model.Tenor{}, &model.CustomerLimit{}, &model.Transaction{})
	suite.Require().NoError(err)

	suite.customerRepository = customerrepo.NewCustomerRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.tenorRepository = tenorrepo.NewTenorRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.limitRepository = limitrepo.NewLimitRepository(suite.db, suite.meter, suite.tracer, suite.log)
	suite.transactionRepository = transactionrepo.NewTransactionRepository(suite.db, suite.meter, suite.tracer, suite.log)

	suite.profileService = profilesrv.NewProfileService(suite.db, suite.customerRepository, suite.limitRepository, suite.tenorRepository, suite.transactionRepository, suite.meter, suite.tracer, suite.log)
}

func (suite *ProfileServiceTestSuite) TearDownSuite() {
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		suite.Require().NoError(err, "Failed to get underlying sql.DB")
		suite.Require().NoError(sqlDB.Close(), "Failed to close database connection")
	}
}

func (suite *ProfileServiceTestSuite) AfterTest(suiteName, testName string) {
	// Dijalankan setelah setiap tes untuk membersihkan data
	suite.db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	suite.db.Exec("TRUNCATE TABLE transactions")
	suite.db.Exec("TRUNCATE TABLE customer_limits")
	suite.db.Exec("TRUNCATE TABLE tenors")
	suite.db.Exec("TRUNCATE TABLE customers")
	suite.db.Exec("SET FOREIGN_KEY_CHECKS = 1")
}

func (suite *ProfileServiceTestSuite) seedCustomer() *model.Customer {
	customer := &model.Customer{
		NIK:            "1122334455667788",
		FullName:       "Jane Smith",
		LegalName:      "Jane Smith (Legal)",
		Password:       "janesmith123",
		Role:           "customer",
		BirthPlace:     "Bandung",
		BirthDate:      time.Date(1995, 5, 5, 0, 0, 0, 0, time.UTC),
		Salary:         10000000,
		KtpPhotoUrl:    "https://example.com/ktp.jpg",
		SelfiePhotoUrl: "https://example.com/selfie.jpg",
	}
	err := suite.db.Create(customer).Error
	suite.Require().NoError(err)
	return customer
}

func (suite *ProfileServiceTestSuite) TestRegister() {
	suite.T().Run("Success - Register new customer", func(t *testing.T) {
		// Arrange
		birthDate, _ := time.Parse("2006-01-02", "2000-01-01")
		req := &domain.Customer{
			NIK:        "1234567890123456",
			FullName:   "John Smith",
			LegalName:  "John Smith (Legal)",
			Password:   "johnsmith123",
			Role:       "customer",
			BirthPlace: "Jakarta",
			BirthDate:  birthDate,
			Salary:     5000000,
			KtpUrl:     "https://example.com/ktp.jpg",
			SelfieUrl:  "https://example.com/selfie.jpg",
		}

		// Act
		customer, err := suite.profileService.Create(suite.ctx, req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, customer)
		assert.Equal(t, req.NIK, customer.NIK)
		assert.Equal(t, domain.VerificationPending, customer.VerificationStatus)
		assert.Equal(t, req.FullName, customer.FullName)
		assert.Equal(t, req.Salary, customer.Salary)

		// Verifikasi data di database
		var savedCustomer model.Customer
		err = suite.db.First(&savedCustomer, "nik = ?", req.NIK).Error
		assert.NoError(t, err)
		assert.Equal(t, "John Smith", savedCustomer.FullName)
	})

	suite.T().Run("Failure - NIK already exists", func(t *testing.T) {
		// Arrange
		suite.seedCustomer()
		req := &domain.Customer{NIK: "1122334455667788"}

		// Act
		customer, err := suite.profileService.Create(suite.ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, customer)
		assert.ErrorIs(t, err, common.ErrNIKExists)
	})
}

func (suite *ProfileServiceTestSuite) TestGetMyLimits() {
	suite.T().Run("Success - Returns all limits with correct calculation", func(t *testing.T) {
		// Arrange
		customer := suite.seedCustomer()

		tenor3 := &model.Tenor{DurationMonths: 3}
		tenor6 := &model.Tenor{DurationMonths: 6}

		suite.db.Create(&[]*model.Tenor{tenor3, tenor6})

		suite.db.Create(&model.CustomerLimit{
			CustomerID:  customer.ID,
			TenorID:     tenor3.ID,
			LimitAmount: 1000,
		})

		suite.db.Create(&model.CustomerLimit{
			CustomerID:  customer.ID,
			TenorID:     tenor6.ID,
			LimitAmount: 5000,
		})

		suite.db.Create(&model.Transaction{
			CustomerID: customer.ID,
			TenorID:    tenor3.ID,
			OTRAmount:  250,
			AdminFee:   0,
			Status:     model.TransactionActive,
		})

		// Act
		limits, err := suite.profileService.GetMyLimits(suite.ctx, customer.ID)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, limits, 2)

		sort.Slice(limits, func(i, j int) bool {
			return limits[i].TenorMonths < limits[j].TenorMonths
		})

		assert.Equal(t, uint8(3), limits[0].TenorMonths)
		assert.Equal(t, float64(1000), limits[0].LimitAmount)
		assert.Equal(t, float64(250), limits[0].UsedAmount)
		assert.Equal(t, float64(750), limits[0].RemainingLimit)

		assert.Equal(t, uint8(6), limits[1].TenorMonths)
		assert.Equal(t, float64(5000), limits[1].LimitAmount)
		assert.Equal(t, float64(0), limits[1].UsedAmount)
		assert.Equal(t, float64(5000), limits[1].RemainingLimit)
	})
}

func (suite *ProfileServiceTestSuite) TestGetMyTransactions() {
	suite.T().Run("Success - Returns paginated transaction data", func(t *testing.T) {
		// Arrange
		customer := suite.seedCustomer()
		tenor := &model.Tenor{DurationMonths: 1}
		suite.db.Create(tenor)

		for i := range 11 {
			contractNumber := fmt.Sprintf("KTR-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%100000)
			tx := &model.Transaction{
				CustomerID:     customer.ID,
				TenorID:        tenor.ID,
				ContractNumber: contractNumber,
				AssetName:      fmt.Sprintf("Asset %d", i+1),
				OTRAmount:      float64(100 * (i + 1)),
				Status:         model.TransactionActive,
			}
			err := suite.db.Create(tx).Error
			suite.Require().NoError(err, "Failed to seed transaction %d", i+1)
		}

		params := domain.Params{Page: 2, Limit: 5}

		// Act
		result, err := suite.profileService.GetMyTransactions(suite.ctx, customer.ID, params)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Data, 5, "Page 2 should have 5 items")
		assert.Equal(t, int64(11), result.Total, "Total should be 11")
		assert.Equal(t, 2, result.Page, "Page should be 2")
		assert.Equal(t, 5, result.Limit, "Limit should be 5")
		assert.Equal(t, 3, result.TotalPages, "Total pages should be 3")
	})
}

func (suite *ProfileServiceTestSuite) TestUpdateProfile() {
	suite.T().Run("Success - Update customer profile", func(t *testing.T) {
		// Arrange
		customer := suite.seedCustomer()
		req := domain.Customer{
			FullName: "New Full Name",
			Salary:   15000000,
		}

		// Act
		err := suite.profileService.Update(suite.ctx, customer.ID, req)

		// Assert
		assert.NoError(t, err)
		var updatedCustomer model.Customer
		suite.db.First(&updatedCustomer, customer.ID)
		assert.Equal(t, "New Full Name", updatedCustomer.FullName)
		assert.Equal(t, float64(15000000), updatedCustomer.Salary)
		assert.Equal(t, customer.LegalName, updatedCustomer.LegalName)
	})

	suite.T().Run("Failure - Customer not found", func(t *testing.T) {
		// Arrange
		nonExistentID := uint64(999)
		req := domain.Customer{FullName: "New Name"}

		// Act
		err := suite.profileService.Update(suite.ctx, nonExistentID, req)

		// Assert
		assert.Error(t, err)
		assert.ErrorIs(t, err, common.ErrCustomerNotFound)
	})
}

func TestProfileServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ProfileServiceTestSuite))
}
