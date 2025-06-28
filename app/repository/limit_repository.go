package repository

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type limitRepository struct {
	db *gorm.DB
}

// FindAllByCustomerID implements LimitRepository.
func (l *limitRepository) FindAllByCustomerID(ctx context.Context, customerID uint64) ([]domain.CustomerLimit, error) {
	var limits []model.CustomerLimit
	err := l.db.WithContext(ctx).Where("customer_id = ?", customerID).Find(&limits).Error

	return model.LimitsToEntity(limits), err
}

// UpsertMany implements LimitRepository.
func (l *limitRepository) UpsertMany(ctx context.Context, limits []domain.CustomerLimit) error {
	if len(limits) == 0 {
		return nil
	}

	// Menggunakan OnConflict untuk melakukan UPSERT
	// Jika terdapat konflik pada composite primary key (customer_id, tenor_id),
	// perbarui kolom 'limit_amount'
	return l.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "customer_id"}, {Name: "tenor_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"limit_amount"}),
	}).Create(&limits).Error
}

// FindByCustomerIDAndTenorID implements LimitRepository.
func (l *limitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*domain.CustomerLimit, error) {
	var limit model.CustomerLimit
	if err := l.db.WithContext(ctx).Where("customer_id = ? AND tenor_id = ?", customerID, tenorID).First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return model.LimitToEntity(limit), nil
}

func NewLimitRepository(db *gorm.DB) LimitRepository {
	return &limitRepository{
		db: db,
	}
}
