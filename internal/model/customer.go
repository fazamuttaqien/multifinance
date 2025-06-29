package model

import (
	"github.com/fazamuttaqien/multifinance/internal/domain"
)

func CustomerFromEntity(data *domain.Customer) Customer {
	return Customer{
		ID:                 data.ID,
		NIK:                data.NIK,
		FullName:           data.FullName,
		LegalName:          data.LegalName,
		BirthPlace:         data.BirthPlace,
		BirthDate:          data.BirthDate,
		Salary:             data.Salary,
		KtpPhotoUrl:        data.KtpUrl,
		SelfiePhotoUrl:     data.SelfieUrl,
		VerificationStatus: VerificationStatus(data.VerificationStatus),
	}
}

func CustomerToEntity(data Customer) *domain.Customer {
	return &domain.Customer{
		ID:                 data.ID,
		NIK:                data.NIK,
		FullName:           data.FullName,
		LegalName:          data.LegalName,
		BirthPlace:         data.BirthPlace,
		BirthDate:          data.BirthDate,
		Salary:             data.Salary,
		KtpUrl:             data.KtpPhotoUrl,
		SelfieUrl:          data.SelfiePhotoUrl,
		VerificationStatus: domain.VerificationStatus(data.VerificationStatus),
	}
}

func CustomersToEntity(data []Customer) []domain.Customer {
	responses := make([]domain.Customer, len(data))
	for i, c := range data {
		responses[i] = domain.Customer{
			ID:                 c.ID,
			NIK:                c.NIK,
			FullName:           c.FullName,
			LegalName:          c.LegalName,
			BirthPlace:         c.BirthPlace,
			BirthDate:          c.BirthDate,
			Salary:             c.Salary,
			KtpUrl:             c.KtpPhotoUrl,
			SelfieUrl:          c.SelfiePhotoUrl,
			VerificationStatus: domain.VerificationStatus(c.VerificationStatus),
		}
	}

	return responses
}