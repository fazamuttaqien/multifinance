package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
)

type tenorRepository struct {
	db *databases.Database
}

// FindByDuration implements TenorRepository.
func (t *tenorRepository) FindByDuration(ctx context.Context, durationMonths uint8) (*models.Tenor, error) {
	var tenor models.Tenor
	if err := t.db.Gorm.WithContext(ctx).Where("duration_months = ?", durationMonths).First(&tenor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenor, nil
}

func NewTenorRepository(db *databases.Database) TenorRepository {
	return &tenorRepository{
		db: db,
	}
}
