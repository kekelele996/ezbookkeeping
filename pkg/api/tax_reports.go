package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// TaxReportsApi represents tax report api
type TaxReportsApi struct {
	ApiUsingConfig
	taxReports *services.TaxReportService
}

// Initialize a tax report api singleton instance
var (
	TaxReports = &TaxReportsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		taxReports: services.TaxReports,
	}
)

// parseFiscalYearStartText parses fiscal year start text (MM-DD), empty means default January 1
func parseFiscalYearStartText(text string) (core.FiscalYearStart, *errs.Error) {
	if text == "" {
		return core.FISCAL_YEAR_START_DEFAULT, nil
	}

	parsed, err := time.Parse("01-02", text)

	if err != nil {
		return 0, errs.ErrFormatInvalid
	}

	fiscalYearStart, err := core.NewFiscalYearStart(uint8(parsed.Month()), uint8(parsed.Day()))

	if err != nil {
		return 0, errs.ErrFormatInvalid
	}

	return fiscalYearStart, nil
}

// TaxSummaryHandler returns annual tax summary of current user
func (a *TaxReportsApi) TaxSummaryHandler(c *core.WebContext) (any, *errs.Error) {
	var summaryReq models.TaxSummaryRequest
	err := c.ShouldBindQuery(&summaryReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxSummaryHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	fiscalYearStart, parseErr := parseFiscalYearStartText(summaryReq.FiscalYearStart)

	if parseErr != nil {
		log.Warnf(c, "[tax_reports.TaxSummaryHandler] parse fiscal year start failed, because %s", summaryReq.FiscalYearStart)
		return nil, parseErr
	}

	uid := c.GetCurrentUid()
	summary, err := a.taxReports.GetTaxSummary(c, uid, summaryReq.Year, fiscalYearStart, summaryReq.BaseCurrency, summaryReq.SnapshotId)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxSummaryHandler] failed to get tax summary for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return summary, nil
}

// TaxReportSnapshotListHandler returns all tax report snapshots of current user
func (a *TaxReportsApi) TaxReportSnapshotListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	snapshots, err := a.taxReports.GetAllSnapshots(c, uid)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxReportSnapshotListHandler] failed to get tax report snapshots for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	items := make([]*models.TaxReportSnapshotInfoResponse, 0, len(snapshots))

	for i := 0; i < len(snapshots); i++ {
		content := &models.TaxReportSnapshotContent{}
		err := json.Unmarshal([]byte(snapshots[i].Content), content)

		if err != nil {
			log.Errorf(c, "[tax_reports.TaxReportSnapshotListHandler] failed to parse snapshot \"snapshot_id:%d\" content, because %s", snapshots[i].SnapshotId, err.Error())
			return nil, errs.ErrOperationFailed
		}

		items = append(items, snapshots[i].ToTaxReportSnapshotInfoResponse(content))
	}

	return &models.TaxReportSnapshotListResponse{
		Items: items,
	}, nil
}

// TaxReportSnapshotGetHandler returns one tax report snapshot with full content
func (a *TaxReportsApi) TaxReportSnapshotGetHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotGetReq models.TransactionGetRequest
	err := c.ShouldBindQuery(&snapshotGetReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxReportSnapshotGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	snapshot, err := a.taxReports.GetSnapshot(c, uid, snapshotGetReq.Id)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxReportSnapshotGetHandler] failed to get tax report snapshot \"snapshot_id:%d\" for user \"uid:%d\", because %s", snapshotGetReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	content := &models.TaxReportSnapshotContent{}
	err = json.Unmarshal([]byte(snapshot.Content), content)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxReportSnapshotGetHandler] failed to parse snapshot content, because %s", err.Error())
		return nil, errs.ErrOperationFailed
	}

	return &models.TaxReportSnapshotGetResponse{
		TaxReportSnapshotInfoResponse: *snapshot.ToTaxReportSnapshotInfoResponse(content),
		Content:                       content,
	}, nil
}

