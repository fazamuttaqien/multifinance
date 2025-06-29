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
	customerrepo "github.com/fazamuttaqien/multifinance/internal/repository/customer"
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

type CustomerRepositoryTestSuite struct {
	suite.Suite
	db                 *gorm.DB
	ctx                context.Context
	customerRepository repository.CustomerRepository

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *CustomerRepositoryTestSuite) SetupSuite() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)

	db, err := sql.Open("mysql", dsn)
	require.NoError(suite.T(), err)

	testDBName := "loan_system_test"

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	require.NoError(suite.T(), err)

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(suite.T(), err)

	db.Close()

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
	suite.tracer = noopTracerProvider.Tracer("test-customer-repository-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-customer-repository-meter")

	err = suite.db.AutoMigrate(
		&model.Customer{},
		&model.Tenor{},
		&model.CustomerLimit{},
		&model.Transaction{},
	)
	require.NoError(suite.T(), err)

	suite.customerRepository = customerrepo.NewCustomerRepository(suite.db, suite.meter, suite.tracer, suite.log)
}

func (suite *CustomerRepositoryTestSuite) TearDownSuite() {
	testDBName := "loan_system_test"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)

	db, err := sql.Open("mysql", dsn)
	if err == nil {
		db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
		db.Close()
	}
}

func (suite *CustomerRepositoryTestSuite) SetupTest() {
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")
}

