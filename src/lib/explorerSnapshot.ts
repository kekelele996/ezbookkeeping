import { NormalizedText } from '@/core/text.ts';
import { TimezoneTypeForStatistics } from '@/core/timezone.ts';
import type { DateTime } from '@/core/datetime.ts';

import type { AccountInfoResponse } from '@/models/account.ts';
import type { TransactionCategoryInfoResponse } from '@/models/transaction_category.ts';
import type { TransactionTagInfoResponse } from '@/models/transaction_tag.ts';
import type {
    TransactionGeoLocationResponse,
    TransactionInsightDataItem,
    TransactionInsightDataItemWithQueryIndexes
} from '@/models/transaction.ts';
import type { LatestExchangeRateResponse } from '@/models/exchange_rate.ts';
import {
    type InsightsExplorerMatchContext,
    TransactionExplorerQuery
} from '@/models/explorer.ts';
import {
    INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION,
    type InsightsExplorerSnapshotFilter,
    type InsightsExplorerSnapshotPayload,
    type InsightsExplorerSnapshotQueryTransactions
} from '@/models/explorer_snapshot.ts';

import {
    parseDateTimeFromUnixTime,
    parseDateTimeFromUnixTimeWithTimezoneOffset
} from '@/lib/datetime.ts';
import { getCurrentUnixTime } from '@/lib/datetime.ts';

export interface BuildSnapshotPayloadOptions {
    defaultCurrency: string;
    filter: InsightsExplorerSnapshotFilter;
    timezoneUsedForDateRange: number;
    explorerName: string;
    explorerData: Record<string, string | number | object[]>;
    queries: TransactionExplorerQuery[];
    transactionsWithQueryIndexes: TransactionInsightDataItemWithQueryIndexes[];
    exchangeRates?: LatestExchangeRateResponse;
    frozenTime?: number;
}

export enum SnapshotDifferenceRemovedReason {
    Deleted = 'deleted',
    NotMatched = 'not_matched'
}

export enum SnapshotDifferenceChangeField {
    Type = 'type',
    Time = 'time',
    UtcOffset = 'utcOffset',
    SourceAmount = 'sourceAmount',
    DestinationAmount = 'destinationAmount',
    PrimaryCategory = 'primaryCategory',
    SecondaryCategory = 'secondaryCategory',
    SourceAccount = 'sourceAccount',
    DestinationAccount = 'destinationAccount',
    Tags = 'tags',
    Comment = 'comment',
    HideAmount = 'hideAmount',
    GeoLocation = 'geoLocation'
}

const RENAME_CHANGE_FIELDS: Record<string, boolean> = {
    [SnapshotDifferenceChangeField.PrimaryCategory]: true,
    [SnapshotDifferenceChangeField.SecondaryCategory]: true,
    [SnapshotDifferenceChangeField.SourceAccount]: true,
    [SnapshotDifferenceChangeField.DestinationAccount]: true,
    [SnapshotDifferenceChangeField.Tags]: true
};

export interface SnapshotDifferenceChange {
    field: SnapshotDifferenceChangeField;
    oldValue: string | number | boolean;
    newValue: string | number | boolean;
}

export interface SnapshotDifferenceRemovedItem {
    transaction: TransactionInsightDataItem;
    reason: SnapshotDifferenceRemovedReason;
}

export interface SnapshotDifferenceModifiedItem {
    transaction: TransactionInsightDataItem;
    frozenTransaction: TransactionInsightDataItem;
    changes: SnapshotDifferenceChange[];
    renamedOnly: boolean;
}

export interface SnapshotDifference {
    added: TransactionInsightDataItem[];
    removed: SnapshotDifferenceRemovedItem[];
    modified: SnapshotDifferenceModifiedItem[];
    hasDifference: boolean;
}

function serializeCategory(category: TransactionCategoryInfoResponse): TransactionCategoryInfoResponse {
    return {
        id: category.id,
        name: category.name,
        parentId: category.parentId,
        type: category.type,
        icon: category.icon,
        iconType: category.iconType,
        color: category.color,
        comment: category.comment,
        displayOrder: category.displayOrder,
        hidden: category.hidden
    };
}

function serializeAccount(account: AccountInfoResponse): AccountInfoResponse {
    return {
        id: account.id,
        name: account.name,
        parentId: account.parentId,
        category: account.category,
        type: account.type,
        icon: account.icon,
        iconType: account.iconType,
        color: account.color,
        currency: account.currency,
        balance: account.balance,
        comment: account.comment,
        displayOrder: account.displayOrder,
        hidden: account.hidden
    };
}

