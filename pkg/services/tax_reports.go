package services

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// TaxReportService represents tax report data service
type TaxReportService struct {
	ServiceUsingDB
}

// Initialize a tax report service singleton instance
var (
	TaxReports = &TaxReportService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// getTaxYearRange returns the start unix time (00:00:00 UTC) and end unix time (23:59:59 UTC) of specified tax year
func getTaxYearRange(year int32, fiscalYearStart core.FiscalYearStart) (int64, int64) {
	month, day, _ := fiscalYearStart.GetMonthDay()
	yearStart := time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC)
	yearEnd := yearStart.AddDate(1, 0, 0).Add(-1)
	return yearStart.Unix(), yearEnd.Unix()
}

// getTransactionLocalDate returns the local date (yyyymmdd) of transaction
func getTransactionLocalDate(unixTime int64, utcOffset int16) int32 {
	t := time.Unix(unixTime, 0).In(time.FixedZone("Transaction Timezone", int(utcOffset)*60))
	return int32(t.Year())*10000 + int32(t.Month())*100 + int32(t.Day())
}

// formatDateText returns date text (yyyy-mm-dd) by numeric date
func formatDateText(date int32) string {
	return fmt.Sprintf("%04d-%02d-%02d", date/10000, (date/100)%100, date%100)
}

// parseDateText returns numeric date (yyyymmdd) by date text (yyyy-mm-dd)
func parseDateText(date string) (int32, error) {
	t, err := time.ParseInLocation("2006-01-02", date, time.UTC)

	if err != nil {
		return 0, err
	}

	return int32(t.Year())*10000 + int32(t.Month())*100 + int32(t.Day()), nil
}

// convertAmountWithRate converts amount using exchange rate (numerator/denominator), rounds half away from zero
func convertAmountWithRate(amount int64, numerator int64, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}

	num := new(big.Int).Mul(big.NewInt(amount), big.NewInt(numerator))
	den := big.NewInt(denominator)

	negative := num.Sign() < 0

	if negative {
		num.Abs(num)
	}

	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(num, den, remainder)

	// Half away from zero: round up if remainder/denominator >= 1/2, i.e. 2*remainder >= denominator
	twiceRemainder := new(big.Int).Add(remainder, remainder)

	if twiceRemainder.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}

	if negative {
		quotient.Neg(quotient)
	}

	return quotient.Int64()
}

// formatExchangeRate returns textual exchange rate by numerator and denominator
func formatExchangeRate(numerator int64, denominator int64) string {
	if denominator <= 0 {
		return ""
	}

	return utils.Float64ToString(float64(numerator) / float64(denominator))
}

// resolvedTaxExchangeRate contains resolved exchange rate data
type resolvedTaxExchangeRate struct {
	numerator   int64
	denominator int64
	date        int32
	source      models.TaxExchangeRateSource
}

// taxExchangeRateResolver resolves exchange rate for tax report
type taxExchangeRateResolver struct {
	baseCurrency   string
	dailyRates     map[string]map[int32]int64 // currency -> transaction date -> rate numerator (denominator is rate factor)
	latestRateRats map[string]*big.Rat        // currency -> exchange rate relative to base currency
}

// newTaxExchangeRateRangeResolver returns a new exchange rate resolver according to loaded data
func newTaxExchangeRateResolver(baseCurrency string, dailyRates []*models.UserCustomDailyExchangeRate, latestRates []*models.UserCustomExchangeRate) *taxExchangeRateResolver {
	dailyRateMap := make(map[string]map[int32]int64)

	for i := 0; i < len(dailyRates); i++ {
		dailyRate := dailyRates[i]

		if _, exists := dailyRateMap[dailyRate.Currency]; !exists {
			dailyRateMap[dailyRate.Currency] = make(map[int32]int64)
		}

		dailyRateMap[dailyRate.Currency][dailyRate.TransactionDate] = dailyRate.Rate
	}

	latestRateRats := make(map[string]*big.Rat)
	var baseCurrencyRate int64
	hasBaseCurrencyRate := false

	for i := 0; i < len(latestRates); i++ {
		if latestRates[i].Currency == baseCurrency {
			baseCurrencyRate = latestRates[i].Rate
			hasBaseCurrencyRate = true
			break
		}
	}

	if hasBaseCurrencyRate && baseCurrencyRate > 0 {
		for i := 0; i < len(latestRates); i++ {
			latestRate := latestRates[i]
			latestRateRats[latestRate.Currency] = new(big.Rat).SetFrac(big.NewInt(latestRate.Rate), big.NewInt(baseCurrencyRate))
		}
	}

	return &taxExchangeRateResolver{
		baseCurrency:   baseCurrency,
		dailyRates:     dailyRateMap,
		latestRateRats: latestRateRats,
	}
}