// TaxReportSnapshotCreateHandler creates a snapshot for current tax summary
func (a *TaxReportsApi) TaxReportSnapshotCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotCreateReq models.TaxReportSnapshotCreateRequest
	err := c.ShouldBindJSON(&snapshotCreateReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxReportSnapshotCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	fiscalYearStart, parseErr := parseFiscalYearStartText(snapshotCreateReq.FiscalYearStart)

	if parseErr != nil {
		log.Warnf(c, "[tax_reports.TaxReportSnapshotCreateHandler] parse fiscal year start failed, because %s", snapshotCreateReq.FiscalYearStart)
		return nil, parseErr
	}

	uid := c.GetCurrentUid()
	snapshot, err := a.taxReports.CreateSnapshot(c, uid, c.ClientIP(), snapshotCreateReq.Year, fiscalYearStart, snapshotCreateReq.BaseCurrency)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxReportSnapshotCreateHandler] failed to create tax report snapshot for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return snapshot, nil
}

// TaxReportSnapshotDeleteHandler deletes a tax report snapshot
func (a *TaxReportsApi) TaxReportSnapshotDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var snapshotDeleteReq models.TaxReportSnapshotDeleteRequest
	err := c.ShouldBindJSON(&snapshotDeleteReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxReportSnapshotDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.taxReports.DeleteSnapshot(c, uid, snapshotDeleteReq.SnapshotId)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxReportSnapshotDeleteHandler] failed to delete tax report snapshot \"snapshot_id:%d\" for user \"uid:%d\", because %s", snapshotDeleteReq.SnapshotId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// TaxDailyExchangeRateListHandler returns all user custom daily exchange rates
func (a *TaxReportsApi) TaxDailyExchangeRateListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	dailyRates, err := a.taxReports.GetAllDailyExchangeRates(c, uid)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxDailyExchangeRateListHandler] failed to get daily exchange rates for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	items := make([]*models.UserCustomDailyExchangeRateResponse, 0, len(dailyRates))

	for i := 0; i < len(dailyRates); i++ {
		transactionDate := dailyRates[i].TransactionDate
		items = append(items, &models.UserCustomDailyExchangeRateResponse{
			Currency: dailyRates[i].Currency,
			Date:     fmt.Sprintf("%04d-%02d-%02d", transactionDate/10000, (transactionDate/100)%100, transactionDate%100),
			Rate:     utils.Float64ToString(float64(dailyRates[i].Rate) / float64(models.UserCustomExchangeRateFactorInDatabase)),
		})
	}

	return items, nil
}

// TaxDailyExchangeRateUpdateHandler creates or updates user custom daily exchange rate
func (a *TaxReportsApi) TaxDailyExchangeRateUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var dailyRateUpdateReq models.UserCustomDailyExchangeRateUpdateRequest
	err := c.ShouldBindJSON(&dailyRateUpdateReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxDailyExchangeRateUpdateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	dailyRate, err := a.taxReports.UpdateDailyExchangeRate(c, uid, dailyRateUpdateReq.Currency, dailyRateUpdateReq.Date, dailyRateUpdateReq.Rate)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxDailyExchangeRateUpdateHandler] failed to update daily exchange rate for user \"uid:%d\", currency \"%s\", date \"%s\", because %s", uid, dailyRateUpdateReq.Currency, dailyRateUpdateReq.Date, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return dailyRate, nil
}

// TaxDailyExchangeRateDeleteHandler deletes user custom daily exchange rate
func (a *TaxReportsApi) TaxDailyExchangeRateDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var dailyRateDeleteReq models.UserCustomDailyExchangeRateDeleteRequest
	err := c.ShouldBindJSON(&dailyRateDeleteReq)

	if err != nil {
		log.Warnf(c, "[tax_reports.TaxDailyExchangeRateDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.taxReports.DeleteDailyExchangeRate(c, uid, dailyRateDeleteReq.Currency, dailyRateDeleteReq.Date)

	if err != nil {
		log.Errorf(c, "[tax_reports.TaxDailyExchangeRateDeleteHandler] failed to delete daily exchange rate for user \"uid:%d\", currency \"%s\", date \"%s\", because %s", uid, dailyRateDeleteReq.Currency, dailyRateDeleteReq.Date, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}
