package services

import (
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// initializeTaxReportTestDatabase creates a temporary sqlite database with tax related tables
func initializeTaxReportTestDatabase(t *testing.T) (*TaxReportService, func()) {
	databasePath := filepath.Join(t.TempDir(), "tax_report_test.db")

	err := datastore.InitializeDataStore(&settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType: settings.Sqlite3DbType,
			DatabasePath: databasePath,
		},
	})
	assertNoError(t, err)

	originalContainer := *datastore.Container

	err = datastore.Container.UserDataStore.SyncStructs(
		new(models.Account),
		new(models.Transaction),
		new(models.TransactionCategory),
		new(models.TransactionPictureInfo),
		new(models.UserCustomExchangeRate),
		new(models.UserCustomDailyExchangeRate),
		new(models.TaxReportSnapshot),
	)
	assertNoError(t, err)

	service := &TaxReportService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}

	cleanup := func() {
		*datastore.Container = originalContainer
	}

	return service, cleanup
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func insertTaxReportFixtures(t *testing.T) {
	insert := func(beans ...any) {
		for _, bean := range beans {
			_, err := datastore.Container.UserDataStore.Get(0).NewSession(core.NewNullContext()).Insert(bean)
			assertNoError(t, err)
		}
	}

	accounts := []*models.Account{
		{
			AccountId: 1, Uid: 1, Deleted: false, Category: models.ACCOUNT_CATEGORY_CASH,
			Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, ParentAccountId: 0, Name: "Wallet CNY",
			DisplayOrder: 1, Icon: 1, Color: "000000", Currency: "CNY", Balance: 0, Hidden: false,
		},
		{
			AccountId: 2, Uid: 1, Deleted: false, Category: models.ACCOUNT_CATEGORY_CASH,
			Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, ParentAccountId: 0, Name: "Cash USD",
			DisplayOrder: 2, Icon: 1, Color: "000000", Currency: "USD", Balance: 0, Hidden: false,
		},
		{
			AccountId: 3, Uid: 1, Deleted: false, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD,
			Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, ParentAccountId: 0, Name: "Credit Card",
			DisplayOrder: 3, Icon: 1, Color: "000000", Currency: "CNY", Balance: 0, Hidden: false,
		},
	}

	for _, account := range accounts {
		insert(account)
	}

	categories := []*models.TransactionCategory{
		{
			CategoryId: 10, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: 0, Name: "Housing", DisplayOrder: 1, Icon: 1, Color: "000000", Hidden: false,
		},
		{
			CategoryId: 100, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: 10, Name: "Rent", DisplayOrder: 1, Icon: 1, Color: "000000",
			Hidden: false, TaxDeductible: true,
		},
		{
			CategoryId: 11, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: 0, Name: "Living", DisplayOrder: 2, Icon: 1, Color: "000000", Hidden: false,
		},
		{
			CategoryId: 101, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_EXPENSE,
			ParentCategoryId: 11, Name: "Books", DisplayOrder: 1, Icon: 1, Color: "000000",
			Hidden: false, TaxDeductible: false,
		},
		{
			CategoryId: 20, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_INCOME,
			ParentCategoryId: 0, Name: "Refund Income", DisplayOrder: 1, Icon: 1, Color: "000000", Hidden: false,
		},
		{
			CategoryId: 200, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_INCOME,
			ParentCategoryId: 20, Name: "Refund", DisplayOrder: 1, Icon: 1, Color: "000000",
			Hidden: false, TaxRefund: true,
		},
		{
			CategoryId: 30, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_TRANSFER,
			ParentCategoryId: 0, Name: "Transfer", DisplayOrder: 1, Icon: 1, Color: "000000", Hidden: false,
		},
		{
			CategoryId: 300, Uid: 1, Deleted: false, Type: models.CATEGORY_TYPE_TRANSFER,
			ParentCategoryId: 30, Name: "Account Transfer", DisplayOrder: 1, Icon: 1, Color: "000000",
			Hidden: false,
		},
	}

	for _, category := range categories {
		insert(category)
	}

	dailyRates := []*models.UserCustomDailyExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", TransactionDate: 20250201,
			Rate: 710000000,
		},
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", TransactionDate: 20250401,
			Rate: 715000000,
		},
	}

	for _, dailyRate := range dailyRates {
		insert(dailyRate)
	}

	transactions := []*models.Transaction{
		// Rent expense 100 USD with daily exact rate 7.1, no receipt
		{
			TransactionId: 901, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
			CategoryId: 100, AccountId: 2, TransactionTime: time.Date(2025, time.February, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
			TimezoneUtcOffset: 0, Amount: 10000,
		},
		// Books expense in non-deductible category
		{
			TransactionId: 902, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
			CategoryId: 101, AccountId: 1, TransactionTime: time.Date(2025, time.March, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
			TimezoneUtcOffset: 0, Amount: 5000,
		},
		// Refund income 20 USD with daily exact rate 7.15, has receipt
		{
			TransactionId: 903, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_INCOME,
			CategoryId: 200, AccountId: 2, TransactionTime: time.Date(2025, time.April, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
			TimezoneUtcOffset: 0, Amount: 2000,
		},
		// Account repayment: wallet CNY -> credit card CNY 300
		{
			TransactionId: 904, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT,
			CategoryId: 300, AccountId: 1, RelatedAccountId: 3,
			TransactionTime: time.Date(2025, time.May, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
			TimezoneUtcOffset: 0, Amount: 30000, RelatedAccountAmount: 30000,
		},
		// Normal transfer: wallet CNY -> cash USD
		{
			TransactionId: 905, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT,
			CategoryId: 300, AccountId: 1, RelatedAccountId: 2,
			TransactionTime: time.Date(2025, time.June, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
			TimezoneUtcOffset: 0, Amount: 7100, RelatedAccountAmount: 100,
		},
	}

	for _, transaction := range transactions {
		insert(transaction)
	}

	// Receipt picture attached to refund transaction
	insert(&models.TransactionPictureInfo{
		Uid: 1, Deleted: false, TransactionId: 903, PictureId: 5001,
		PictureExtension: "png",
	})
}

func TestTaxReportService_GetTaxSummary(t *testing.T) {
	service, cleanup := initializeTaxReportTestDatabase(t)
	defer cleanup()

	insertTaxReportFixtures(t)

	summary, err := service.GetTaxSummary(core.NewNullContext(), 1, 2025, core.FISCAL_YEAR_START_DEFAULT, "CNY", 0)
	assertNoError(t, err)

	if summary.TotalDeductibleAmount != "71000" {
		t.Fatalf("expected total deductible amount 71000, actual %s", summary.TotalDeductibleAmount)
	}

	if summary.TotalRefundAmount != "14300" {
		t.Fatalf("expected total refund amount 14300, actual %s", summary.TotalRefundAmount)
	}

	if summary.TotalNetDeductibleAmount != "56700" {
		t.Fatalf("expected net deductible amount 56700, actual %s", summary.TotalNetDeductibleAmount)
	}

	if len(summary.Categories) != 1 {
		t.Fatalf("expected 1 deductible category, actual %d", len(summary.Categories))
	}

	rentCategory := summary.Categories[0]

	if rentCategory.CategoryName != "Rent" || rentCategory.TransactionCount != 1 {
		t.Fatalf("unexpected rent category: %+v", rentCategory)
	}

	if rentCategory.MissingReceiptCount != 1 || rentCategory.MissingReceiptAmount != "71000" {
		t.Fatalf("expected 1 missing receipt of 71000, actual %d / %s", rentCategory.MissingReceiptCount, rentCategory.MissingReceiptAmount)
	}

	rentTransaction := rentCategory.Transactions[0]

	if rentTransaction.OriginalAmount != "10000" || rentTransaction.OriginalCurrency != "USD" {
		t.Fatalf("unexpected original amount: %s %s", rentTransaction.OriginalAmount, rentTransaction.OriginalCurrency)
	}

	if rentTransaction.ExchangeRate != "7.1" || rentTransaction.ExchangeRateDate != "2025-02-01" {
		t.Fatalf("unexpected exchange rate: %s on %s", rentTransaction.ExchangeRate, rentTransaction.ExchangeRateDate)
	}

	if rentTransaction.ConvertedAmount != "71000" || rentTransaction.HasReceipt {
		t.Fatalf("unexpected converted amount / receipt: %s / %v", rentTransaction.ConvertedAmount, rentTransaction.HasReceipt)
	}

	if len(summary.Refunds) != 1 || summary.Refunds[0].CategoryName != "Refund" {
		t.Fatalf("expected 1 refund group, actual %+v", summary.Refunds)
	}

	if !summary.Refunds[0].Transactions[0].HasReceipt {
		t.Fatal("refund transaction should have receipt")
	}

	if len(summary.ExcludedTransactions.TransferTransactions) != 1 ||
		summary.ExcludedTransactions.TotalTransferAmount != "7100" {
		t.Fatalf("unexpected transfers: %+v", summary.ExcludedTransactions.TransferTransactions)
	}

	if len(summary.ExcludedTransactions.AccountRepaymentTransactions) != 1 ||
		summary.ExcludedTransactions.TotalAccountRepaymentAmount != "30000" {
		t.Fatalf("unexpected account repayments: %+v", summary.ExcludedTransactions.AccountRepaymentTransactions)
	}

	if len(summary.RateWarnings) != 0 {
		t.Fatalf("expected no rate warnings, actual %d", len(summary.RateWarnings))
	}

	if summary.Changes != nil {
		t.Fatal("expected no changes before snapshot created")
	}
}

func TestTaxReportService_SnapshotAndChanges(t *testing.T) {
	service, cleanup := initializeTaxReportTestDatabase(t)
	defer cleanup()

	insertTaxReportFixtures(t)

	// Create snapshot of current summary
	snapshotInfo, err := service.CreateSnapshot(core.NewNullContext(), 1, "127.0.0.1", 2025, core.FISCAL_YEAR_START_DEFAULT, "CNY")
	assertNoError(t, err)

	if snapshotInfo.TotalDeductibleAmount != "71000" || snapshotInfo.TotalRefundAmount != "14300" {
		t.Fatalf("unexpected snapshot info: %+v", snapshotInfo)
	}

	allSnapshots, err := service.GetAllSnapshots(core.NewNullContext(), 1)
	assertNoError(t, err)

	if len(allSnapshots) != 1 {
		t.Fatalf("expected 1 snapshot, actual %d", len(allSnapshots))
	}

	// Load full snapshot with content
	snapshot, err := service.GetSnapshot(core.NewNullContext(), 1, snapshotInfo.SnapshotId)
	assertNoError(t, err)

	if snapshot.BaseCurrency != "CNY" || snapshot.Year != 2025 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	// Add another deductible rent expense after snapshot
	_, err = datastore.Container.UserDataStore.Get(0).NewSession(core.NewNullContext()).Insert(&models.Transaction{
		TransactionId: 906, Uid: 1, Deleted: false, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
		CategoryId: 100, AccountId: 1,
		TransactionTime: time.Date(2025, time.July, 1, 10, 0, 0, 0, time.UTC).Unix() * 1000,
		TimezoneUtcOffset: 0, Amount: 1000,
	})
	assertNoError(t, err)

	// Regenerate and compare with the latest snapshot automatically
	summary, err := service.GetTaxSummary(core.NewNullContext(), 1, 2025, core.FISCAL_YEAR_START_DEFAULT, "CNY", 0)
	assertNoError(t, err)

	if summary.TotalDeductibleAmount != "72000" {
		t.Fatalf("expected total deductible amount 72000, actual %s", summary.TotalDeductibleAmount)
	}

	if summary.Changes == nil || summary.ComparedSnapshotId != snapshotInfo.SnapshotId {
		t.Fatal("expected changes against latest snapshot")
	}

	if summary.Changes.TotalDeductibleAmountDelta != "1000" ||
		summary.Changes.TotalRefundAmountDelta != "0" ||
		summary.Changes.TotalNetDeductibleAmountDelta != "1000" {
		t.Fatalf("unexpected total deltas: %+v", summary.Changes)
	}

	// All tax related categories are compared, including unchanged refund category
	if len(summary.Changes.CategoryChanges) != 2 {
		t.Fatalf("expected 2 category changes, actual %d", len(summary.Changes.CategoryChanges))
	}

	var rentChange *models.TaxSummaryCategoryChangeResponseItem

	for i := 0; i < len(summary.Changes.CategoryChanges); i++ {
		if summary.Changes.CategoryChanges[i].CategoryName == "Rent" {
			rentChange = summary.Changes.CategoryChanges[i]
		}
	}

	if rentChange == nil {
		t.Fatal("cannot find rent category change")
	}

	if rentChange.BeforeAmount != "71000" ||
		rentChange.AfterAmount != "72000" || rentChange.Delta != "1000" {
		t.Fatalf("unexpected rent change: %+v", rentChange)
	}

	// Delete snapshot, then no comparison
	err = service.DeleteSnapshot(core.NewNullContext(), 1, snapshotInfo.SnapshotId)
	assertNoError(t, err)

	summary, err = service.GetTaxSummary(core.NewNullContext(), 1, 2025, core.FISCAL_YEAR_START_DEFAULT, "CNY", 0)
	assertNoError(t, err)

	if summary.Changes != nil {
		t.Fatal("expected no changes after snapshot deleted")
	}
}