// resolve returns exchange rate for specified currency and transaction date
func (r *taxExchangeRateResolver) resolve(currency string, date int32) *resolvedTaxExchangeRate {
	if currency == r.baseCurrency {
		return &resolvedTaxExchangeRate{
			numerator:   1,
			denominator: 1,
			date:        date,
			source:      models.TAX_EXCHANGE_RATE_SOURCE_NOT_USED,
		}
	}

	if dateRates, exists := r.dailyRates[currency]; exists {
		if rate, exists := dateRates[date]; exists {
			return &resolvedTaxExchangeRate{
				numerator:   rate,
				denominator: models.UserCustomExchangeRateFactorInDatabase,
				date:        date,
				source:      models.TAX_EXCHANGE_RATE_SOURCE_DAILY_EXACT,
			}
		}

		fallbackDate := int32(0)
		fallbackRate := int64(0)

		for dailyDate, dailyRate := range dateRates {
			if dailyDate <= date && dailyDate > fallbackDate {
				fallbackDate = dailyDate
				fallbackRate = dailyRate
			}
		}

		if fallbackDate > 0 {
			return &resolvedTaxExchangeRate{
				numerator:   fallbackRate,
				denominator: models.UserCustomExchangeRateFactorInDatabase,
				date:        fallbackDate,
				source:      models.TAX_EXCHANGE_RATE_SOURCE_DAILY_NEAREST,
			}
		}
	}

	if rate, exists := r.latestRateRats[currency]; exists {
		return &resolvedTaxExchangeRate{
			numerator:   rate.Num().Int64(),
			denominator: rate.Denom().Int64(),
			date:        0,
			source:      models.TAX_EXCHANGE_RATE_SOURCE_LATEST_FALLBACK,
		}
	}

	return &resolvedTaxExchangeRate{
		source: models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE,
	}
}