func (suite *CustomerRepositoryTestSuite) TestSave_Success() {
	customer := domain.Customer{
		NIK:                "1234567890123456",
		FullName:           "John Doe",
		LegalName:          "John Doe",
		Password:           "johndoe123",
		Role:               "customer",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpUrl:             "https://example.com/ktp.jpg",
		SelfieUrl:          "https://example.com/selfie.jpg",
		VerificationStatus: domain.VerificationPending,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	data, err := suite.customerRepository.CreateCustomer(suite.ctx, &customer)

	assert.NoError(suite.T(), err)
	assert.NotZero(suite.T(), data.ID)

	var savedCustomer model.Customer
	err = suite.db.First(&savedCustomer, data.ID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), data.NIK, savedCustomer.NIK)
	assert.Equal(suite.T(), data.FullName, savedCustomer.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByID_Success() {
	customerModel := model.Customer{
		NIK:                "1234567890123456",
		FullName:           "John Doe",
		LegalName:          "John Doe",
		Password:           "johndoe123",
		Role:               "customer",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpPhotoUrl:        "https://example.com/ktp.jpg",
		SelfiePhotoUrl:     "https://example.com/selfie.jpg",
		VerificationStatus: model.VerificationPending,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	err := suite.db.Create(&customerModel).Error
	require.NoError(suite.T(), err)

	result, err := suite.customerRepository.FindByID(suite.ctx, customerModel.ID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), customerModel.ID, result.ID)
	assert.Equal(suite.T(), customerModel.NIK, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByID_NotFound() {
	result, err := suite.customerRepository.FindByID(suite.ctx, 999999)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_Success_WithoutLock() {
	nik := "1234567890123456"
	customerModel := model.Customer{
		NIK:                nik,
		FullName:           "John Doe",
		LegalName:          "John Doe",
		Password:           "johndoe123",
		Role:               "customer",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpPhotoUrl:        "https://example.com/ktp.jpg",
		SelfiePhotoUrl:     "https://example.com/selfie.jpg",
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	err := suite.db.Create(&customerModel).Error
	require.NoError(suite.T(), err)

	result, err := suite.customerRepository.FindByNIK(suite.ctx, nik)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), nik, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_Success_WithLock() {
	nik := "1234567890123456"
	customerModel := model.Customer{
		NIK:                nik,
		FullName:           "John Doe",
		LegalName:          "John Doe",
		Password:           "johndoe123",
		Role:               "customer",
		BirthPlace:         "Bandung",
		BirthDate:          time.Date(1992, 5, 15, 0, 0, 0, 0, time.UTC),
		Salary:             7000000,
		KtpPhotoUrl:        "https://example.com/ktp2.jpg",
		SelfiePhotoUrl:     "https://example.com/selfie2.jpg",
		VerificationStatus: model.VerificationVerified,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	err := suite.db.Create(&customerModel).Error
	require.NoError(suite.T(), err)

	result, err := suite.customerRepository.FindByNIKWithLock(suite.ctx, nik)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), nik, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_NotFound() {
	result, err := suite.customerRepository.FindByNIK(suite.ctx, "nonexistent")

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_Success_WithoutFilter() {
	customers := []model.Customer{
		{
			NIK:                "1111111111111111",
			FullName:           "John Doe",
			LegalName:          "John Doe",
			Password:           "johndoe123",
			Role:               "customer",
			BirthPlace:         "Jakarta",
			BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             5000000,
			VerificationStatus: model.VerificationPending,
			CreatedAt:          time.Now().Add(-2 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "2222222222222222",
			FullName:           "Jane Doe",
			LegalName:          "Jane Doe",
			Password:           "janedoe123",
			Role:               "customer",
			BirthPlace:         "Bandung",
			BirthDate:          time.Date(1991, 2, 2, 0, 0, 0, 0, time.UTC),
			Salary:             6000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now().Add(-1 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "3333333333333333",
			FullName:           "John Smith",
			LegalName:          "John Smith",
			Password:           "johnsmith123",
			Role:               "customer",
			BirthPlace:         "Surabaya",
			BirthDate:          time.Date(1992, 3, 3, 0, 0, 0, 0, time.UTC),
			Salary:             7000000,
			VerificationStatus: model.VerificationRejected,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
	}

	for _, customer := range customers {
		err := suite.db.Create(&customer).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Page:  1,
		Limit: 2,
	}

	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), total)
	assert.Len(suite.T(), result, 2)

	assert.Equal(suite.T(), "John Smith", result[0].FullName)
	assert.Equal(suite.T(), "Jane Doe", result[1].FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_Success_WithStatusFilter() {
	customers := []model.Customer{
		{
			NIK:                "1111111111111111",
			FullName:           "John Doe",
			LegalName:          "John Doe",
			Password:           "johndoe123",
			Role:               "customer",
			BirthPlace:         "Jakarta",
			BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             5000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now().Add(-1 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "2222222222222222",
			FullName:           "Jane Doe",
			LegalName:          "Jane Doe",
			Password:           "janedoe123",
			Role:               "customer",
			BirthPlace:         "Bandung",
			BirthDate:          time.Date(1991, 2, 2, 0, 0, 0, 0, time.UTC),
			Salary:             6000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "3333333333333333",
			FullName:           "John Smith",
			LegalName:          "John Smith",
			Password:           "johnsmith123",
			Role:               "customer",
			BirthPlace:         "Surabaya",
			BirthDate:          time.Date(1992, 3, 3, 0, 0, 0, 0, time.UTC),
			Salary:             7000000,
			VerificationStatus: model.VerificationPending,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
	}

	for _, customer := range customers {
		err := suite.db.Create(&customer).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Status: string(domain.VerificationVerified),
		Page:   1,
		Limit:  10,
	}

	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), total)
	assert.Len(suite.T(), result, 2)
	for _, customer := range result {
		assert.Equal(suite.T(), domain.VerificationVerified, customer.VerificationStatus)
	}
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_EmptyResult() {
	params := domain.Params{
		Status: string(domain.VerificationVerified),
		Page:   1,
		Limit:  10,
	}

	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), total)
	assert.Len(suite.T(), result, 0)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_SecondPage() {
	customers := make([]model.Customer, 5)
	for i := range 5 {
		customers[i] = model.Customer{
			NIK:                fmt.Sprintf("111111111111111%d", i),
			FullName:           fmt.Sprintf("Customer %d", i+1),
			LegalName:          fmt.Sprintf("Customer %d Legal", i+1),
			Password:           fmt.Sprintf("customer%d%d%d", i, i+1, i+2),
			Role:               "customer",
			BirthPlace:         "Jakarta",
			BirthDate:          time.Date(1990+i, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             float64(5000000 + i*1000000),
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now().Add(time.Duration(-i) * time.Hour),
			UpdatedAt:          time.Now(),
		}
		err := suite.db.Create(&customers[i]).Error
		require.NoError(suite.T(), err)
	}

	params := domain.Params{
		Page:  2,
		Limit: 2,
	}

	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), total)
	assert.Len(suite.T(), result, 2)
}

func TestCustomerRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(CustomerRepositoryTestSuite))
}
