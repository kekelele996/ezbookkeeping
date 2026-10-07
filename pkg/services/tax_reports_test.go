package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestGetTaxYearRange_DefaultFiscalYearStart(t *testing.T) {
	startTime, endTime := getTaxYearRange(2025, core.FISCAL_YEAR_START_DEFAULT)

	expectedStart := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
	expectedEnd := time.Date(2025, time.December, 31, 23, 59, 59, 0, time.UTC).Unix()

	assert.Equal(t, expectedStart, startTime)
	assert.Equal(t, expectedEnd, endTime)
}

func TestGetTaxYearRange_CustomFiscalYearStart(t *testing.T) {
	fiscalYearStart, err := core.NewFiscalYearStart(4, 6)
	assert.Nil(t, err)

	startTime, endTime := getTaxYearRange(2025, fiscalYearStart)

	expectedStart := time.Date(2025, time.April, 6, 0, 0, 0, 0, time.UTC).Unix()
	expectedEnd := time.Date(2026, time.April, 5, 23, 59, 59, 0, time.UTC).Unix()

	assert.Equal(t, expectedStart, startTime)
	assert.Equal(t, expectedEnd, endTime)
}

func TestGetTransactionLocalDate(t *testing.T) {
	// 2025-01-01 00:00 UTC
	unixTime := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	assert.Equal(t, int32(20250101), getTransactionLocalDate(unixTime, 0))
	assert.Equal(t, int32(20250101), getTransactionLocalDate(unixTime, 480))  // UTC+8: 2025-01-01 08:00
	assert.Equal(t, int32(20241231), getTransactionLocalDate(unixTime, -480)) // UTC-8: 2024-12-31 16:00
}

func TestParseDateTextAndFormatDateText(t *testing.T) {
	date, err := parseDateText("2025-03-01")
	assert.Nil(t, err)
	assert.Equal(t, int32(20250301), date)
	assert.Equal(t, "2025-03-01", formatDateText(date))

	_, err = parseDateText("2025/03/01")
	assert.NotNil(t, err)

	_, err = parseDateText("invalid")
	assert.NotNil(t, err)
}

func TestConvertAmountWithRate(t *testing.T) {
	// rate = 7.0
	assert.Equal(t, int64(700), convertAmountWithRate(100, 700000000, 100000000))

	// rate = 1.5, half rounds away from zero
	assert.Equal(t, int64(2), convertAmountWithRate(1, 150000000, 100000000))
	assert.Equal(t, int64(-2), convertAmountWithRate(-1, 150000000, 100000000))

	// rate = 1.49999999
	assert.Equal(t, int64(1), convertAmountWithRate(1, 149999999, 100000000))

	// rate = 2/3
	assert.Equal(t, int64(67), convertAmountWithRate(100, 2, 3))

	// invalid denominator
	assert.Equal(t, int64(0), convertAmountWithRate(100, 100, 0))
}

func TestFormatExchangeRate(t *testing.T) {
	assert.Equal(t, "7", formatExchangeRate(700000000, 100000000))
	assert.Equal(t, "1.5", formatExchangeRate(150000000, 100000000))
	assert.Equal(t, "", formatExchangeRate(100, 0))
}

func TestTaxExchangeRateResolver_BaseCurrency(t *testing.T) {
	resolver := newTaxExchangeRateResolver("CNY", nil, nil)
	resolved := resolver.resolve("CNY", 20250101)

	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_NOT_USED, resolved.source)
	assert.Equal(t, int64(1), resolved.numerator)
	assert.Equal(t, int64(1), resolved.denominator)
}

func TestTaxExchangeRateResolver_DailyExact(t *testing.T) {
	dailyRates := []*models.UserCustomDailyExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", TransactionDate: 20250115,
			Rate: 700000000,
		},
	}
	resolver := newTaxExchangeRateResolver("CNY", dailyRates, nil)
	resolved := resolver.resolve("USD", 20250115)

	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_DAILY_EXACT, resolved.source)
	assert.Equal(t, int64(700000000), resolved.numerator)
	assert.Equal(t, int64(100000000), resolved.denominator)
	assert.Equal(t, int32(20250115), resolved.date)
}