// GetTaxSummary returns annual tax summary of current user
func (s *TaxReportService) GetTaxSummary(c core.Context, uid int64, year int32, fiscalYearStart core.FiscalYearStart, baseCurrency string, compareSnapshotId int64) (*models.TaxSummaryResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	startTime, endTime := getTaxYearRange(year, fiscalYearStart)

	var transactions []*models.Transaction
	err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND transaction_time>=? AND transaction_time<=?", uid, false, utils.GetMinTransactionTimeFromUnixTime(startTime), utils.GetMaxTransactionTimeFromUnixTime(endTime)).
		OrderBy("transaction_time desc").
		Find(&transactions)

	if err != nil {
		return nil, err
	}

	categories, err := TransactionCategories.GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		return nil, err
	}

	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	transactionIds := make([]int64, 0, len(transactions))

	for i := 0; i < len(transactions); i++ {
		transactionIds = append(transactionIds, transactions[i].TransactionId)
	}

	pictureMap, err := TransactionPictures.GetPictureInfosByTransactionIds(c, uid, transactionIds)

	if err != nil {
		return nil, err
	}

	dailyRates, err := s.GetAllDailyExchangeRates(c, uid)

	if err != nil {
		return nil, err
	}

	latestRates, err := UserCustomExchangeRates.GetAllCustomExchangeRatesByUid(c, uid)

	if err != nil {
		return nil, err
	}

	categoryMap := make(map[int64]*models.TransactionCategory)

	for i := 0; i < len(categories); i++ {
		categoryMap[categories[i].CategoryId] = categories[i]
	}

	accountMap := make(map[int64]*models.Account)

	for i := 0; i < len(accounts); i++ {
		accountMap[accounts[i].AccountId] = accounts[i]
	}

	resolver := newTaxExchangeRateResolver(baseCurrency, dailyRates, latestRates)

	deductibleGroups := make(map[int64]*models.TaxCategorySummaryResponseItem)
	refundGroups := make(map[int64]*models.TaxCategorySummaryResponseItem)
	transferTransactions := make([]*models.TaxExcludedTransactionResponseItem, 0)
	accountRepaymentTransactions := make([]*models.TaxExcludedTransactionResponseItem, 0)
	rateWarnings := make([]*models.TaxRateWarningResponseItem, 0)
	existedRateWarnings := make(map[string]bool)
	totalDeductibleAmount := int64(0)
	totalRefundAmount := int64(0)
	totalTransferAmount := int64(0)
	totalAccountRepaymentAmount := int64(0)

	for i := 0; i < len(transactions); i++ {
		transaction := transactions[i]
		unixTime := utils.GetUnixTimeFromTransactionTime(transaction.TransactionTime)

		if unixTime < startTime || unixTime > endTime {
			continue
		}

		account := accountMap[transaction.AccountId]
		category := categoryMap[transaction.CategoryId]

		if transaction.Type == models.TRANSACTION_DB_TYPE_EXPENSE {
			if category == nil || !category.TaxDeductible {
				continue
			}

			transactionDate := getTransactionLocalDate(unixTime, transaction.TimezoneUtcOffset)
			transactionCurrency := ""

			if account != nil {
				transactionCurrency = account.Currency
			}

			resolved := resolver.resolve(transactionCurrency, transactionDate)
			convertedAmount := int64(0)

			if resolved.source != models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE {
				convertedAmount = convertAmountWithRate(transaction.Amount, resolved.numerator, resolved.denominator)
			}

			rateWarnings = appendRateWarningIfNeeded(rateWarnings, existedRateWarnings, transactionCurrency, transactionDate, resolved)

			hasReceipt := len(pictureMap[transaction.TransactionId]) > 0
			item := newTaxTransactionResponseItem(transaction, account, category, transactionDate, resolved, convertedAmount, hasReceipt)

			group, exists := deductibleGroups[transaction.CategoryId]

			if !exists {
				group = newTaxCategorySummaryResponseItem(category)
				deductibleGroups[transaction.CategoryId] = group
			}

			group.Transactions = append(group.Transactions, item)
			group.TransactionCount++
			group.Amount = addAmountText(group.Amount, convertedAmount)

			if !hasReceipt {
				group.MissingReceiptCount++
				group.MissingReceiptAmount = addAmountText(group.MissingReceiptAmount, convertedAmount)
			}

			totalDeductibleAmount += convertedAmount
		} else if transaction.Type == models.TRANSACTION_DB_TYPE_INCOME {
			if category == nil || !category.TaxRefund {
				continue
			}

			transactionDate := getTransactionLocalDate(unixTime, transaction.TimezoneUtcOffset)
			transactionCurrency := ""

			if account != nil {
				transactionCurrency = account.Currency
			}

			resolved := resolver.resolve(transactionCurrency, transactionDate)
			convertedAmount := int64(0)

			if resolved.source != models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE {
				convertedAmount = convertAmountWithRate(transaction.Amount, resolved.numerator, resolved.denominator)
			}

			rateWarnings = appendRateWarningIfNeeded(rateWarnings, existedRateWarnings, transactionCurrency, transactionDate, resolved)

			hasReceipt := len(pictureMap[transaction.TransactionId]) > 0
			item := newTaxTransactionResponseItem(transaction, account, category, transactionDate, resolved, convertedAmount, hasReceipt)

			group, exists := refundGroups[transaction.CategoryId]

			if !exists {
				group = newTaxCategorySummaryResponseItem(category)
				refundGroups[transaction.CategoryId] = group
			}

			group.Transactions = append(group.Transactions, item)
			group.TransactionCount++
			group.Amount = addAmountText(group.Amount, convertedAmount)

			if !hasReceipt {
				group.MissingReceiptCount++
				group.MissingReceiptAmount = addAmountText(group.MissingReceiptAmount, convertedAmount)
			}

			totalRefundAmount += convertedAmount
		} else if transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT {
			destinationAccount := accountMap[transaction.RelatedAccountId]
			transactionDate := getTransactionLocalDate(unixTime, transaction.TimezoneUtcOffset)
			transactionCurrency := ""

			if account != nil {
				transactionCurrency = account.Currency
			}

			resolved := resolver.resolve(transactionCurrency, transactionDate)
			convertedAmount := int64(0)

			if resolved.source != models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE {
				convertedAmount = convertAmountWithRate(transaction.Amount, resolved.numerator, resolved.denominator)
			}

			rateWarnings = appendRateWarningIfNeeded(rateWarnings, existedRateWarnings, transactionCurrency, transactionDate, resolved)

			hasReceipt := len(pictureMap[transaction.TransactionId]) > 0
			item := &models.TaxExcludedTransactionResponseItem{
				Id:                     transaction.TransactionId,
				Time:                   unixTime,
				UtcOffset:              transaction.TimezoneUtcOffset,
				AccountId:              transaction.AccountId,
				AccountName:            getAccountName(account),
				DestinationAccountId:   transaction.RelatedAccountId,
				DestinationAccountName: getAccountName(destinationAccount),
				Comment:                transaction.Comment,
				OriginalAmount:         utils.Int64ToString(transaction.Amount),
				OriginalCurrency:       transactionCurrency,
				ExchangeRateDate:       formatDateText(resolved.date),
				ExchangeRate:           formatExchangeRate(resolved.numerator, resolved.denominator),
				ExchangeRateSource:     resolved.source,
				ConvertedAmount:        utils.Int64ToString(convertedAmount),
				HasReceipt:             hasReceipt,
			}

			isAccountRepayment := (account != nil && account.Category.IsLiability()) || (destinationAccount != nil && destinationAccount.Category.IsLiability())

			if isAccountRepayment {
				accountRepaymentTransactions = append(accountRepaymentTransactions, item)
				totalAccountRepaymentAmount += convertedAmount
			} else {
				transferTransactions = append(transferTransactions, item)
				totalTransferAmount += convertedAmount
			}
		}
	}

	deductibleCategoryItems := buildSortedTaxCategoryItems(deductibleGroups, categoryMap)
	refundCategoryItems := buildSortedTaxCategoryItems(refundGroups, categoryMap)

	summaryResp := &models.TaxSummaryResponse{
		Year:                     year,
		StartTime:                startTime,
		EndTime:                  endTime,
		FiscalYearStart:          fiscalYearStart.String(),
		BaseCurrency:             baseCurrency,
		GeneratedUnixTime:        time.Now().Unix(),
		TotalDeductibleAmount:    utils.Int64ToString(totalDeductibleAmount),
		TotalRefundAmount:        utils.Int64ToString(totalRefundAmount),
		TotalNetDeductibleAmount: utils.Int64ToString(totalDeductibleAmount - totalRefundAmount),
		Categories:               deductibleCategoryItems,
		Refunds:                  refundCategoryItems,
		ExcludedTransactions: &models.TaxExcludedTransactionsResponse{
			TransferTransactions:         transferTransactions,
			AccountRepaymentTransactions: accountRepaymentTransactions,
			TotalTransferAmount:          utils.Int64ToString(totalTransferAmount),
			TotalAccountRepaymentAmount:  utils.Int64ToString(totalAccountRepaymentAmount),
		},
		RateWarnings: rateWarnings,
	}

	comparedSnapshot, err := s.getSnapshotForComparison(c, uid, year, compareSnapshotId)

	if err != nil {
		return nil, err
	}

	if comparedSnapshot != nil {
		changes, err := buildTaxSummaryChanges(comparedSnapshot, summaryResp)

		if err != nil {
			return nil, err
		}

		summaryResp.ComparedSnapshotId = comparedSnapshot.SnapshotId
		summaryResp.Changes = changes
	}

	return summaryResp, nil
}

