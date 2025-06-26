package repositories

import (
	"context"

	"github.com/fazamuttaqien/xyz-multifinance/databases"
	"github.com/fazamuttaqien/xyz-multifinance/models"
)

type transactionRepository struct {
	db *databases.Database
}

// SumActivePrincipalByCustomerIDAndTenorID implements TransactionRepository.
func (t *transactionRepository) SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error) {
	var totalUsed float64
	err := t.db.Gorm.WithContext(ctx).Model(&models.Transaction{}).
		Where("customer_id = ? AND tenor_id = ? AND status = ?", customerID, tenorID, models.TransactionActive).
		Select("COALESCE(SUM(otr_amount + admin_fee), 0)").
		Row().
		Scan(&totalUsed)
	if err != nil {
		return 0, err
	}

	return totalUsed, nil
}

func NewTransactionRepository(db *databases.Database) TransactionRepository {
	return &transactionRepository{
		db: db,
	}
}
