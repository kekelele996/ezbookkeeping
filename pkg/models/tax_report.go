package models

import "github.com/mayswind/ezbookkeeping/pkg/core"

// TaxExchangeRateSource represents the source of exchange rate used for tax report conversion
type TaxExchangeRateSource byte

// Tax exchange rate sources
const (
	TAX_EXCHANGE_RATE_SOURCE_NOT_USED        TaxExchangeRateSource = 0 // The transaction currency is same as base currency
	TAX_EXCHANGE_RATE_SOURCE_DAILY_EXACT     TaxExchangeRateSource = 1 // Exact user custom daily exchange rate on transaction date
	TAX_EXCHANGE_RATE_SOURCE_DAILY_NEAREST   TaxExchangeRateSource = 2 // Nearest earlier user custom daily exchange rate
	TAX_EXCHANGE_RATE_SOURCE_LATEST_FALLBACK TaxExchangeRateSource = 3 // Latest user custom exchange rate fallback
	TAX_EXCHANGE_RATE_SOURCE_UNAVAILABLE     TaxExchangeRateSource = 4 // No exchange rate available, converted amount is 0
)

// TaxExchangeRateWarningReason represents the reason why the exchange rate cannot be resolved exactly
type TaxExchangeRateWarningReason byte

// Tax exchange rate warning reasons
const (
	TAX_EXCHANGE_RATE_WARNING_REASON_DAILY_NOT_FOUND TaxExchangeRateWarningReason = 1 // No exact daily rate, fallback rate used
	TAX_EXCHANGE_RATE_WARNING_REASON_ALL_NOT_FOUND   TaxExchangeRateWarningReason = 2 // No any available exchange rate
)

// TaxSummaryRequest represents all parameters of tax summary request
type TaxSummaryRequest struct {
	Year            int32  `form:"year" binding:"required,min=1900,max=2999"`
	FiscalYearStart string `form:"fiscal_year_start"` // MM-DD, empty means default (January 1)
	BaseCurrency    string `form:"base_currency" binding:"required,len=3,validCurrency"`
	SnapshotId      int64  `form:"snapshot_id"` // Optional snapshot id to compare, 0 means compare with latest snapshot
}

// TaxSummaryResponse represents the response of annual tax summary
type TaxSummaryResponse struct {
	Year                     int32                             `json:"year"`
	StartTime                int64                             `json:"startTime"`
	EndTime                  int64                             `json:"endTime"`
	FiscalYearStart          string                            `json:"fiscalYearStart"`
	BaseCurrency             string                            `json:"baseCurrency"`
	GeneratedUnixTime        int64                             `json:"generatedUnixTime"`
	TotalDeductibleAmount    string                            `json:"totalDeductibleAmount"`
	TotalRefundAmount        string                            `json:"totalRefundAmount"`
	TotalNetDeductibleAmount string                            `json:"totalNetDeductibleAmount"`
	Categories               []*TaxCategorySummaryResponseItem `json:"categories"`
	Refunds                  []*TaxCategorySummaryResponseItem `json:"refunds"`
	ExcludedTransactions     *TaxExcludedTransactionsResponse  `json:"excludedTransactions"`
	RateWarnings             []*TaxRateWarningResponseItem     `json:"rateWarnings"`
	ComparedSnapshotId       int64                             `json:"comparedSnapshotId,string,omitempty"`
	Changes                  *TaxSummaryChangesResponse        `json:"changes,omitempty"`
}

// TaxCategorySummaryResponseItem represents summary data of one tax related category
type TaxCategorySummaryResponseItem struct {
	CategoryId           int64                         `json:"categoryId,string"`
	ParentCategoryId     int64                         `json:"parentCategoryId,string"`
	CategoryName         string                        `json:"categoryName"`
	Amount               string                        `json:"amount"`
	TransactionCount     int32                         `json:"transactionCount"`
	MissingReceiptCount  int32                         `json:"missingReceiptCount"`
	MissingReceiptAmount string                        `json:"missingReceiptAmount"`
	Transactions         []*TaxTransactionResponseItem `json:"transactions"`
}

// TaxTransactionResponseItem represents one source transaction in tax report
type TaxTransactionResponseItem struct {
	Id                 int64                 `json:"id,string"`
	Time               int64                 `json:"time"`
	UtcOffset          int16                 `json:"utcOffset"`
	AccountId          int64                 `json:"accountId,string"`
	AccountName        string                `json:"accountName"`
	CategoryId         int64                 `json:"categoryId,string"`
	Comment            string                `json:"comment"`
	OriginalAmount     string                `json:"originalAmount"`
	OriginalCurrency   string                `json:"originalCurrency"`
	ExchangeRateDate   string                `json:"exchangeRateDate"`
	ExchangeRate       string                `json:"exchangeRate"`
	ExchangeRateSource TaxExchangeRateSource `json:"exchangeRateSource"`
	ConvertedAmount    string                `json:"convertedAmount"`
	HasReceipt         bool                  `json:"hasReceipt"`
}