function serializeTag(tag: TransactionTagInfoResponse): TransactionTagInfoResponse {
    return {
        id: tag.id,
        name: tag.name,
        groupId: tag.groupId,
        displayOrder: tag.displayOrder,
        hidden: tag.hidden
    };
}

function serializeTransaction(transaction: TransactionInsightDataItem): TransactionInsightDataItem {
    const result: TransactionInsightDataItem = {
        id: transaction.id,
        timeSequenceId: transaction.timeSequenceId,
        type: transaction.type,
        categoryId: transaction.categoryId,
        time: transaction.time,
        utcOffset: transaction.utcOffset,
        sourceAccountId: transaction.sourceAccountId,
        destinationAccountId: transaction.destinationAccountId,
        sourceAmount: transaction.sourceAmount,
        destinationAmount: transaction.destinationAmount,
        hideAmount: transaction.hideAmount,
        tagIds: [ ...transaction.tagIds ],
        comment: transaction.comment,
        editable: false,
        primaryCategory: serializeCategory(transaction.primaryCategory),
        primaryCategoryName: transaction.primaryCategoryName,
        secondaryCategory: serializeCategory(transaction.secondaryCategory),
        secondaryCategoryName: transaction.secondaryCategoryName,
        sourceAccount: serializeAccount(transaction.sourceAccount),
        sourceAccountName: transaction.sourceAccountName,
        destinationAccount: transaction.destinationAccount ? serializeAccount(transaction.destinationAccount) : undefined,
        destinationAccountName: transaction.destinationAccount ? transaction.destinationAccountName : undefined,
        geoLocation: transaction.geoLocation ? {
            latitude: transaction.geoLocation.latitude,
            longitude: transaction.geoLocation.longitude
        } : undefined,
        tags: transaction.tags.map(tag => serializeTag(tag))
    };

    return result;
}

// Builds an immutable frozen payload from the current live exploration view
export function buildSnapshotPayload(options: BuildSnapshotPayloadOptions): InsightsExplorerSnapshotPayload {
    const transactionsMap = new Map<string, TransactionInsightDataItem>();
    const queryTransactions: InsightsExplorerSnapshotQueryTransactions[] = options.queries.map((query, queryIndex) => {
        const transactionIds: string[] = [];

        for (const item of options.transactionsWithQueryIndexes) {
            if (item.queryIndexes.includes(queryIndex)) {
                transactionIds.push(item.id);

                if (!transactionsMap.has(item.id)) {
                    transactionsMap.set(item.id, serializeTransaction(item));
                }
            }
        }

        return {
            queryId: query.id,
            transactionIds: transactionIds
        };
    });

    // Transactions matched by no query (e.g. table shows all when query source is empty) are not frozen
    const transactions: TransactionInsightDataItem[] = [];

    for (const transaction of transactionsMap.values()) {
        transactions.push(transaction);
    }

    return {
        version: INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION,
        frozenTime: options.frozenTime ?? getCurrentUnixTime(),
        defaultCurrency: options.defaultCurrency,
        filter: {
            dateRangeType: options.filter.dateRangeType,
            startTime: options.filter.startTime,
            endTime: options.filter.endTime
        },
        timezoneUsedForDateRange: options.timezoneUsedForDateRange,
        explorerName: options.explorerName,
        explorer: options.explorerData,
        exchangeRates: {
            updateTime: options.exchangeRates?.updateTime ?? 0,
            rates: (options.exchangeRates?.exchangeRates ?? []).map(rate => ({
                currency: rate.currency,
                rate: rate.rate
            }))
        },
        queryTransactions: queryTransactions,
        transactions: transactions
    };
}

// Returns frozen transactions as enriched insight items for replaying aggregation
export function rehydrateSnapshotTransactions(payload: InsightsExplorerSnapshotPayload): TransactionInsightDataItem[] {
    return payload.transactions.map(transaction => serializeTransaction(transaction));
}

function buildMatchContext(timezoneUsedForDateRange: number, transaction: TransactionInsightDataItem): InsightsExplorerMatchContext {
    let cachedTransactionDateTime: DateTime | undefined = undefined;

    return {
        getTransactionDateTime(): DateTime {
            if (!cachedTransactionDateTime) {
                let transactionTimeUtfOffset: number | undefined = undefined;

                if (timezoneUsedForDateRange === TimezoneTypeForStatistics.TransactionTimezone.type) {
                    transactionTimeUtfOffset = transaction.utcOffset;
                }

                cachedTransactionDateTime = isDefined(transactionTimeUtfOffset)
                    ? parseDateTimeFromUnixTimeWithTimezoneOffset(transaction.time, transactionTimeUtfOffset)
                    : parseDateTimeFromUnixTime(transaction.time);
            }

            return cachedTransactionDateTime;
        },
        getNormalizedDescription(): NormalizedText {
            return NormalizedText.of(transaction.comment ? transaction.comment : '');
        }
    };
}

