package service_test

import (
	"testing"

	"github.com/fazamuttaqien/multifinance/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = model.AutoMigrate(db)
	assert.NoError(t, err)

	return db
}
