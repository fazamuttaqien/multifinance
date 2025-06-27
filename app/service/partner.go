package service

import (
	"context"
	"fmt"

	"github.com/fazamuttaqien/multifinance/domain"
	"github.com/fazamuttaqien/multifinance/dto"
	var_error "github.com/fazamuttaqien/multifinance/helper/error"
	"github.com/fazamuttaqien/multifinance/repository"
)

type partnerService struct {
	customerRepository    repository.CustomerRepository
	tenorRepository       repository.TenorRepository
	limitRepository       repository.LimitRepository
	transactionRepository repository.TransactionRepository
}

// CheckLimit implements PartnerUsecases.
func (p *partnerService) CheckLimit(ctx context.Context, req dto.CheckLimitRequest) (*dto.CheckLimitResponse, error) {
	// 1. Validasi Customer & Tenor
	cust, err := p.customerRepository.FindByNIK(ctx, req.NIK, false)
	if err != nil {
		return nil, err
	}
	if cust == nil {
		return nil, var_error.ErrCustomerNotFound
	}
	if cust.VerificationStatus != domain.VerificationVerified {
		return nil, fmt.Errorf("customer %s is not verified", req.NIK)
	}

	tenor, err := p.tenorRepository.FindByDuration(ctx, req.TenorMonths)
	if err != nil {
		return nil, err
	}
	if tenor == nil {
		return nil, var_error.ErrTenorNotFound
	}

	// 2. Hitung Sisa Limit
	limit, err := p.limitRepository.FindByCustomerIDAndTenorID(ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}
	if limit == nil {
		return nil, var_error.ErrLimitNotSet
	}

	usedAmount, err := p.transactionRepository.SumActivePrincipalByCustomerIDAndTenorID(
		ctx, cust.ID, tenor.ID)
	if err != nil {
		return nil, err
	}

	remainingLimit := limit.LimitAmount - usedAmount

	// 3. Buat Response
	if remainingLimit >= req.TransactionAmount {
		return &dto.CheckLimitResponse{
			Status:         "approved",
			Message:        "Limit is sufficient.",
			RemainingLimit: remainingLimit,
		}, nil
	}

	return &dto.CheckLimitResponse{
		Status:         "rejected",
		Message:        "Insufficient limit for this transaction.",
		RemainingLimit: remainingLimit,
	}, nil
}

func NewPartnetService(
	customerRepository repository.CustomerRepository,
	tenorRepository repository.TenorRepository,
	limitRepository repository.LimitRepository,
	transactionRepository repository.TransactionRepository,
) PartnerServices {
	return &partnerService{
		customerRepository:    customerRepository,
		tenorRepository:       tenorRepository,
		limitRepository:       limitRepository,
		transactionRepository: transactionRepository,
	}
}
