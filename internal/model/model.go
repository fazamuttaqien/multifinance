package model

import (
	"time"

	"gorm.io/gorm"
)

// Customer represents the customers table
type Customer struct {
	ID                 uint64             `gorm:"primaryKey;autoIncrement" json:"id"`
	NIK                string             `gorm:"type:varchar(16);not null;uniqueIndex" json:"nik"`
	FullName           string             `gorm:"type:varchar(255);not null" json:"full_name"`
	LegalName          string             `gorm:"type:varchar(255);not null" json:"legal_name"`
	BirthPlace         string             `gorm:"type:varchar(100);not null" json:"birth_place"`
	BirthDate          time.Time          `gorm:"type:date;not null" json:"birth_date"`
	Salary             float64            `gorm:"type:decimal(15,2);not null" json:"salary"`
	KtpPhotoUrl        string             `gorm:"type:varchar(255);not null" json:"ktp_photo_url"`
	SelfiePhotoUrl     string             `gorm:"type:varchar(255);not null" json:"selfie_photo_url"`
	VerificationStatus VerificationStatus `gorm:"type:enum('PENDING','VERIFIED','REJECTED');default:'PENDING';not null" json:"verification_status"`
	CreatedAt          time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time          `gorm:"autoUpdateTime" json:"updated_at"`

	CustomerLimits []CustomerLimit `gorm:"foreignKey:CustomerID" json:"customer_limits,omitempty"`
	Transactions   []Transaction   `gorm:"foreignKey:CustomerID" json:"transactions,omitempty"`
}

// VerificationStatus enum for customer verification
type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "PENDING"
	VerificationVerified VerificationStatus = "VERIFIED"
	VerificationRejected VerificationStatus = "REJECTED"
)

// Tenor represents the tenors table
type Tenor struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	DurationMonths uint8  `gorm:"not null;uniqueIndex" json:"duration_months"`
	Description    string `gorm:"type:varchar(50)" json:"description"`

	CustomerLimits []CustomerLimit `gorm:"foreignKey:TenorID" json:"customer_limits,omitempty"`
	Transactions   []Transaction   `gorm:"foreignKey:TenorID" json:"transactions,omitempty"`
}

// CustomerLimit represents the customer_limits table
type CustomerLimit struct {
	CustomerID  uint64  `gorm:"primaryKey" json:"customer_id"`
	TenorID     uint    `gorm:"primaryKey" json:"tenor_id"`
	LimitAmount float64 `gorm:"type:decimal(15,2);not null" json:"limit_amount"`

	Customer Customer `gorm:"foreignKey:CustomerID;constraint:OnDelete:CASCADE" json:"customer"`
	Tenor    Tenor    `gorm:"foreignKey:TenorID;constraint:OnDelete:RESTRICT" json:"tenor"`
}

// Transaction represents the transactions table
type Transaction struct {
	ID                     uint64            `gorm:"primaryKey;autoIncrement" json:"id"`
	ContractNumber         string            `gorm:"type:varchar(50);not null;uniqueIndex" json:"contract_number"`
	CustomerID             uint64            `gorm:"not null" json:"customer_id"`
	TenorID                uint              `gorm:"not null" json:"tenor_id"`
	AssetName              string            `gorm:"type:varchar(255);not null" json:"asset_name"`
	OTRAmount              float64           `gorm:"type:decimal(15,2);not null" json:"otr_amount"`
	AdminFee               float64           `gorm:"type:decimal(15,2);not null" json:"admin_fee"`
	TotalInterest          float64           `gorm:"type:decimal(15,2);not null" json:"total_interest"`
	TotalInstallmentAmount float64           `gorm:"type:decimal(15,2);not null" json:"total_installment_amount"`
	Status                 TransactionStatus `gorm:"type:enum('PENDING','APPROVED','ACTIVE','PAID_OFF','CANCELLED');default:'PENDING';not null" json:"status"`
	TransactionDate        time.Time         `gorm:"autoCreateTime" json:"transaction_date"`

	Customer Customer `gorm:"foreignKey:CustomerID;constraint:OnDelete:RESTRICT" json:"customer"`
	Tenor    Tenor    `gorm:"foreignKey:TenorID;constraint:OnDelete:RESTRICT" json:"tenor"`
}

// TransactionStatus enum for transaction status
type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "PENDING"
	TransactionApproved  TransactionStatus = "APPROVED"
	TransactionActive    TransactionStatus = "ACTIVE"
	TransactionPaidOff   TransactionStatus = "PAID_OFF"
	TransactionCancelled TransactionStatus = "CANCELLED"
)

// TableName methods to specify custom table names if needed
func (Customer) TableName() string {
	return "customers"
}

func (Tenor) TableName() string {
	return "tenors"
}

func (CustomerLimit) TableName() string {
	return "customer_limits"
}

func (Transaction) TableName() string {
	return "transactions"
}

// Database migration function
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Customer{},
		&Tenor{},
		&CustomerLimit{},
		&Transaction{},
	)
}