// newTaxTransactionResponseItem returns a source transaction view-object
func newTaxTransactionResponseItem(transaction *models.Transaction, account *models.Account, category *models.TransactionCategory, transactionDate int32, resolved *resolvedTaxExchangeRate, convertedAmount int64, hasReceipt bool) *models.TaxTransactionResponseItem {
	return &models.TaxTransactionResponseItem{
		Id:                 transaction.TransactionId,
		Time:               utils.GetUnixTimeFromTransactionTime(transaction.TransactionTime),
		UtcOffset:          transaction.TimezoneUtcOffset,
		AccountId:          transaction.AccountId,
		AccountName:        getAccountName(account),
		CategoryId:         transaction.CategoryId,
		Comment:            transaction.Comment,
		OriginalAmount:     utils.Int64ToString(transaction.Amount),
		OriginalCurrency:   getAccountCurrency(account),
		ExchangeRateDate:   formatDateText(resolved.date),
		ExchangeRate:       formatExchangeRate(resolved.numerator, resolved.denominator),
		ExchangeRateSource: resolved.source,
		ConvertedAmount:    utils.Int64ToString(convertedAmount),
		HasReceipt:         hasReceipt,
	}
}

// newTaxCategorySummaryResponseItem returns an empty category group
func newTaxCategorySummaryResponseItem(category *models.TransactionCategory) *models.TaxCategorySummaryResponseItem {
	parentCategoryId := int64(0)

	if category != nil {
		parentCategoryId = category.ParentCategoryId
	}

	return &models.TaxCategorySummaryResponseItem{
		CategoryId:           category.CategoryId,
		ParentCategoryId:     parentCategoryId,
		CategoryName:         category.Name,
		Amount:               "0",
		MissingReceiptAmount: "0",
		Transactions:         make([]*models.TaxTransactionResponseItem, 0),
	}
}

