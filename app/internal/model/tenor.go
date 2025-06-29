package model

import (
	"github.com/fazamuttaqien/multifinance/internal/domain"
)

func TenorToEntity(data Tenor) *domain.Tenor {
	return &domain.Tenor{
		ID:             data.ID,
		DurationMonths: data.DurationMonths,
		Description:    data.Description,
	}
}

func TenorsToEntity(data []Tenor) []domain.Tenor {
	responses := make([]domain.Tenor, len(data))
	for i, c := range data {
		responses[i] = domain.Tenor{
			ID:             c.ID,
			DurationMonths: c.DurationMonths,
			Description:    c.Description,
		}
	}

	return responses
}
