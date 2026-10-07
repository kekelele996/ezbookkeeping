package errs

import "net/http"

// Error codes related to tax report
var (
	ErrTaxReportSnapshotNotFound              = NewNormalError(NormalSubcategoryTaxReport, 0, http.StatusBadRequest, "tax report snapshot not found")
	ErrTaxReportDailyExchangeRateNotFound     = NewNormalError(NormalSubcategoryTaxReport, 1, http.StatusBadRequest, "tax daily exchange rate data not found")
	ErrTaxReportDailyExchangeRateDateInvalid  = NewNormalError(NormalSubcategoryTaxReport, 2, http.StatusBadRequest, "tax daily exchange rate date is invalid")
	ErrTaxReportDailyExchangeRateRateInvalid  = NewNormalError(NormalSubcategoryTaxReport, 3, http.StatusBadRequest, "tax daily exchange rate is invalid")
)
