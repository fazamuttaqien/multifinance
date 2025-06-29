package model

import (
	"github.com/fazamuttaqien/multifinance/internal/domain"
)

func TransactionFromEntity(data *domain.Transaction) Transaction {
	return Transaction{
		ID:                     data.ID,
		ContractNumber:         data.ContractNumber,
		CustomerID:             data.CustomerID,
		TenorID:                data.TenorID,
		AssetName:              data.AssetName,
		OTRAmount:              data.OTRAmount,
		AdminFee:               data.AdminFee,
		TotalInterest:          data.TotalInterest,
		TotalInstallmentAmount: data.TotalInstallmentAmount,
		Status:                 TransactionStatus(data.Status),
		TransactionDate:        data.TransactionDate,
	}
}

func TransactionToEntity(data Transaction) *domain.Transaction {
	return &domain.Transaction{
		ID:                     data.ID,
		ContractNumber:         data.ContractNumber,
		CustomerID:             data.CustomerID,
		TenorID:                data.TenorID,
		AssetName:              data.AssetName,
		OTRAmount:              data.OTRAmount,
		AdminFee:               data.AdminFee,
		TotalInterest:          data.TotalInterest,
		TotalInstallmentAmount: data.TotalInstallmentAmount,
		Status:                 domain.TransactionStatus(data.Status),
		TransactionDate:        data.TransactionDate,
	}
}

func TransactionsToEntity(data []Transaction) []domain.Transaction {
	responses := make([]domain.Transaction, len(data))
	for i, t := range data {
		responses[i] = domain.Transaction{
			ID:                     t.ID,
			ContractNumber:         t.ContractNumber,
			CustomerID:             t.CustomerID,
			TenorID:                t.TenorID,
			AssetName:              t.AssetName,
			OTRAmount:              t.OTRAmount,
			AdminFee:               t.AdminFee,
			TotalInterest:          t.TotalInterest,
			TotalInstallmentAmount: t.TotalInstallmentAmount,
			Status:                 domain.TransactionStatus(t.Status),
			TransactionDate:        t.TransactionDate,
		}
	}

	return responses
}
