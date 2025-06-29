package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/fazamuttaqien/multifinance/internal/model"
	"github.com/fazamuttaqien/multifinance/internal/repository"
	tenorrepo "github.com/fazamuttaqien/multifinance/internal/repository/tenor"
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

type TenorRepositoryTestSuite struct {
	suite.Suite
	db              *gorm.DB
	ctx             context.Context
	tenorRepository repository.TenorRepository

	testTenors []model.Tenor

	meter  metric.Meter
	tracer trace.Tracer
	log    *zap.Logger
}

func (suite *TenorRepositoryTestSuite) SetupSuite() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		common.GetEnv("MYSQL_USER", "root"),
		common.GetEnv("MYSQL_PASSWORD", "rootpassword123"),
		common.GetEnv("MYSQL_HOST", "127.0.0.1"),
		common.GetEnv("MYSQL_PORT", "3306"),
	)
	sqlDB, err := sql.Open("mysql", dsn)
	require.NoError(suite.T(), err)

	testDBName := "loan_system_tenor_test"
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
	suite.tracer = noopTracerProvider.Tracer("test-tenor-repository-tracer")
	noopMeterProvider := noop_metric.NewMeterProvider()
	suite.meter = noopMeterProvider.Meter("test-tenor-repository-meter")

	err = suite.db.AutoMigrate(&model.Customer{}, &model.Tenor{}, &model.CustomerLimit{}, &model.Transaction{})
	require.NoError(suite.T(), err)

	suite.tenorRepository = tenorrepo.NewTenorRepository(suite.db, suite.meter, suite.tracer, suite.log)
}

func (suite *TenorRepositoryTestSuite) TearDownSuite() {
	testDBName := "loan_system_tenor_test"
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

func (suite *TenorRepositoryTestSuite) SetupTest() {
	// Bersihkan data dari tabel
	suite.db.Exec("DELETE FROM customer_limits")
	suite.db.Exec("DELETE FROM transactions")
	suite.db.Exec("DELETE FROM customers")
	suite.db.Exec("DELETE FROM tenors")

	suite.testTenors = []model.Tenor{
		{DurationMonths: 3, Description: "3 Bulan"},
		{DurationMonths: 6, Description: "6 Bulan"},
		{DurationMonths: 12, Description: "12 Bulan"},
	}
	require.NoError(suite.T(), suite.db.Create(&suite.testTenors).Error)
}

func (suite *TenorRepositoryTestSuite) TestFindAll_Success() {
	result, err := suite.tenorRepository.FindAll(suite.ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Len(suite.T(), result, 3, "Should return all 3 tenors created in setup")

	assert.Equal(suite.T(), uint8(6), result[1].DurationMonths)
	assert.Equal(suite.T(), "6 Bulan", result[1].Description)
}

func (suite *TenorRepositoryTestSuite) TestFindAll_EmptyResult() {
	suite.db.Exec("DELETE FROM tenors")

	result, err := suite.tenorRepository.FindAll(suite.ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result, "Result should be a non-nil empty slice")
	assert.Len(suite.T(), result, 0, "Result should be empty")
}

func (suite *TenorRepositoryTestSuite) TestFindByDuration_Success() {
	result, err := suite.tenorRepository.FindByDuration(suite.ctx, 6)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), uint8(6), result.DurationMonths)
	assert.Equal(suite.T(), "6 Bulan", result.Description)
	assert.Equal(suite.T(), suite.testTenors[1].ID, result.ID, "ID should match the one in the database")
}

func (suite *TenorRepositoryTestSuite) TestFindByDuration_NotFound() {
	result, err := suite.tenorRepository.FindByDuration(suite.ctx, 24)

	assert.NoError(suite.T(), err, "Not found should not be treated as a database error")
	assert.Nil(suite.T(), result, "Result should be nil when the tenor is not found")
}

func TestTenorRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(TenorRepositoryTestSuite))
}
