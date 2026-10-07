export interface TaxSummaryRequest {
    readonly year: number;
    readonly fiscalYearStart: string; // MM-DD, empty means default (January 1)
    readonly baseCurrency: string;
    readonly snapshotId: string; // Optional snapshot id to compare, empty means compare with latest snapshot
}

export enum TaxExchangeRateSource {
    NotUsed = 0,
    DailyExact = 1,
    DailyNearest = 2,
    LatestFallback = 3,
    Unavailable = 4
}

export enum TaxExchangeRateWarningReason {
    DailyNotFound = 1,
    AllNotFound = 2
}

export enum TaxReportCategoryChangeType {
    Deductible = 1,
    Refund = 2
}

export interface TaxTransaction {
    readonly id: string;
    readonly time: number;
    readonly utcOffset: number;
    readonly accountId: string;
    readonly accountName: string;
    readonly categoryId: string;
    readonly comment: string;
    readonly originalAmount: string;
    readonly originalCurrency: string;
    readonly exchangeRateDate: string;
    readonly exchangeRate: string;
    readonly exchangeRateSource: TaxExchangeRateSource;
    readonly convertedAmount: string;
    readonly hasReceipt: boolean;
}

export interface TaxCategorySummary {
    readonly categoryId: string;
    readonly parentCategoryId: string;
    readonly categoryName: string;
    readonly amount: string;
    readonly transactionCount: number;
    readonly missingReceiptCount: number;
    readonly missingReceiptAmount: string;
    readonly transactions: TaxTransaction[];
}

export interface TaxExcludedTransaction {
    readonly id: string;
    readonly time: number;
    readonly utcOffset: number;
    readonly accountId: string;
    readonly accountName: string;
    readonly destinationAccountId: string;
    readonly destinationAccountName: string;
    readonly comment: string;
    readonly originalAmount: string;
    readonly originalCurrency: string;
    readonly exchangeRateDate: string;
    readonly exchangeRate: string;
    readonly exchangeRateSource: TaxExchangeRateSource;
    readonly convertedAmount: string;
    readonly hasReceipt: boolean;
}

export interface TaxExcludedTransactions {
    readonly transferTransactions: TaxExcludedTransaction[];
    readonly accountRepaymentTransactions: TaxExcludedTransaction[];
    readonly totalTransferAmount: string;
    readonly totalAccountRepaymentAmount: string;
}

export interface TaxRateWarning {
    readonly currency: string;
    readonly date: string;
    readonly reason: TaxExchangeRateWarningReason;
}

export interface TaxCategoryChange {
    readonly categoryId: string;
    readonly categoryName: string;
    readonly type: TaxReportCategoryChangeType;
    readonly beforeAmount: string;
    readonly afterAmount: string;
    readonly delta: string;
}

export interface TaxSummaryChanges {
    readonly snapshotId: string;
    readonly snapshotCreatedUnixTime: number;
    readonly totalDeductibleAmountDelta: string;
    readonly totalRefundAmountDelta: string;
    readonly totalNetDeductibleAmountDelta: string;
    readonly categoryChanges: TaxCategoryChange[];
}

export interface TaxSummary {
    readonly year: number;
    readonly startTime: number;
    readonly endTime: number;
    readonly fiscalYearStart: string;
    readonly baseCurrency: string;
    readonly generatedUnixTime: number;
    readonly totalDeductibleAmount: string;
    readonly totalRefundAmount: string;
    readonly totalNetDeductibleAmount: string;
    readonly categories: TaxCategorySummary[];
    readonly refunds: TaxCategorySummary[];
    readonly excludedTransactions: TaxExcludedTransactions;
    readonly rateWarnings: TaxRateWarning[];
    readonly comparedSnapshotId?: string;
    readonly changes?: TaxSummaryChanges;
}

export interface UserCustomDailyExchangeRate {
    readonly currency: string;
    readonly date: string; // yyyy-mm-dd
    readonly rate: string; // amount of base currency per 1 unit of this currency
}

export interface UserCustomDailyExchangeRateUpdateRequest {
    readonly currency: string;
    readonly date: string;
    readonly rate: string;
}

export interface UserCustomDailyExchangeRateDeleteRequest {
    readonly currency: string;
    readonly date: string;
}

export interface TaxReportSnapshotInfo {
    readonly snapshotId: string;
    readonly year: number;
    readonly fiscalYearStart: string;
    readonly baseCurrency: string;
    readonly totalDeductibleAmount: string;
    readonly totalRefundAmount: string;
    readonly totalNetDeductibleAmount: string;
    readonly createdUnixTime: number;
}

export interface TaxReportSnapshotListResponse {
    readonly items: TaxReportSnapshotInfo[];
}

export interface TaxReportSnapshotCreateRequest {
    readonly year: number;
    readonly fiscalYearStart: string;
    readonly baseCurrency: string;
}

export interface TaxReportSnapshotDeleteRequest {
    readonly snapshotId: string;
}
