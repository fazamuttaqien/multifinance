package repository_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"github.com/fazamuttaqien/multifinance/repository"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type CustomerRepositoryTestSuite struct {
	suite.Suite
	db                 *gorm.DB
	ctx                context.Context
	container          testcontainers.Container
	meter              metric.Meter
	tracer             trace.Tracer
	log                *zap.Logger
	customerRepository repository.CustomerRepository
}

func (suite *CustomerRepositoryTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	req := testcontainers.ContainerRequest{
		Image: "alpine:latest",
		Cmd:   []string{"tail", "-f", "/dev/null"},
		// Files: []testcontainers.ContainerFile{
		// 	{
		// 		HostFilePath: "/tmp/testdb",
		// 		ContainerFilePath: "/data",
		// 		FileMode: 0644,
		// 	},
		// },
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = []string{"/tmp/testdb:/data"} // bind host to container
		},
		WaitingFor: wait.ForLog(""), // optional: wait strategy
	}

	container, err := testcontainers.GenericContainer(suite.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(suite.T(), err)
	suite.container = container

	// Setup SQLite database connection
	// Use temporary directory for test database
	tmpDir := suite.T().TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(suite.T(), err)

	suite.db = gormDB

	err = suite.db.AutoMigrate(
		&model.Customer{},
		&model.Tenor{},
		&model.Transaction{},
		&model.CustomerLimit{},
	)
	require.NoError(suite.T(), err)

	// Initialize repository
	suite.customerRepository = repository.NewCustomerRepository(suite.db, suite.meter, suite.tracer, suite.log)
}

func (suite *CustomerRepositoryTestSuite) TearDownSuite() {
	// Close database connection
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	// Terminate container
	if suite.container != nil {
		suite.container.Terminate(suite.ctx)
	}
}

func (suite *CustomerRepositoryTestSuite) SetupTest() {
	// Clean up database sebelum setiap test
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")
}

