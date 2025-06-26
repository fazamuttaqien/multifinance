package repositories

import (
	"context"
	"errors"

	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"gorm.io/gorm"
)

type limitRepository struct {
	db *databases.Database
}

// FindByCustomerIDAndTenorID implements LimitRepository.
func (l *limitRepository) FindByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (*models.CustomerLimit, error) {
	var limit models.CustomerLimit
	if err := l.db.Gorm.WithContext(ctx).Where("customer_id = ? AND tenor_id = ?", customerID, tenorID).First(&limit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &limit, nil
}

func NewLimitRepository(db *databases.Database) LimitRepository {
	return &limitRepository{
		db: db,
	}
}
