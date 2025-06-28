package repository

import (
	"context"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/model"
	"gorm.io/gorm"
)

type transactionRepository struct {
	db *gorm.DB
}

// FindPaginationByCustomerID implements TransactionRepository.
func (t *transactionRepository) FindPaginatedByCustomerID(ctx context.Context, customerID uint64, params domain.Params) ([]domain.Transaction, int64, error) {
	var transactions []model.Transaction
	var total int64

	// Buat query dasar
	query := t.db.WithContext(ctx).Model(&model.Transaction{}).Where("customer_id = ?", customerID)
	countQuery := t.db.WithContext(ctx).Model(&model.Transaction{}).Where("customer_id = ?", customerID)

	// Terapkan filter status jika ada
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
		countQuery = countQuery.Where("status = ?", params.Status)
	}

	// Hitung total record (sebelum limit dan offset)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Terapkan paginasi
	offset := (params.Page - 1) * params.Limit
	query = query.Limit(params.Limit).Offset(offset).Order("transaction_date DESC")

	if err := query.Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return model.TransactionsToEntity(transactions), total, nil
}

// CreateTransaction implements TransactionRepository.
func (t *transactionRepository) CreateTransaction(ctx context.Context, transaction *domain.Transaction) error {
	data := model.TransactionFromEntity(transaction)
	return t.db.WithContext(ctx).Create(&data).Error
}

// SumActivePrincipalByCustomerIDAndTenorID implements TransactionRepository.
func (t *transactionRepository) SumActivePrincipalByCustomerIDAndTenorID(ctx context.Context, customerID uint64, tenorID uint) (float64, error) {
	var totalUsed float64
	err := t.db.WithContext(ctx).Model(&model.Transaction{}).
		Where("customer_id = ? AND tenor_id = ? AND status = ?", customerID, tenorID, model.TransactionActive).
		Select("COALESCE(SUM(otr_amount + admin_fee), 0)").
		Row().
		Scan(&totalUsed)
	if err != nil {
		return 0, err
	}

	return totalUsed, nil
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{
		db: db,
	}
}
