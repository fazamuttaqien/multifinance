package dtos

import "github.com/fazamuttaqien/xyz-multifinance/models"

type LimitDetailResponse struct {
	TenorMonths    uint8   `json:"tenor_months"`
	LimitAmount    float64 `json:"limit_amount"`
	UsedAmount     float64 `json:"used_amount"`
	RemainingLimit float64 `json:"remaining_limit"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

type CheckLimitResponse struct {
	Status         string  `json:"status"` // "approved" atau "rejected"
	Message        string  `json:"message"`
	RemainingLimit float64 `json:"remaining_limit,omitempty"`
}

type VerificationRequest struct {
	Status models.VerificationStatus `json:"status" validate:"required,oneof=VERIFIED REJECTED"`
	Reason string                    `json:"reason,omitempty"`
}

type CustomerQueryParams struct {
	Status string `query:"status"` // PENDING, VERIFIED, REJECTED
	Page   int    `query:"page"`
	Limit  int    `query:"limit"`
}
