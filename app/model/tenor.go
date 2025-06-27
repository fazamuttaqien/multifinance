package model

import (
	"github.com/fazamuttaqien/multifinance/domain"
)

func TenorToEntity(data Tenor) *domain.Tenor {
	return &domain.Tenor{
		DurationMonths: data.DurationMonths,
		Description:    data.Description,
	}
}

func TenorsToEntity(data []Tenor) []domain.Tenor {
	responses := make([]domain.Tenor, len(data))
	for i, c := range data {
		responses[i] = domain.Tenor{
			DurationMonths: c.DurationMonths,
			Description:    c.Description,
		}
	}

	return responses
}
