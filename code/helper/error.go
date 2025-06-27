package helper

import "errors"

var (
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrTenorNotFound      = errors.New("tenor not found")
	ErrLimitNotSet        = errors.New("limit for this tenor is not set for the customer")
	ErrInvalidLimitAmount = errors.New("limit amount cannot be negative")
	ErrInsufficientLimit  = errors.New("insufficient limit for this transaction")
)