// buildSortedTaxCategoryItems returns category groups sorted by parent category and category display order
func buildSortedTaxCategoryItems(groups map[int64]*models.TaxCategorySummaryResponseItem, categoryMap map[int64]*models.TransactionCategory) []*models.TaxCategorySummaryResponseItem {
	items := make([]*models.TaxCategorySummaryResponseItem, 0, len(groups))

	for _, group := range groups {
		items = append(items, group)
	}

	sort.Slice(items, func(i, j int) bool {
		left := categoryMap[items[i].CategoryId]
		right := categoryMap[items[j].CategoryId]

		leftParentDisplayOrder := int32(0)
		rightParentDisplayOrder := int32(0)
		leftDisplayOrder := int32(0)
		rightDisplayOrder := int32(0)

		if left != nil {
			leftDisplayOrder = left.DisplayOrder

			if parent, exists := categoryMap[left.ParentCategoryId]; exists {
				leftParentDisplayOrder = parent.DisplayOrder
			}
		}

		if right != nil {
			rightDisplayOrder = right.DisplayOrder

			if parent, exists := categoryMap[right.ParentCategoryId]; exists {
				rightParentDisplayOrder = parent.DisplayOrder
			}
		}

		if leftParentDisplayOrder != rightParentDisplayOrder {
			return leftParentDisplayOrder < rightParentDisplayOrder
		}

		if leftDisplayOrder != rightDisplayOrder {
			return leftDisplayOrder < rightDisplayOrder
		}

		return items[i].CategoryId < items[j].CategoryId
	})

	return items
}

// appendRateWarningIfNeeded appends a rate warning if exchange rate cannot be resolved exactly
func appendRateWarningIfNeeded(warnings []*models.TaxRateWarningResponseItem, existed map[string]bool, currency string, date int32, resolved *resolvedTaxExchangeRate) []*models.TaxRateWarningResponseItem {
	var reason models.TaxExchangeRateWarningReason

	if resolved.source == models.TAX_EXCHANGE_RATE_SOURCE_DAILY_NEAREST || resolved.source == models.TAX_EXCHANGE_RATE_SOURCE_LATEST_FALLBACK {
		reason = models.TAX_EXCHANGE_RATE_WARNING_REASON_DAILY_NOT_FOUND
	} else if resolved.source == models.TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE {
		reason = models.TAX_EXCHANGE_RATE_WARNING_REASON_ALL_NOT_FOUND
	} else {
		return warnings
	}

	warningKey := fmt.Sprintf("%s_%d_%d", currency, date, reason)

	if existed[warningKey] {
		return warnings
	}

	existed[warningKey] = true

	return append(warnings, &models.TaxRateWarningResponseItem{
		Currency: currency,
		Date:     formatDateText(date),
		Reason:   reason,
	})
}

