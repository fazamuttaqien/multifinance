package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/internal/domain"
	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	limitrepo "github.com/fazamuttaqien/multifinance/internal/repository/limit"
	"github.com/fazamuttaqien/multifinance/pkg/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

type LimitRepositoryTestSuite struct {
	suite.Suite
	db              *gorm.DB
	ctx             context.Context
	limitRepository repository.LimitRepository

	testCustomer model.Customer
	testTenors   []model.Tenor

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *LimitRepositoryTestSuite) SetupSuite() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)
	sqlDB, err := sql.Open("mysql", dsn)
	require.NoError(suite.T(), err)

	testDBName := "loan_system_limit_test"
	_, err = sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	require.NoError(suite.T(), err)
	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(suite.T(), err)
	sqlDB.Close()

	testDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
		testDBName,
	)
	gormDB, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(suite.T(), err)

	suite.db = gormDB
	suite.ctx = context.Background()

	suite.log = zap.NewNop()
	noopTracerProvider := noop_trace.NewTracerProvider()
	suite.tracer = noopTracerProvider.Tracer("test-limit-repository-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-limit-repository-meter")

	err = suite.db.AutoMigrate(&model.Customer{}, &model.Tenor{}, &model.CustomerLimit{}, &model.Transaction{})
	require.NoError(suite.T(), err)

	suite.limitRepository = limitrepo.NewLimitRepository(suite.db, suite.meter, suite.tracer, suite.log)
}

func (suite *LimitRepositoryTestSuite) TearDownSuite() {
	testDBName := "loan_system_limit_test"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)
	sqlDB, err := sql.Open("mysql", dsn)
	if err == nil {
		sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
		sqlDB.Close()
	}
}

func (suite *LimitRepositoryTestSuite) SetupTest() {
	// Bersihkan data dari tabel untuk memastikan isolasi tes
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")

	// Buat data prasyarat (customer dan tenor) yang akan digunakan di banyak tes
	suite.testCustomer = model.Customer{
		NIK:                "1112223334445556",
		FullName:           "Alan Smith",
		LegalName:          "Alan Smith",
		Password:           "alansmith123",
		Role:               "customer",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1995, 5, 20, 0, 0, 0, 0, time.UTC),
		Salary:             10000000,
		KtpPhotoUrl:        "https://res.cloudinary.com/test/image/upload/v1/ktp_placeholder.jpg",
		SelfiePhotoUrl:     "https://res.cloudinary.com/test/image/upload/v1/selfie_placeholder.jpg",
		VerificationStatus: model.VerificationVerified,
	}
	// Membuat record customer di database untuk digunakan sebagai foreign key
	require.NoError(suite.T(), suite.db.Create(&suite.testCustomer).Error)

	// Membuat record tenor di database untuk digunakan sebagai foreign key
	suite.testTenors = []model.Tenor{
		{DurationMonths: 3},
		{DurationMonths: 6},
		{DurationMonths: 12},
	}
	require.NoError(suite.T(), suite.db.Create(&suite.testTenors).Error)
}

func (suite *LimitRepositoryTestSuite) TestUpsertMany_Success() {
	limitsToInsert := []domain.CustomerLimit{
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[0].ID, LimitAmount: 1000000},
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[1].ID, LimitAmount: 2000000},
	}

	err := suite.limitRepository.UpsertMany(suite.ctx, limitsToInsert)

	assert.NoError(suite.T(), err)
	var count int64
	suite.db.Model(&model.CustomerLimit{}).Where("customer_id = ?", suite.testCustomer.ID).Count(&count)
	assert.Equal(suite.T(), int64(2), count)

	limitsToUpdate := []domain.CustomerLimit{
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[1].ID, LimitAmount: 2500000},
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[2].ID, LimitAmount: 5000000},
	}

	err = suite.limitRepository.UpsertMany(suite.ctx, limitsToUpdate)

	assert.NoError(suite.T(), err)
	suite.db.Model(&model.CustomerLimit{}).Where("customer_id = ?", suite.testCustomer.ID).Count(&count)
	assert.Equal(suite.T(), int64(3), count, "Total limits should be 3 after upserting new and updating old")

	var updatedLimit model.CustomerLimit
	suite.db.Where("customer_id = ? AND tenor_id = ?", suite.testCustomer.ID, suite.testTenors[1].ID).First(&updatedLimit)
	assert.Equal(suite.T(), 2500000.0, updatedLimit.LimitAmount, "Limit amount should be updated")
}

