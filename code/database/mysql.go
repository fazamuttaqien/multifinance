package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	Charset      string
	ParseTime    bool
	Loc          string
}

type Database struct {
	DB     *gorm.DB
	Config *DatabaseConfig
}

// LoadConfigFromEnv Loads database configuration from environment variables
func LoadConfigFromEnv() *DatabaseConfig {
	port, err := strconv.Atoi(helper.GetEnv("DB_PORT", "3306"))
	if err != nil {
		port = 3306
	}

	parseTime, err := strconv.ParseBool(helper.GetEnv("DB_PARSE_TIME", "true"))
	if err != nil {
		parseTime = true
	}

	return &DatabaseConfig{
		Host:         helper.GetEnv("DB_HOST", "localhost"),
		Port:         port,
		Username:     helper.GetEnv("DB_USERNAME", "root"),
		Password:     helper.GetEnv("DB_PASSWORD", ""),
		DatabaseName: helper.GetEnv("DB_NAME", "loan_system"),
		Charset:      helper.GetEnv("DB_CHARSET", "uft8mb4"),
		ParseTime:    parseTime,
		Loc:          helper.GetEnv("DB_LOC", "Local"),
	}
}

// CreateConfig creates database configuration manually
func CreateConfig(host, username, password, dbname string, port int) *DatabaseConfig {
	return &DatabaseConfig{
		Host:         host,
		Port:         port,
		Username:     username,
		Password:     password,
		DatabaseName: dbname,
		Charset:      "utf8mb4",
		ParseTime:    true,
		Loc:          "Local",
	}
}

// BuildDSN builds MySQL DSN (Data Source Name) from config
func (config *DatabaseConfig) BuildDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		config.Username, config.Password, config.Host, config.Port,
		config.DatabaseName, config.Charset, config.ParseTime, config.Loc,
	)
}

// Connect establishes database connection
func Connect(config *DatabaseConfig) (*Database, error) {
	dsn := config.BuildDSN()

	// GORM configuration
	gormConfig := &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,   // Slow SQL threshold
				LogLevel:                  logger.Silent, // Log level
				IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
				Colorful:                  true,          // Enable color
			},
		),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	}

	// Open connection
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &Database{
		DB:     db,
		Config: config,
	}, nil
}

// ConnectWithRetry connects to database with retry mechanism
func ConnectWithRetry(config *DatabaseConfig, maxRetries int, retryDelay time.Duration) (db *Database, err error) {
	for i := range maxRetries {
		db, err := Connect(config)
		if err == nil {
			log.Printf("Successfully connected to database on attempt %d", i+1)
			return db, nil
		}

		log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Close()
}

// Ping checks if database connection is alive
func (d *Database) Ping() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Ping()
}

// GetStats returns database connection statistics
func (d *Database) GetStats() map[string]any {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return map[string]any{
			"error": err.Error(),
		}
	}

	stats := sqlDB.Stats()
	return map[string]any{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
	}
}

// Usage functions

// InitializeDatabase initializes database connection with environment config
func InitializeDatabase() (*Database, error) {
	config := LoadConfigFromEnv()
	return ConnectWithRetry(config, 5, time.Second*2)
}

// InitializeDatabaseWithConfig initializes database with custom config
func InitializeDatabaseWithConfig(host, username, password, dbname string, port int) (*Database, error) {
	config := CreateConfig(host, username, password, dbname, port)
	return Connect(config)
}

// EnableDebugMode enables GORM debug mode for development
func (d *Database) EnableDebugMode() {
	d.DB = d.DB.Debug()
}
