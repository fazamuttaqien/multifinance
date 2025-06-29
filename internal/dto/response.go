package dto

type LoginResponse struct {
	Token string `json:"token"`
}

type LimitDetailResponse struct {
	TenorMonths    uint8   `json:"tenor_months"`
	LimitAmount    float64 `json:"limit_amount"`
	UsedAmount     float64 `json:"used_amount"`
	RemainingLimit float64 `json:"remaining_limit"`
}

type CheckLimitResponse struct {
	Status         string  `json:"status"`
	Message        string  `json:"message"`
	RemainingLimit float64 `json:"remaining_limit,omitempty"`
}
