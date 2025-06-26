package dtos

type LimitDetailResponse struct {
	TotalLimit     float64 `json:"total_limit"`
	UsedAmount     float64 `json:"used_amount"`
	RemainingLimit float64 `json:"remaining_limit"`
}