// TaxExcludedTransactionsResponse represents transactions which are excluded from tax deductible amount
type TaxExcludedTransactionsResponse struct {
	TransferTransactions         []*TaxExcludedTransactionResponseItem `json:"transferTransactions"`
	AccountRepaymentTransactions []*TaxExcludedTransactionResponseItem `json:"accountRepaymentTransactions"`
	TotalTransferAmount          string                                `json:"totalTransferAmount"`
	TotalAccountRepaymentAmount  string                                `json:"totalAccountRepaymentAmount"`
}

// TaxExcludedTransactionResponseItem represents one excluded transaction (transfer or account repayment)
type TaxExcludedTransactionResponseItem struct {
	Id                     int64                 `json:"id,string"`
	Time                   int64                 `json:"time"`
	UtcOffset              int16                 `json:"utcOffset"`
	AccountId              int64                 `json:"accountId,string"`
	AccountName            string                `json:"accountName"`
	DestinationAccountId   int64                 `json:"destinationAccountId,string"`
	DestinationAccountName string                `json:"destinationAccountName"`
	Comment                string                `json:"comment"`
	OriginalAmount         string                `json:"originalAmount"`
	OriginalCurrency       string                `json:"originalCurrency"`
	ExchangeRateDate       string                `json:"exchangeRateDate"`
	ExchangeRate           string                `json:"exchangeRate"`
	ExchangeRateSource     TaxExchangeRateSource `json:"exchangeRateSource"`
	ConvertedAmount        string                `json:"convertedAmount"`
	HasReceipt             bool                  `json:"hasReceipt"`
}

// TaxRateWarningResponseItem represents a warning item for exchange rate resolution
type TaxRateWarningResponseItem struct {
	Currency string                       `json:"currency"`
	Date     string                       `json:"date"`
	Reason   TaxExchangeRateWarningReason `json:"reason"`
}

// TaxSummaryChangesResponse represents differences between current summary and a snapshot
type TaxSummaryChangesResponse struct {
	SnapshotId                    int64                                   `json:"snapshotId,string"`
	SnapshotCreatedUnixTime       int64                                   `json:"snapshotCreatedUnixTime"`
	TotalDeductibleAmountDelta    string                                  `json:"totalDeductibleAmountDelta"`
	TotalRefundAmountDelta        string                                  `json:"totalRefundAmountDelta"`
	TotalNetDeductibleAmountDelta string                                  `json:"totalNetDeductibleAmountDelta"`
	CategoryChanges               []*TaxSummaryCategoryChangeResponseItem `json:"categoryChanges"`
}

// TaxReportCategoryChangeType represents the type of category in comparison
type TaxReportCategoryChangeType byte

// Tax report category change types
const (
	TAX_REPORT_CATEGORY_CHANGE_TYPE_DEDUCTIBLE TaxReportCategoryChangeType = 1 // Deductible expense category
	TAX_REPORT_CATEGORY_CHANGE_TYPE_REFUND     TaxReportCategoryChangeType = 2 // Refund income category
)

// TaxSummaryCategoryChangeResponseItem represents before/after amounts of one tax related category
type TaxSummaryCategoryChangeResponseItem struct {
	CategoryId   int64                       `json:"categoryId,string"`
	CategoryName string                      `json:"categoryName"`
	Type         TaxReportCategoryChangeType `json:"type"`
	BeforeAmount string                      `json:"beforeAmount"`
	AfterAmount  string                      `json:"afterAmount"`
	Delta        string                      `json:"delta"`
}