function isDefined(value: unknown): value is number {
    return value !== undefined && value !== null;
}

function getTagNames(transaction: TransactionInsightDataItem): string {
    return transaction.tagIds
        .map(tagId => transaction.tags.find(tag => tag.id === tagId)?.name ?? '')
        .filter(name => !!name)
        .sort()
        .join(', ');
}

function stringifyGeoLocation(geoLocation: TransactionGeoLocationResponse | undefined): string {
    if (!geoLocation) {
        return '';
    }

    return `${geoLocation.latitude},${geoLocation.longitude}`;
}

// Replays frozen queries against current live transactions and returns per-transaction differences
export function diffSnapshotWithCurrentTransactions(payload: InsightsExplorerSnapshotPayload, liveTransactions: TransactionInsightDataItem[]): SnapshotDifference {
    const frozenMap = new Map<string, TransactionInsightDataItem>();
    const frozenMatchedIds = new Set<string>();

    for (const transaction of payload.transactions) {
        frozenMap.set(transaction.id, transaction);
    }

    for (const queryResult of payload.queryTransactions) {
        for (const transactionId of queryResult.transactionIds) {
            frozenMatchedIds.add(transactionId);
        }
    }

    const queryDataItems = Array.isArray(payload.explorer['queries']) ? payload.explorer['queries'] as object[] : [];
    const queries: TransactionExplorerQuery[] = [];

    for (const queryItem of queryDataItems) {
        const query = TransactionExplorerQuery.parse(queryItem);

        if (query) {
            queries.push(query);
        }
    }

    const liveMap = new Map<string, TransactionInsightDataItem>();

    for (const transaction of liveTransactions) {
        liveMap.set(transaction.id, transaction);
    }

    const added: TransactionInsightDataItem[] = [];
    const removed: SnapshotDifferenceRemovedItem[] = [];
    const modified: SnapshotDifferenceModifiedItem[] = [];
    const currentMatchedIds = new Set<string>();

    for (const transaction of liveTransactions) {
        let matched = queries.length < 1;

        if (!matched) {
            const matchContext = buildMatchContext(payload.timezoneUsedForDateRange, transaction);

            for (const query of queries) {
                if (query.match(transaction, matchContext)) {
                    matched = true;
                    break;
                }
            }
        }

        if (!matched) {
            continue;
        }

        currentMatchedIds.add(transaction.id);

        const frozenTransaction = frozenMap.get(transaction.id);

        if (!frozenTransaction) {
            added.push(transaction);
        } else {
            const changes = getTransactionChanges(frozenTransaction, transaction);

            if (changes.length > 0) {
                let structuralChangeCount = 0;

                for (const change of changes) {
                    if (!RENAME_CHANGE_FIELDS[change.field]) {
                        structuralChangeCount++;
                    }
                }

                modified.push({
                    transaction: transaction,
                    frozenTransaction: frozenTransaction,
                    changes: changes,
                    renamedOnly: structuralChangeCount === 0
                });
            }
        }
    }

    for (const frozenId of frozenMatchedIds) {
        if (currentMatchedIds.has(frozenId)) {
            continue;
        }

        const frozenTransaction = frozenMap.get(frozenId);

        if (!frozenTransaction) {
            continue;
        }

        removed.push({
            transaction: frozenTransaction,
            reason: liveMap.has(frozenId) ? SnapshotDifferenceRemovedReason.NotMatched : SnapshotDifferenceRemovedReason.Deleted
        });
    }

    return {
        added: added,
        removed: removed,
        modified: modified,
        hasDifference: added.length > 0 || removed.length > 0 || modified.length > 0
    };
}

function appendChange(changes: SnapshotDifferenceChange[], field: SnapshotDifferenceChangeField, oldValue: string | number | boolean, newValue: string | number | boolean): void {
    if (oldValue !== newValue) {
        changes.push({
            field: field,
            oldValue: oldValue,
            newValue: newValue
        });
    }
}