func (suite *LimitRepositoryTestSuite) TestUpsertMany_EmptySlice() {
	emptyLimits := []domain.CustomerLimit{}

	err := suite.limitRepository.UpsertMany(suite.ctx, emptyLimits)

	assert.NoError(suite.T(), err, "Upserting an empty slice should not return an error")
}

func (suite *LimitRepositoryTestSuite) TestFindAllByCustomerID_Success() {
	limits := []model.CustomerLimit{
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[0].ID, LimitAmount: 100},
		{CustomerID: suite.testCustomer.ID, TenorID: suite.testTenors[1].ID, LimitAmount: 200},
	}
	require.NoError(suite.T(), suite.db.Create(&limits).Error)

	otherCustomer := model.Customer{
		NIK:        "9998887776665554",
		FullName:   "Alan Smith",
		Password:   "alansmith123",
		Role:       "customer",
		BirthPlace: "Jakarta",
		BirthDate:  time.Date(1995, 5, 20, 0, 0, 0, 0, time.UTC),
		Salary:     10000000,
	}
	require.NoError(suite.T(), suite.db.Create(&otherCustomer).Error)
	otherLimit := model.CustomerLimit{CustomerID: otherCustomer.ID, TenorID: suite.testTenors[0].ID, LimitAmount: 999}
	require.NoError(suite.T(), suite.db.Create(&otherLimit).Error)

	result, err := suite.limitRepository.FindAllByCustomerID(suite.ctx, suite.testCustomer.ID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 2, "Should only return limits for the specified customer")
	assert.Equal(suite.T(), float64(100), result[0].LimitAmount)
	assert.Equal(suite.T(), float64(200), result[1].LimitAmount)
}

func (suite *LimitRepositoryTestSuite) TestFindAllByCustomerID_NotFound() {
	result, err := suite.limitRepository.FindAllByCustomerID(suite.ctx, suite.testCustomer.ID)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 0, "Result should be an empty slice for a customer with no limits")

	result, err = suite.limitRepository.FindAllByCustomerID(suite.ctx, 9999)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 0, "Result should be an empty slice for a non-existent customer")
}

func (suite *LimitRepositoryTestSuite) TestFindByCustomerIDAndTenorID_Success() {
	limitModel := model.CustomerLimit{
		CustomerID:  suite.testCustomer.ID,
		TenorID:     suite.testTenors[0].ID,
		LimitAmount: 123456.78,
	}
	require.NoError(suite.T(), suite.db.Create(&limitModel).Error)

	result, err := suite.limitRepository.FindByCustomerIDAndTenorID(suite.ctx, suite.testCustomer.ID, suite.testTenors[0].ID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), suite.testCustomer.ID, result.CustomerID)
	assert.Equal(suite.T(), suite.testTenors[0].ID, result.TenorID)
	assert.Equal(suite.T(), 123456.78, result.LimitAmount)
}

func (suite *LimitRepositoryTestSuite) TestFindByCustomerIDAndTenorID_NotFound() {
	result, err := suite.limitRepository.FindByCustomerIDAndTenorID(suite.ctx, suite.testCustomer.ID, suite.testTenors[0].ID)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result, "Result should be nil when the specific limit is not found")
}

func TestLimitRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(LimitRepositoryTestSuite))
}
