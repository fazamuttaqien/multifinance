package usecases

import (
	"context"
	"fmt"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"github.com/fazamuttaqien/xyz-multifinance/repositories"
)

type partnerUsecase struct {
	customerRepository    repositories.CustomerRepository
	tenorRepository       repositories.TenorRepository
	limitRepository       repositories.LimitRepository
	transactionRepository repositories.TransactionRepository
}

// CheckLimit implements PartnerUsecases.
func (p *partnerUsecase) CheckLimit(ctx context.Context, req dtos.CheckLimitRequest) (*dtos.CheckLimitResponse, error) {
	// 1. Validasi Customer & Tenor
	cust, err := p.customerRepository.FindByNIK(ctx, req.NIK)
	if err != nil {
		return nil, err
	}
	if cust == nil {
		return nil, helper.ErrCustomerNotFound
	}
	if cust.VerificationStatus != models.VerificationVerified {
		return nil, fmt.Errorf("customer %s is not verified", req.NIK)
	}

	tenor, err := p.tenorRepository.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		return nil, err
	}
	if tenor == nil {
		return nil, helper.ErrTenorNotFound
	}

	// 2. Hitung Sisa Limit
	limit, err := p.limitRepository.FindByCustomerIDAndTenorID(ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		return nil, helper.ErrLimitNotSet
	}

	usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}

	remainingLimit := limit.LimitAmount - usedAmount

	// 3. Buat Response
	if remainingLimit >= req.TransactionAmount {
		return &dtos.CheckLimitResponse{
			Status:         "approved",
			Message:        "Limit is sufficient.",
			RemainingLimit: remainingLimit,
		}, nil
	}

	return &dtos.CheckLimitResponse{
		Status:         "rejected",
		Message:        "Insufficient limit for this transaction.",
		RemainingLimit: remainingLimit,
	}, nil
}

func NewService(
	cr repositories.CustomerRepository,
	tr repositories.TenorRepository,
	lr repositories.LimitRepository,
	txr repositories.TransactionRepository,
) PartnerUsecases {
	return &partnerUsecase{
		customerRepository:    cr,
		tenorRepository:       tr,
		limitRepository:       lr,
		transactionRepository: txr,
	}
}
