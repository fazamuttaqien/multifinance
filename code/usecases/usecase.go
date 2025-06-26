package usecases

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fazamuttaqien/xyz-multifinance/dtos"
	"github.com/fazamuttaqien/xyz-multifinance/helper"
	"github.com/fazamuttaqien/xyz-multifinance/models"
	"github.com/fazamuttaqien/xyz-multifinance/repositories"
	"gorm.io/gorm"
)

type usecase struct {
	db          *gorm.DB
	customer    repositories.CustomerRepository
	tenor       repositories.TenorRepository
	limit       repositories.LimitRepository
	transaction repositories.TransactionRepository
	media       Media
}

// SetLimits implements Usecases.
func (cu *usecase) SetLimits(ctx context.Context, customerID uint64, req dtos.SetLimitsRequest) error {
	// Start transaction
	tx := cu.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	// 1. Validasi customer
	customerTx := repositories.NewCustomerRepository(tx)
	customer, err := customerTx.FindByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("error finding customer: %w", err)
	}
	if customer == nil {
		return helper.ErrCustomerNotFound
	}

	limitsToUpsert := make([]models.CustomerLimit, 0, len(req.Limits))
	tenorTx := repositories.NewTenorRepository(tx)

	// 2. Loop dan validasi setiap item limit dalam request
	for _, item := range req.Limits {
		if item.LimitAmount < 0 {
			return helper.ErrInvalidLimitAmount
		}

		// Cari tenor ID berdasarkan durasi bulan
		tenor, err := tenorTx.FindByDuration(ctx, item.TenorMonths)
		if err != nil {
			return fmt.Errorf("error finding tenor for %d months: %w", item.TenorMonths, err)
		}
		if tenor == nil {
			return fmt.Errorf("%w: for %d months", helper.ErrTenorNotFound, item.TenorMonths)
		}

		// Menyiapkan data untuk di upsert
		limitsToUpsert = append(limitsToUpsert, models.CustomerLimit{
			CustomerID:  customerID,
			TenorID:     tenor.ID,
			LimitAmount: item.LimitAmount,
		})
	}

	// 3. Melakukan operasi upsert massal
	if len(limitsToUpsert) > 0 {
		limitTx := repositories.NewLimitRepository(tx)
		if err := limitTx.UpsertMany(ctx, limitsToUpsert); err != nil {
			return fmt.Errorf("failed to upsert limits: %w", err)
		}
	}

	// 4. Jika semua berhasil, commit transaksi
	return tx.Commit().Error
}

// CalculateLimit implements Usecases.
func (cu *usecase) CalculateLimit(ctx context.Context, customerID uint64, tenorMonths uint8) (*dtos.LimitDetailResponse, error) {
	// 1. Validasi customer
	customer, err := cu.customer.FindByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("error finding customer: %w", err)
	}
	if customer == nil {
		return nil, helper.ErrCustomerNotFound
	}

	// 2. Dapatkan ID Tenor dari durasi bulan
	tenor, err := cu.tenor.FindByDuration(ctx, tenorMonths)
	if err != nil {
		return nil, fmt.Errorf("error finding tenor: %w", err)
	}
	if tenor == nil {
		return nil, helper.ErrTenorNotFound
	}

	// 3. Mendapatkan total limit yang ditetapkan untuk customer & tenor
	limit, err := cu.limit.FindByCustomerIDAndTenorID(ctx, customerID, tenor.ID)
	if err != nil {
		return nil, fmt.Errorf("error finding limit: %w", err)
	}
	if limit == nil {
		// Jika limit tidak di-set, anggap 0. Ini keputusan bisnis.
		return nil, helper.ErrLimitNotSet
	}
	totalLimit := limit.LimitAmount

	// 4. Hitung jumlah pemakaian (used amount) dari transaksi aktif
	usedAmount, err := cu.transaction.SumActivePrincipalByCustomerIDAndTenorID(ctx, customerID, tenor.ID)
	if err != nil {
		return nil, fmt.Errorf("error calculating used amount: %w", err)
	}

	// 5. Kalkulasi sisa limit
	remainigLimit := totalLimit - usedAmount
	if remainigLimit < 0 {
		remainigLimit = 0
	}

	response := &dtos.LimitDetailResponse{
		TotalLimit:     totalLimit,
		UsedAmount:     usedAmount,
		RemainingLimit: remainigLimit,
	}

	return response, nil
}

// Register implements CustomerUsecase.
func (cu *usecase) Register(ctx context.Context, req *dtos.CustomerRegister) (*models.Customer, error) {
	// 1. Cek duplikasi NIK
	existingCustomer, err := cu.customer.FindByNIK(ctx, req.NIK)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existingCustomer != nil {
		return nil, errors.New("customer already registered")
	}

	// 2. Upload gambar ke Cloudinary secara bersamaan (concurrent)
	var wg sync.WaitGroup
	var ktpURL, selfieURL string
	var ktpErr, selfieErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		ktpURL, ktpErr = cu.media.Upload(ctx, req.KTPPhoto)
	}()

	go func() {
		defer wg.Done()
		selfieURL, selfieErr = cu.media.Upload(ctx, req.SelfiePhoto)
	}()

	wg.Wait()

	if ktpErr != nil {
		return nil, errors.New("failed to upload KTP photo: " + ktpErr.Error())
	}
	if selfieErr != nil {
		return nil, errors.New("failed to upload selfie photo: " + selfieErr.Error())
	}

	// 3. Parsing tanggal lahir
	birthDate, _ := time.Parse("2006-01-02", req.BirthDate)

	// 4. Buat entitas customer baru
	newCustomer := &models.Customer{
		NIK:                req.NIK,
		FullName:           req.FullName,
		LegalName:          req.LegalName,
		BirthPlace:         req.BirthPlace,
		BirthDate:          birthDate,
		Salary:             req.Salary,
		KTPPhotoURL:        ktpURL,
		SelfiePhotoURL:     selfieURL,
		VerificationStatus: models.VerificationPending,
	}

	// 5. Simpan ke database
	if err := cu.customer.Save(ctx, newCustomer); err != nil {
		return nil, err
	}

	return newCustomer, nil
}

func NewUsecase(
	db *gorm.DB,
	cr repositories.CustomerRepository,
	lr repositories.LimitRepository,
	tr repositories.TenorRepository,
	ttr repositories.TransactionRepository,
	media Media,
) Usecases {
	return &usecase{
		db:          db,
		customer:    cr,
		limit:       lr,
		tenor:       tr,
		transaction: ttr,
		media:       media,
	}
}