func (suite *CustomerRepositoryTestSuite) TestSave_Success() {
	// Arrange
	customer := &domain.Customer{
		NIK:                "1234567890123456",
		FullName:           "John Doe",
		LegalName:          "John Doe Legal",
		BirthPlace:         "Jakarta",
		BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Salary:             5000000,
		KtpUrl:             "https://example.com/ktp.jpg",
		SelfieUrl:          "https://example.com/selfie.jpg",
		VerificationStatus: domain.VerificationPending,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Act
	err := suite.customerRepository.CreateCustomer(suite.ctx, customer)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotZero(suite.T(), customer.ID)

	// Verify data tersimpan di database
	var savedCustomer model.Customer
	err = suite.db.First(&savedCustomer, customer.ID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), customer.NIK, savedCustomer.NIK)
	assert.Equal(suite.T(), customer.FullName, savedCustomer.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByID_Success() {
	// Arrange
	customerModel := model.Customer{
		NIK:                "1234567890123456",
		FullName:           "John Doe",
		LegalName:          "John Doe Legal",
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

	// Act
	result, err := suite.customerRepository.FindByID(suite.ctx, customerModel.ID)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), customerModel.ID, result.ID)
	assert.Equal(suite.T(), customerModel.NIK, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByID_NotFound() {
	// Act
	result, err := suite.customerRepository.FindByID(suite.ctx, 999999)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_Success_WithoutLock() {
	// Arrange
	nik := "1234567890123456"
	customerModel := model.Customer{
		NIK:                nik,
		FullName:           "John Doe",
		LegalName:          "John Doe Legal",
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

	// Act
	result, err := suite.customerRepository.FindByNIK(suite.ctx, nik)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), nik, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_Success_WithLock() {
	// Arrange
	nik := "1234567890123456"
	customerModel := model.Customer{
		NIK:                nik,
		FullName:           "Jane Doe",
		LegalName:          "Jane Doe Legal",
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

	// Act
	result, err := suite.customerRepository.FindByNIKWithLock(suite.ctx, nik)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), nik, result.NIK)
	assert.Equal(suite.T(), customerModel.FullName, result.FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindByNIK_NotFound() {
	// Act
	result, err := suite.customerRepository.FindByNIK(suite.ctx, "nonexistent")

	// Assert
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_Success_WithoutFilter() {
	// Arrange
	customers := []model.Customer{
		{
			NIK:                "1111111111111111",
			FullName:           "Customer 1",
			LegalName:          "Customer 1 Legal",
			BirthPlace:         "Jakarta",
			BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             5000000,
			VerificationStatus: model.VerificationPending,
			CreatedAt:          time.Now().Add(-2 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "2222222222222222",
			FullName:           "Customer 2",
			LegalName:          "Customer 2 Legal",
			BirthPlace:         "Bandung",
			BirthDate:          time.Date(1991, 2, 2, 0, 0, 0, 0, time.UTC),
			Salary:             6000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now().Add(-1 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "3333333333333333",
			FullName:           "Customer 3",
			LegalName:          "Customer 3 Legal",
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

	// Act
	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), total)
	assert.Len(suite.T(), result, 2)
	// Verify ordering (newest first)
	assert.Equal(suite.T(), "Customer 3", result[0].FullName)
	assert.Equal(suite.T(), "Customer 2", result[1].FullName)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_Success_WithStatusFilter() {
	// Arrange
	customers := []model.Customer{
		{
			NIK:                "1111111111111111",
			FullName:           "Customer 1",
			LegalName:          "Customer 1 Legal",
			BirthPlace:         "Jakarta",
			BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Salary:             5000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now().Add(-1 * time.Hour),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "2222222222222222",
			FullName:           "Customer 2",
			LegalName:          "Customer 2 Legal",
			BirthPlace:         "Bandung",
			BirthDate:          time.Date(1991, 2, 2, 0, 0, 0, 0, time.UTC),
			Salary:             6000000,
			VerificationStatus: model.VerificationVerified,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			NIK:                "3333333333333333",
			FullName:           "Customer 3",
			LegalName:          "Customer 3 Legal",
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

	// Act
	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), total)
	assert.Len(suite.T(), result, 2)
	// Verify all results have VERIFIED status
	for _, customer := range result {
		assert.Equal(suite.T(), domain.VerificationVerified, customer.VerificationStatus)
	}
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_EmptyResult() {
	// Arrange
	params := domain.Params{
		Status: string(domain.VerificationVerified),
		Page:   1,
		Limit:  10,
	}

	// Act
	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), total)
	assert.Len(suite.T(), result, 0)
}

func (suite *CustomerRepositoryTestSuite) TestFindPaginated_SecondPage() {
	// Arrange
	customers := make([]model.Customer, 5)
	for i := 0; i < 5; i++ {
		customers[i] = model.Customer{
			NIK:                fmt.Sprintf("111111111111111%d", i),
			FullName:           fmt.Sprintf("Customer %d", i+1),
			LegalName:          fmt.Sprintf("Customer %d Legal", i+1),
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

	// Act
	result, total, err := suite.customerRepository.FindPaginated(suite.ctx, params)

	// Assert
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), total)
	assert.Len(suite.T(), result, 2)
}

// Test runner function
func TestCustomerRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(CustomerRepositoryTestSuite))
}

// Benchmark tests
// func BenchmarkCustomerRepository_Save(b *testing.B) {
// 	// Setup database untuk benchmark
// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/multifinance_test?charset=utf8mb4&parseTime=True&loc=Local",
// 		common.GetEnv("DB_USER", "root"),
// 		common.GetEnv("DB_PASSWORD", ""),
// 		common.GetEnv("DB_HOST", "localhost"),
// 		common.GetEnv("DB_PORT", "3306"),
// 	)

// 	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
// 		Logger: logger.Default.LogMode(logger.Silent),
// 	})
// 	if err != nil {
// 		b.Fatal(err)
// 	}

// 	repo := repository.NewCustomerRepository(db)
// 	ctx := context.Background()

// 	b.ResetTimer()
// 	b.RunParallel(func(pb *testing.PB) {
// 		i := 0
// 		for pb.Next() {
// 			customer := &domain.Customer{
// 				NIK:                fmt.Sprintf("BENCH%016d", i),
// 				FullName:           fmt.Sprintf("Benchmark User %d", i),
// 				LegalName:          fmt.Sprintf("Benchmark User %d Legal", i),
// 				BirthPlace:         "Jakarta",
// 				BirthDate:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
// 				Salary:             5000000,
// 				VerificationStatus: domain.VerificationPending,
// 				CreatedAt:          time.Now(),
// 				UpdatedAt:          time.Now(),
// 			}
// 			repo.CreateCustomer(ctx, customer)
// 			i++
// 		}
// 	})
// }
