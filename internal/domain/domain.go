package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

const (
	AdminRole    Role = "admin"
	CustomerRole Role = "customer"
	PartnerRole  Role = "partner"
)

type Customer struct {
	ID                 uint64
	NIK                string
	FullName           string
	LegalName          string
	Password           string
	Role               Role
	BirthPlace         string
	BirthDate          time.Time
	Salary             float64
	KtpUrl             string
	SelfieUrl          string
	VerificationStatus VerificationStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time

	CustomerLimits []CustomerLimit
	Transactions   []Transaction
}

type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "PENDING"
	VerificationVerified VerificationStatus = "VERIFIED"
	VerificationRejected VerificationStatus = "REJECTED"
)

type Tenor struct {
	ID             uint
	DurationMonths uint8
	Description    string

	CustomerLimits []CustomerLimit
	Transactions   []Transaction
}

type CustomerLimit struct {
	CustomerID  uint64
	TenorID     uint
	LimitAmount float64

	Customer Customer
	Tenor    Tenor
}

type Transaction struct {
	ID                     uint64
	ContractNumber         string
	CustomerID             uint64
	TenorID                uint
	AssetName              string
	OTRAmount              float64
	AdminFee               float64
	TotalInterest          float64
	TotalInstallmentAmount float64
	Status                 TransactionStatus
	TransactionDate        time.Time

	Customer Customer
	Tenor    Tenor
}

type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "PENDING"
	TransactionApproved  TransactionStatus = "APPROVED"
	TransactionActive    TransactionStatus = "ACTIVE"
	TransactionPaidOff   TransactionStatus = "PAID_OFF"
	TransactionCancelled TransactionStatus = "CANCELLED"
)

type JwtCustomClaims struct {
	UserID uint64 `json:"user_id"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

type Params struct {
	Status string
	Page   int
	Limit  int
}

type Paginated struct {
	Data       any
	Total      int64
	Page       int
	Limit      int
	TotalPages int
}