func TestTaxExchangeRateResolver_DailyNearestEarlier(t *testing.T) {
	dailyRates := []*models.UserCustomDailyExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", TransactionDate: 20250115,
			Rate: 700000000,
		},
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", TransactionDate: 20250301,
			Rate: 720000000,
		},
	}
	resolver := newTaxExchangeRateResolver("CNY", dailyRates, nil)

	// The rate on 2025-02-01 should fallback to the earlier rate on 2025-01-15, not the later one
	resolved := resolver.resolve("USD", 20250201)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_DAILY_NEAREST, resolved.source)
	assert.Equal(t, int32(20250115), resolved.date)
	assert.Equal(t, int64(700000000), resolved.numerator)

	// Before any daily rate, no rate available
	resolved = resolver.resolve("USD", 20250110)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE, resolved.source)
}

func TestTaxExchangeRateResolver_LatestFallback(t *testing.T) {
	latestRates := []*models.UserCustomExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "CNY", Rate: 100000000,
		},
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", Rate: 690000000,
		},
	}
	resolver := newTaxExchangeRateResolver("CNY", nil, latestRates)

	resolved := resolver.resolve("USD", 20241201)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_LATEST_FALLBACK, resolved.source)
	// big.Rat normalizes 6.9 = 690000000/100000000 = 69/10
	assert.Equal(t, int64(69), resolved.numerator)
	assert.Equal(t, int64(10), resolved.denominator)

	// Cross base currency conversion, big.Rat normalizes 1.08 = 108/100 = 27/25
	latestRatesForEur := []*models.UserCustomExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "EUR", Rate: 100000000,
		},
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", Rate: 108000000,
		},
	}
	resolver = newTaxExchangeRateResolver("EUR", nil, latestRatesForEur)
	resolved = resolver.resolve("USD", 20241201)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_LATEST_FALLBACK, resolved.source)
	assert.Equal(t, int64(27), resolved.numerator)
	assert.Equal(t, int64(25), resolved.denominator)
}

func TestTaxExchangeRateResolver_LatestFallbackUnavailableWithoutBaseRate(t *testing.T) {
	latestRates := []*models.UserCustomExchangeRate{
		{
			Uid: 1, DeletedUnixTime: 0, Currency: "USD", Rate: 690000000,
		},
	}
	resolver := newTaxExchangeRateResolver("CNY", nil, latestRates)

	resolved := resolver.resolve("USD", 20241201)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE, resolved.source)
}

func TestAppendRateWarningIfNeeded(t *testing.T) {
	warnings := make([]*models.TaxRateWarningResponseItem, 0)
	existed := make(map[string]bool)

	resolved := &resolvedTaxExchangeRate{
		source: models.TAX_EXCHANGE_RATE_SOURCE_DAILY_NEAREST,
		date:   20250115,
	}

	warnings = appendRateWarningIfNeeded(warnings, existed, "USD", 20250201, resolved)
	warnings = appendRateWarningIfNeeded(warnings, existed, "USD", 20250201, resolved) // duplicate

	assert.Equal(t, 1, len(warnings))
	assert.Equal(t, "USD", warnings[0].Currency)
	assert.Equal(t, "2025-02-01", warnings[0].Date)
	assert.Equal(t, models.TAX_EXCHANGE_RATE_WARNING_REASON_DAILY_NOT_FOUND, warnings[0].Reason)

	// Exact rate does not produce warning
	exactResolved := &resolvedTaxExchangeRate{
		source: models.TAX_EXCHANGE_RATE_SOURCE_DAILY_EXACT,
	}
	warnings = appendRateWarningIfNeeded(warnings, existed, "EUR", 20250201, exactResolved)
	assert.Equal(t, 1, len(warnings))

	// Unavailable rate produces all-not-found warning
	unavailableResolved := &resolvedTaxExchangeRate{
		source: models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE,
	}
	warnings = appendRateWarningIfNeeded(warnings, existed, "JPY", 20250201, unavailableResolved)
	assert.Equal(t, 2, len(warnings))
	assert.Equal(t, models.TAX_EXCHANGE_RATE_WARNING_REASON_ALL_NOT_FOUND, warnings[1].Reason)
}