// addAmountText returns the sum of textual amount and numeric amount
func addAmountText(amountText string, amount int64) string {
	existing, err := utils.StringToInt64(amountText)

	if err != nil {
		existing = 0
	}

	return utils.Int64ToString(existing + amount)
}

// getAccountName returns the account name, returns empty string if account is nil
func getAccountName(account *models.Account) string {
	if account == nil {
		return ""
	}

	return account.Name
}

// getAccountCurrency returns the account currency, returns empty string if account is nil
func getAccountCurrency(account *models.Account) string {
	if account == nil {
		return ""
	}

	return account.Currency
}

// getSnapshotForComparison returns the snapshot which will be compared with current summary
func (s *TaxReportService) getSnapshotForComparison(c core.Context, uid int64, year int32, snapshotId int64) (*models.TaxReportSnapshot, error) {
	if snapshotId > 0 {
		return s.GetSnapshot(c, uid, snapshotId)
	}

	return s.getLatestSnapshotByYear(c, uid, year)
}

// getLatestSnapshotByYear returns the latest snapshot of specified tax year, returns nil if no snapshot exists
func (s *TaxReportService) getLatestSnapshotByYear(c core.Context, uid int64, year int32) (*models.TaxReportSnapshot, error) {
	snapshot := &models.TaxReportSnapshot{}
	has, err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND year=?", uid, false, year).
		OrderBy("created_unix_time desc").
		Limit(1, 0).
		Get(snapshot)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}

	return snapshot, nil
}

// buildTaxSummaryChanges builds differences between snapshot and current summary
func buildTaxSummaryChanges(snapshot *models.TaxReportSnapshot, current *models.TaxSummaryResponse) (*models.TaxSummaryChangesResponse, error) {
	content := &models.TaxReportSnapshotContent{}
	err := json.Unmarshal([]byte(snapshot.Content), content)

	if err != nil {
		return nil, err
	}

	categoryChanges := make([]*models.TaxSummaryCategoryChangeResponseItem, 0)
	existedCategories := make(map[int64]bool)

	appendChanges := func(beforeItems []*models.TaxReportSnapshotCategoryItem, afterItems []*models.TaxCategorySummaryResponseItem, changeType models.TaxReportCategoryChangeType) {
		afterMap := make(map[int64]*models.TaxCategorySummaryResponseItem)

		for i := 0; i < len(afterItems); i++ {
			afterMap[afterItems[i].CategoryId] = afterItems[i]
		}

		for i := 0; i < len(beforeItems); i++ {
			beforeItem := beforeItems[i]
			afterAmount := int64(0)
			categoryName := beforeItem.CategoryName

			if afterItem, exists := afterMap[beforeItem.CategoryId]; exists {
				afterAmount, _ = utils.StringToInt64(afterItem.Amount)
				categoryName = afterItem.CategoryName
			}

			beforeAmount, _ := utils.StringToInt64(beforeItem.Amount)

			categoryChanges = append(categoryChanges, &models.TaxSummaryCategoryChangeResponseItem{
				CategoryId:   beforeItem.CategoryId,
				CategoryName: categoryName,
				Type:         changeType,
				BeforeAmount: utils.Int64ToString(beforeAmount),
				AfterAmount:  utils.Int64ToString(afterAmount),
				Delta:        utils.Int64ToString(afterAmount - beforeAmount),
			})

			existedCategories[beforeItem.CategoryId] = true
		}

		for i := 0; i < len(afterItems); i++ {
			afterItem := afterItems[i]

			if existedCategories[afterItem.CategoryId] {
				continue
			}

			afterAmount, _ := utils.StringToInt64(afterItem.Amount)

			categoryChanges = append(categoryChanges, &models.TaxSummaryCategoryChangeResponseItem{
				CategoryId:   afterItem.CategoryId,
				CategoryName: afterItem.CategoryName,
				Type:         changeType,
				BeforeAmount: "0",
				AfterAmount:  utils.Int64ToString(afterAmount),
				Delta:        utils.Int64ToString(afterAmount),
			})

			existedCategories[afterItem.CategoryId] = true
		}
	}

	appendChanges(content.Categories, current.Categories, models.TAX_REPORT_CATEGORY_CHANGE_TYPE_DEDUCTIBLE)
	appendChanges(content.Refunds, current.Refunds, models.TAX_REPORT_CATEGORY_CHANGE_TYPE_REFUND)

	beforeDeductible, _ := utils.StringToInt64(content.TotalDeductibleAmount)
	beforeRefund, _ := utils.StringToInt64(content.TotalRefundAmount)
	beforeNet, _ := utils.StringToInt64(content.TotalNetDeductibleAmount)
	afterDeductible, _ := utils.StringToInt64(current.TotalDeductibleAmount)
	afterRefund, _ := utils.StringToInt64(current.TotalRefundAmount)
	afterNet, _ := utils.StringToInt64(current.TotalNetDeductibleAmount)

	return &models.TaxSummaryChangesResponse{
		SnapshotId:                    snapshot.SnapshotId,
		SnapshotCreatedUnixTime:       snapshot.CreatedUnixTime,
		TotalDeductibleAmountDelta:    utils.Int64ToString(afterDeductible - beforeDeductible),
		TotalRefundAmountDelta:        utils.Int64ToString(afterRefund - beforeRefund),
		TotalNetDeductibleAmountDelta: utils.Int64ToString(afterNet - beforeNet),
		CategoryChanges:               categoryChanges,
	}, nil
}

