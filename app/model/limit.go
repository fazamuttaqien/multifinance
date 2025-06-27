package model

import (
	"github.com/fazamuttaqien/multifinance/domain"
)

func LimitToEntity(data CustomerLimit) *domain.CustomerLimit {
	return &domain.CustomerLimit{
		CustomerID:  data.CustomerID,
		TenorID:     data.TenorID,
		LimitAmount: data.LimitAmount,
	}
}

func LimitsToEntity(data []CustomerLimit) []domain.CustomerLimit {
	responses := make([]domain.CustomerLimit, len(data))
	for i, c := range data {
		responses[i] = domain.CustomerLimit{
			CustomerID:  c.CustomerID,
			TenorID:     c.TenorID,
			LimitAmount: c.LimitAmount,
		}
	}

	return responses
}
