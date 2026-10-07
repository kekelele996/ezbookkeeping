import type { AccountInfoResponse } from '@/models/account.ts';
import type { TransactionCategoryInfoResponse } from '@/models/transaction_category.ts';
import type { TransactionTagInfoResponse } from '@/models/transaction_tag.ts';
import type {
    TransactionGeoLocationResponse,
    TransactionInsightDataItem
} from '@/models/transaction.ts';
import type { LatestExchangeRate } from '@/models/exchange_rate.ts';

export const INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION = 1;

export interface InsightsExplorerSnapshotFilter {
    readonly dateRangeType: number;
    readonly startTime: number;
    readonly endTime: number;
}

export interface InsightsExplorerSnapshotFrozenExchangeRates {
    readonly updateTime: number;
    readonly rates: LatestExchangeRate[];
}

export interface InsightsExplorerSnapshotQueryTransactions {
    readonly queryId: string;
    readonly transactionIds: string[];
}

export interface InsightsExplorerSnapshotPayload {
    readonly version: number;
    readonly frozenTime: number;
    readonly defaultCurrency: string;
    readonly filter: InsightsExplorerSnapshotFilter;
    readonly timezoneUsedForDateRange: number;
    readonly explorerName: string;
    readonly explorer: Record<string, string | number | object[]>;
    readonly exchangeRates: InsightsExplorerSnapshotFrozenExchangeRates;
    readonly queryTransactions: InsightsExplorerSnapshotQueryTransactions[];
    readonly transactions: TransactionInsightDataItem[];
}

export interface InsightsExplorerSnapshotCreateRequest {
    readonly name: string;
    readonly explorerId: string;
    readonly data: InsightsExplorerSnapshotPayload;
}

export interface InsightsExplorerSnapshotDeleteRequest {
    readonly id: string;
}

export interface InsightsExplorerSnapshotInfoResponse {
    readonly id: string;
    readonly explorerId: string;
    readonly name: string;
    readonly createdUnixTime: number;
    readonly data?: InsightsExplorerSnapshotPayload;
}

export class InsightsExplorerSnapshot implements InsightsExplorerSnapshotInfoResponse {
    public id: string;
    public explorerId: string;
    public name: string;
    public createdUnixTime: number;
    public payload: InsightsExplorerSnapshotPayload | null;

    private constructor(id: string, explorerId: string, name: string, createdUnixTime: number, payload: InsightsExplorerSnapshotPayload | null) {
        this.id = id;
        this.explorerId = explorerId;
        this.name = name;
        this.createdUnixTime = createdUnixTime;
        this.payload = payload;
    }

    public static of(snapshotResponse: InsightsExplorerSnapshotInfoResponse): InsightsExplorerSnapshot {
        return new InsightsExplorerSnapshot(
            snapshotResponse.id,
            snapshotResponse.explorerId,
            snapshotResponse.name,
            snapshotResponse.createdUnixTime,
            snapshotResponse.data ? parseSnapshotPayload(snapshotResponse.data) : null
        );
    }
}

export function parseSnapshotPayload(data: unknown): InsightsExplorerSnapshotPayload | null {
    if (typeof data !== 'object' || data === null) {
        return null;
    }

    const payload = data as Record<string, unknown>;

    if (payload['version'] !== INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION) {
        return null;
    }

    if (typeof payload['frozenTime'] !== 'number'
        || typeof payload['defaultCurrency'] !== 'string'
        || typeof payload['timezoneUsedForDateRange'] !== 'number'
        || typeof payload['explorerName'] !== 'string'
        || typeof payload['explorer'] !== 'object' || payload['explorer'] === null
        || !Array.isArray(payload['queryTransactions'])
        || !Array.isArray(payload['transactions'])) {
        return null;
    }

    const filterData = payload['filter'];

    if (typeof filterData !== 'object' || filterData === null) {
        return null;
    }

    const filter = filterData as Record<string, unknown>;

    if (typeof filter['dateRangeType'] !== 'number'
        || typeof filter['startTime'] !== 'number'
        || typeof filter['endTime'] !== 'number') {
        return null;
    }

    const exchangeRatesData = payload['exchangeRates'];

    if (typeof exchangeRatesData !== 'object' || exchangeRatesData === null) {
        return null;
    }

    const exchangeRates = exchangeRatesData as Record<string, unknown>;

    if (typeof exchangeRates['updateTime'] !== 'number' || !Array.isArray(exchangeRates['rates'])) {
        return null;
    }

    return data as InsightsExplorerSnapshotPayload;
}

export type FrozenAccountInfo = AccountInfoResponse;
export type FrozenTransactionCategoryInfo = TransactionCategoryInfoResponse;
export type FrozenTransactionTagInfo = TransactionTagInfoResponse;
export type FrozenTransactionGeoLocation = TransactionGeoLocationResponse;