// CreateSnapshot creates a snapshot for current tax summary
func (s *TaxReportService) CreateSnapshot(c core.Context, uid int64, ip string, year int32, fiscalYearStart core.FiscalYearStart, baseCurrency string) (*models.TaxReportSnapshotInfoResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	summary, err := s.GetTaxSummary(c, uid, year, fiscalYearStart, baseCurrency, 0)

	if err != nil {
		return nil, err
	}

	content := &models.TaxReportSnapshotContent{
		Year:                           summary.Year,
		StartTime:                      summary.StartTime,
		EndTime:                        summary.EndTime,
		FiscalYearStart:                summary.FiscalYearStart,
		BaseCurrency:                   summary.BaseCurrency,
		TotalDeductibleAmount:          summary.TotalDeductibleAmount,
		TotalRefundAmount:              summary.TotalRefundAmount,
		TotalNetDeductibleAmount:       summary.TotalNetDeductibleAmount,
		Categories:                     buildSnapshotCategoryItems(summary.Categories),
		Refunds:                        buildSnapshotCategoryItems(summary.Refunds),
		ExcludedTransferAmount:         summary.ExcludedTransactions.TotalTransferAmount,
		ExcludedAccountRepaymentAmount: summary.ExcludedTransactions.TotalAccountRepaymentAmount,
	}

	contentJson, err := json.Marshal(content)

	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	snapshot := &models.TaxReportSnapshot{
		Uid:             uid,
		Deleted:         false,
		Year:            year,
		FiscalYearStart: fiscalYearStart,
		BaseCurrency:    baseCurrency,
		Content:         string(contentJson),
		CreatedIp:       ip,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}

	_, err = s.UserDataDB(uid).NewSession(c).Insert(snapshot)

	if err != nil {
		return nil, err
	}

	return snapshot.ToTaxReportSnapshotInfoResponse(content), nil
}

// buildSnapshotCategoryItems returns snapshot category items by category summary items
func buildSnapshotCategoryItems(items []*models.TaxCategorySummaryResponseItem) []*models.TaxReportSnapshotCategoryItem {
	snapshotItems := make([]*models.TaxReportSnapshotCategoryItem, 0, len(items))

	for i := 0; i < len(items); i++ {
		item := items[i]
		snapshotItems = append(snapshotItems, &models.TaxReportSnapshotCategoryItem{
			CategoryId:       item.CategoryId,
			ParentCategoryId: item.ParentCategoryId,
			CategoryName:     item.CategoryName,
			Amount:           item.Amount,
			TransactionCount: item.TransactionCount,
		})
	}

	return snapshotItems
}

// GetAllSnapshots returns all tax report snapshots of current user
func (s *TaxReportService) GetAllSnapshots(c core.Context, uid int64) ([]*models.TaxReportSnapshot, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var snapshots []*models.TaxReportSnapshot
	err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=?", uid, false).
		OrderBy("year desc, created_unix_time desc").
		Find(&snapshots)

	return snapshots, err
}

