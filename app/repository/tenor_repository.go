package repository

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"
)

type tenorRepository struct {
	db *gorm.DB
}

// FindAll implements TenorRepository.
func (t *tenorRepository) FindAll(ctx context.Context) ([]domain.Tenor, error) {
	var tenors []model.Tenor
	err := t.db.WithContext(ctx).Find(&tenors).Error

	return model.TenorsToEntity(tenors), err
}

// FindByDuration implements TenorRepository.
func (t *tenorRepository) FindByDuration(ctx context.Context, durationMonths uint8) (*domain.Tenor, error) {
	var tenor model.Tenor
	if err := t.db.WithContext(ctx).Where("duration_months = ?", durationMonths).First(&tenor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return model.TenorToEntity(tenor), nil
}

func NewTenorRepository(db *gorm.DB) TenorRepository {
	return &tenorRepository{
		db: db,
	}
}