function getTransactionChanges(frozenTransaction: TransactionInsightDataItem, currentTransaction: TransactionInsightDataItem): SnapshotDifferenceChange[] {
    const changes: SnapshotDifferenceChange[] = [];

    appendChange(changes, SnapshotDifferenceChangeField.Type, frozenTransaction.type, currentTransaction.type);
    appendChange(changes, SnapshotDifferenceChangeField.Time, frozenTransaction.time, currentTransaction.time);
    appendChange(changes, SnapshotDifferenceChangeField.UtcOffset, frozenTransaction.utcOffset, currentTransaction.utcOffset);
    appendChange(changes, SnapshotDifferenceChangeField.SourceAmount, frozenTransaction.sourceAmount, currentTransaction.sourceAmount);
    appendChange(changes, SnapshotDifferenceChangeField.DestinationAmount, frozenTransaction.destinationAmount, currentTransaction.destinationAmount);
    appendChange(changes, SnapshotDifferenceChangeField.HideAmount, frozenTransaction.hideAmount, currentTransaction.hideAmount);
    appendChange(changes, SnapshotDifferenceChangeField.Comment, frozenTransaction.comment ?? '', currentTransaction.comment ?? '');
    appendChange(changes, SnapshotDifferenceChangeField.GeoLocation, stringifyGeoLocation(frozenTransaction.geoLocation), stringifyGeoLocation(currentTransaction.geoLocation));

    const frozenTagIds = [ ...frozenTransaction.tagIds ].sort();
    const currentTagIds = [ ...currentTransaction.tagIds ].sort();
    const frozenTagIdsText = frozenTagIds.join(',');
    const currentTagIdsText = currentTagIds.join(',');

    if (frozenTagIdsText !== currentTagIdsText) {
        appendChange(changes, SnapshotDifferenceChangeField.Tags, getTagNames(frozenTransaction), getTagNames(currentTransaction));
    } else {
        const frozenTagNames = frozenTagIds.map(tagId => frozenTransaction.tags.find(tag => tag.id === tagId)?.name ?? '');
        const currentTagNames = currentTagIds.map(tagId => currentTransaction.tags.find(tag => tag.id === tagId)?.name ?? '');

        for (let i = 0; i < frozenTagIds.length; i++) {
            if ((frozenTagNames[i] ?? '') !== (currentTagNames[i] ?? '')) {
                appendChange(changes, SnapshotDifferenceChangeField.Tags, getTagNames(frozenTransaction), getTagNames(currentTransaction));
                break;
            }
        }
    }

    // Category: leaf category id change covers both parent and leaf change
    if (frozenTransaction.categoryId !== currentTransaction.categoryId) {
        appendChange(changes, SnapshotDifferenceChangeField.SecondaryCategory,
            frozenTransaction.secondaryCategoryName || frozenTransaction.secondaryCategory.name,
            currentTransaction.secondaryCategoryName || currentTransaction.secondaryCategory.name);
    } else {
        if (frozenTransaction.primaryCategory.id !== currentTransaction.primaryCategory.id) {
            appendChange(changes, SnapshotDifferenceChangeField.PrimaryCategory,
                frozenTransaction.primaryCategoryName || frozenTransaction.primaryCategory.name,
                currentTransaction.primaryCategoryName || currentTransaction.primaryCategory.name);
        } else {
            appendChange(changes, SnapshotDifferenceChangeField.PrimaryCategory,
                frozenTransaction.primaryCategoryName || frozenTransaction.primaryCategory.name,
                currentTransaction.primaryCategoryName || currentTransaction.primaryCategory.name);
            appendChange(changes, SnapshotDifferenceChangeField.SecondaryCategory,
                frozenTransaction.secondaryCategoryName || frozenTransaction.secondaryCategory.name,
                currentTransaction.secondaryCategoryName || currentTransaction.secondaryCategory.name);
        }
    }

    if (frozenTransaction.sourceAccountId !== currentTransaction.sourceAccountId) {
        appendChange(changes, SnapshotDifferenceChangeField.SourceAccount,
            frozenTransaction.sourceAccountName || frozenTransaction.sourceAccount.name,
            currentTransaction.sourceAccountName || currentTransaction.sourceAccount.name);
    } else {
        appendChange(changes, SnapshotDifferenceChangeField.SourceAccount,
            frozenTransaction.sourceAccountName || frozenTransaction.sourceAccount.name,
            currentTransaction.sourceAccountName || currentTransaction.sourceAccount.name);
    }

    const frozenDestinationAccountId = frozenTransaction.destinationAccountId || '0';
    const currentDestinationAccountId = currentTransaction.destinationAccountId || '0';

    if (frozenDestinationAccountId !== currentDestinationAccountId) {
        appendChange(changes, SnapshotDifferenceChangeField.DestinationAccount,
            frozenTransaction.destinationAccountName || frozenTransaction.destinationAccount?.name || '',
            currentTransaction.destinationAccountName || currentTransaction.destinationAccount?.name || '');
    } else {
        appendChange(changes, SnapshotDifferenceChangeField.DestinationAccount,
            frozenTransaction.destinationAccountName || frozenTransaction.destinationAccount?.name || '',
            currentTransaction.destinationAccountName || currentTransaction.destinationAccount?.name || '');
    }

    return changes;
}