func TestBuildTaxSummaryChanges(t *testing.T) {
	content := &models.TaxReportSnapshotContent{
		Year:                     2025,
		TotalDeductibleAmount:    "1000",
		TotalRefundAmount:        "100",
		TotalNetDeductibleAmount: "900",
		Categories: []*models.TaxReportSnapshotCategoryItem{
			{
				CategoryId:       10,
				ParentCategoryId: 1,
				CategoryName:     "Books",
				Amount:           "1000",
				TransactionCount: 2,
			},
			{
				CategoryId:       11,
				ParentCategoryId: 1,
				CategoryName:     "Removed Category",
				Amount:           "300",
				TransactionCount: 1,
			},
		},
		Refunds: []*models.TaxReportSnapshotCategoryItem{},
	}

	contentJson, err := json.Marshal(content)
	assert.Nil(t, err)

	snapshot := &models.TaxReportSnapshot{
		SnapshotId:      5,
		Uid:             1,
		Year:            2025,
		BaseCurrency:    "CNY",
		Content:         string(contentJson),
		CreatedUnixTime: 1735689600,
	}

	current := &models.TaxSummaryResponse{
		TotalDeductibleAmount:    "1700",
		TotalRefundAmount:        "100",
		TotalNetDeductibleAmount: "1600",
		Categories: []*models.TaxCategorySummaryResponseItem{
			{
				CategoryId:   10,
				CategoryName: "Books",
				Amount:       "1200",
			},
			{
				CategoryId:   20,
				CategoryName: "New Category",
				Amount:       "500",
			},
		},
		Refunds: []*models.TaxCategorySummaryResponseItem{},
	}

	changes, err := buildTaxSummaryChanges(snapshot, current)
	assert.Nil(t, err)
	assert.Equal(t, int64(5), changes.SnapshotId)
	assert.Equal(t, int64(1735689600), changes.SnapshotCreatedUnixTime)
	assert.Equal(t, "700", changes.TotalDeductibleAmountDelta)
	assert.Equal(t, "0", changes.TotalRefundAmountDelta)
	assert.Equal(t, "700", changes.TotalNetDeductibleAmountDelta)

	// Three category change lines: changed (10), removed (11), added (20)
	assert.Equal(t, 3, len(changes.CategoryChanges))

	changeMap := make(map[int64]*models.TaxSummaryCategoryChangeResponseItem)

	for i := 0; i < len(changes.CategoryChanges); i++ {
		changeMap[changes.CategoryChanges[i].CategoryId] = changes.CategoryChanges[i]
	}

	assert.Equal(t, "1000", changeMap[10].BeforeAmount)
	assert.Equal(t, "1200", changeMap[10].AfterAmount)
	assert.Equal(t, "200", changeMap[10].Delta)
	assert.Equal(t, models.TAX_REPORT_CATEGORY_CHANGE_TYPE_DEDUCTIBLE, changeMap[10].Type)

	assert.Equal(t, "300", changeMap[11].BeforeAmount)
	assert.Equal(t, "0", changeMap[11].AfterAmount)
	assert.Equal(t, "-300", changeMap[11].Delta)

	assert.Equal(t, "0", changeMap[20].BeforeAmount)
	assert.Equal(t, "500", changeMap[20].AfterAmount)
	assert.Equal(t, "500", changeMap[20].Delta)
}
