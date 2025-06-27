package model

import (
	"github.com/fazamuttaqien/multifinance/domain"
)

func CustomerFromEntity(data *domain.Customer) Customer {
	return Customer{
		NIK:                data.NIK,
		FullName:           data.FullName,
		LegalName:          data.LegalName,
		BirthPlace:         data.BirthPlace,
		BirthDate:          data.BirthDate,
		Salary:             data.Salary,
		KTPPhotoURL:        data.KtpUrl,
		SelfiePhotoURL:     data.SelfieUrl,
		VerificationStatus: VerificationStatus(data.VerificationStatus),
	}
}

func CustomerToEntity(data Customer) *domain.Customer {
	return &domain.Customer{
		NIK:                data.NIK,
		FullName:           data.FullName,
		LegalName:          data.LegalName,
		BirthPlace:         data.BirthPlace,
		BirthDate:          data.BirthDate,
		Salary:             data.Salary,
		KtpUrl:             data.KTPPhotoURL,
		SelfieUrl:          data.SelfiePhotoURL,
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
			KtpUrl:             c.KTPPhotoURL,
			SelfieUrl:          c.SelfiePhotoURL,
			VerificationStatus: domain.VerificationStatus(c.VerificationStatus),
		}
	}

	return responses
}