// UserCustomDailyExchangeRate represents user custom daily exchange rate data
type UserCustomDailyExchangeRate struct {
	Uid             int64  `xorm:"PK NOT NULL"`
	DeletedUnixTime int64  `xorm:"PK NOT NULL"`
	Currency        string `xorm:"PK VARCHAR(3) NOT NULL"`
	TransactionDate int32  `xorm:"PK NOT NULL"` // Local date (yyyymmdd) of the transaction for which this exchange rate applies
	Rate            int64  `xorm:"NOT NULL"`    // Exchange rate factor relative to base currency
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

// UserCustomDailyExchangeRateListRequest represents all parameters of user custom daily exchange rate list request
type UserCustomDailyExchangeRateListRequest struct {
	StartTime int64 `form:"start_time"`
	EndTime   int64 `form:"end_time"`
}

// UserCustomDailyExchangeRateUpdateRequest represents all parameters of user custom daily exchange rate update request
type UserCustomDailyExchangeRateUpdateRequest struct {
	Currency string `json:"currency" binding:"required,len=3,validCurrency"`
	Date     string `json:"date" binding:"required"` // yyyy-mm-dd
	Rate     string `json:"rate"`                    // Amount of base currency per 1 unit of this currency
}

// UserCustomDailyExchangeRateDeleteRequest represents all parameters of user custom daily exchange rate delete request
type UserCustomDailyExchangeRateDeleteRequest struct {
	Currency string `json:"currency" binding:"required,len=3,validCurrency"`
	Date     string `json:"date" binding:"required"` // yyyy-mm-dd
}

// UserCustomDailyExchangeRateResponse represents a view-object of user custom daily exchange rate
type UserCustomDailyExchangeRateResponse struct {
	Currency string `json:"currency"`
	Date     string `json:"date"`
	Rate     string `json:"rate"`
}

// TaxReportSnapshot represents a snapshot of tax report stored in database
type TaxReportSnapshot struct {
	SnapshotId      int64 `xorm:"PK"`
	Uid             int64 `xorm:"INDEX(IDX_tax_report_snapshot_uid_deleted_year_snapshot_id) NOT NULL"`
	Deleted         bool  `xorm:"INDEX(IDX_tax_report_snapshot_uid_deleted_year_snapshot_id) NOT NULL"`
	Year            int32 `xorm:"INDEX(IDX_tax_report_snapshot_uid_deleted_year_snapshot_id) NOT NULL"`
	FiscalYearStart core.FiscalYearStart
	BaseCurrency    string `xorm:"VARCHAR(3) NOT NULL"`
	Content         string `xorm:"LONGTEXT NOT NULL"`
	CreatedIp       string `xorm:"VARCHAR(39)"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// TaxReportSnapshotContent represents the snapshot content of tax report
type TaxReportSnapshotContent struct {
	Year                           int32                            `json:"year"`
	StartTime                      int64                            `json:"startTime"`
	EndTime                        int64                            `json:"endTime"`
	FiscalYearStart                string                           `json:"fiscalYearStart"`
	BaseCurrency                   string                           `json:"baseCurrency"`
	TotalDeductibleAmount          string                           `json:"totalDeductibleAmount"`
	TotalRefundAmount              string                           `json:"totalRefundAmount"`
	TotalNetDeductibleAmount       string                           `json:"totalNetDeductibleAmount"`
	Categories                     []*TaxReportSnapshotCategoryItem `json:"categories"`
	Refunds                        []*TaxReportSnapshotCategoryItem `json:"refunds"`
	ExcludedTransferAmount         string                           `json:"excludedTransferAmount"`
	ExcludedAccountRepaymentAmount string                           `json:"excludedAccountRepaymentAmount"`
}

// TaxReportSnapshotCategoryItem represents category amount within a tax report snapshot
type TaxReportSnapshotCategoryItem struct {
	CategoryId       int64  `json:"categoryId,string"`
	ParentCategoryId int64  `json:"parentCategoryId,string"`
	CategoryName     string `json:"categoryName"`
	Amount           string `json:"amount"`
	TransactionCount int32  `json:"transactionCount"`
}

// TaxReportSnapshotCreateRequest represents all parameters of tax report snapshot creation request
type TaxReportSnapshotCreateRequest struct {
	Year            int32  `json:"year" binding:"required,min=1900,max=2999"`
	FiscalYearStart string `json:"fiscalYearStart"` // MM-DD, empty means default (January 1)
	BaseCurrency    string `json:"baseCurrency" binding:"required,len=3,validCurrency"`
}

// TaxReportSnapshotDeleteRequest represents all parameters of tax report snapshot deletion request
type TaxReportSnapshotDeleteRequest struct {
	SnapshotId int64 `json:"snapshotId,string" binding:"required,min=1"`
}

// TaxReportSnapshotInfoResponse represents basic info of a tax report snapshot
type TaxReportSnapshotInfoResponse struct {
	SnapshotId               int64  `json:"snapshotId,string"`
	Year                     int32  `json:"year"`
	FiscalYearStart          string `json:"fiscalYearStart"`
	BaseCurrency             string `json:"baseCurrency"`
	TotalDeductibleAmount    string `json:"totalDeductibleAmount"`
	TotalRefundAmount        string `json:"totalRefundAmount"`
	TotalNetDeductibleAmount string `json:"totalNetDeductibleAmount"`
	CreatedUnixTime          int64  `json:"createdUnixTime"`
}

// TaxReportSnapshotListResponse represents the response of all tax report snapshots
type TaxReportSnapshotListResponse struct {
	Items []*TaxReportSnapshotInfoResponse `json:"items"`
}

// TaxReportSnapshotGetResponse represents the response of one tax report snapshot with content
type TaxReportSnapshotGetResponse struct {
	TaxReportSnapshotInfoResponse
	Content *TaxReportSnapshotContent `json:"content"`
}

// ToTaxReportSnapshotInfoResponse returns a view-object according to database model
func (s *TaxReportSnapshot) ToTaxReportSnapshotInfoResponse(content *TaxReportSnapshotContent) *TaxReportSnapshotInfoResponse {
	return &TaxReportSnapshotInfoResponse{
		SnapshotId:               s.SnapshotId,
		Year:                     s.Year,
		FiscalYearStart:          s.FiscalYearStart.String(),
		BaseCurrency:             s.BaseCurrency,
		TotalDeductibleAmount:    content.TotalDeductibleAmount,
		TotalRefundAmount:        content.TotalRefundAmount,
		TotalNetDeductibleAmount: content.TotalNetDeductibleAmount,
		CreatedUnixTime:          s.CreatedUnixTime,
	}
}
