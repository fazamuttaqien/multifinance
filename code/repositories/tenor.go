package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
)

type tenorRepository struct {
	db *gorm.DB
}

// FindAll implements TenorRepository.
func (t *tenorRepository) FindAll(ctx context.Context) ([]models.Tenor, error) {
	var tenors []models.Tenor
	err := t.db.WithContext(ctx).Find(&tenors).Error
	return tenors, err
}

// FindByDuration implements TenorRepository.
func (t *tenorRepository) FindByDuration(ctx context.Context, durationMonths uint8) (*models.Tenor, error) {
	var tenor models.Tenor
	if err := t.db.WithContext(ctx).Where("duration_months = ?", durationMonths).First(&tenor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenor, nil
}

func NewTenorRepository(db *gorm.DB) TenorRepository {
	return &tenorRepository{
		db: db,
	}
}