// GetSnapshot returns a tax report snapshot by snapshot id
func (s *TaxReportService) GetSnapshot(c core.Context, uid int64, snapshotId int64) (*models.TaxReportSnapshot, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	snapshot := &models.TaxReportSnapshot{}
	has, err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND snapshot_id=?", uid, false, snapshotId).
		Get(snapshot)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrTaxReportSnapshotNotFound
	}

	return snapshot, nil
}

// DeleteSnapshot deletes a tax report snapshot
func (s *TaxReportService) DeleteSnapshot(c core.Context, uid int64, snapshotId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.Cols("deleted", "deleted_unix_time", "updated_unix_time").
			Where("uid=? AND deleted=? AND snapshot_id=?", uid, false, snapshotId).
			Update(&models.TaxReportSnapshot{
				Deleted:         true,
				DeletedUnixTime: now,
				UpdatedUnixTime: now,
			})

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrTaxReportSnapshotNotFound
		}

		return nil
	})
}

// GetAllDailyExchangeRates returns all effective user custom daily exchange rates
func (s *TaxReportService) GetAllDailyExchangeRates(c core.Context, uid int64) ([]*models.UserCustomDailyExchangeRate, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var dailyRates []*models.UserCustomDailyExchangeRate
	err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted_unix_time=?", uid, 0).
		OrderBy("transaction_date asc").
		Find(&dailyRates)

	return dailyRates, err
}

// UpdateDailyExchangeRate creates or updates user custom daily exchange rate
func (s *TaxReportService) UpdateDailyExchangeRate(c core.Context, uid int64, currency string, date string, rate string) (*models.UserCustomDailyExchangeRateResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	transactionDate, err := parseDateText(date)

	if err != nil {
		return nil, errs.ErrTaxReportDailyExchangeRateDateInvalid
	}

	rateValue, err := utils.StringToFloat64(rate)

	if err != nil || rateValue <= 0 {
		return nil, errs.ErrTaxReportDailyExchangeRateRateInvalid
	}

	rateFactor := int64(rateValue * float64(models.UserCustomExchangeRateFactorInDatabase))
	now := time.Now().Unix()
	newDailyRate := &models.UserCustomDailyExchangeRate{}

	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		oldDailyRate := &models.UserCustomDailyExchangeRate{}
		has, err := sess.Where("uid=? AND deleted_unix_time=? AND currency=? AND transaction_date=?", uid, 0, currency, transactionDate).Get(oldDailyRate)

		if err != nil {
			return err
		}

		if has {
			_, err = sess.Cols("deleted_unix_time").
				Where("uid=? AND deleted_unix_time=? AND currency=? AND transaction_date=?", uid, 0, currency, transactionDate).
				Update(&models.UserCustomDailyExchangeRate{
					DeletedUnixTime: now,
				})

			if err != nil {
				return err
			}
		}

		newDailyRate = &models.UserCustomDailyExchangeRate{
			Uid:             uid,
			DeletedUnixTime: 0,
			Currency:        currency,
			TransactionDate: transactionDate,
			Rate:            rateFactor,
			CreatedUnixTime: now,
			UpdatedUnixTime: now,
		}

		_, err = sess.Insert(newDailyRate)

		return err
	})

	if err != nil {
		return nil, err
	}

	return &models.UserCustomDailyExchangeRateResponse{
		Currency: currency,
		Date:     formatDateText(transactionDate),
		Rate:     utils.Float64ToString(rateValue),
	}, nil
}

// DeleteDailyExchangeRate deletes user custom daily exchange rate
func (s *TaxReportService) DeleteDailyExchangeRate(c core.Context, uid int64, currency string, date string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	transactionDate, err := parseDateText(date)

	if err != nil {
		return errs.ErrTaxReportDailyExchangeRateDateInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.Cols("deleted_unix_time").
			Where("uid=? AND deleted_unix_time=? AND currency=? AND transaction_date=?", uid, 0, currency, transactionDate).
			Update(&models.UserCustomDailyExchangeRate{
				DeletedUnixTime: now,
			})

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrTaxReportDailyExchangeRateNotFound
		}

		return nil
	})
}
