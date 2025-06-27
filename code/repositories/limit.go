package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type limitRepository struct {
	db *gorm.DB
}

// FindAllByCustomerID implements LimitRepository.
func (l *limitRepository) FindAllByCustomerID(ctx context.Context, customerID uint64) ([]models.CustomerLimit, error) {
	var limits []models.CustomerLimit
	err := l.db.WithContext(ctx).Where("customer_id = ?", customerID).Find(&limits).Error
	return limits, err
}

// UpsertMany implements LimitRepository.
func (l *limitRepository) UpsertMany(ctx context.Context, limits []models.CustomerLimit) error {
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
func (l *limitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*models.CustomerLimit, error) {
	var limit models.CustomerLimit
	if err := l.db.WithContext(ctx).Where("customer_id = ? AND tenor_id = ?", customerID, tenorID).First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &limit, nil
}

func NewLimitRepository(db *gorm.DB) LimitRepository {
	return &limitRepository{
		db: db,
	}
}
