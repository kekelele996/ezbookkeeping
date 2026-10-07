import { describe, expect, it } from 'vitest';

import {
    TransactionExplorerConditionField,
    TransactionExplorerConditionOperatorType
} from '@/core/explorer.ts';
import { TransactionType } from '@/core/transaction.ts';

import type { AccountInfoResponse } from '@/models/account.ts';
import type { TransactionCategoryInfoResponse } from '@/models/transaction_category.ts';
import type { TransactionInsightDataItem } from '@/models/transaction.ts';
import { TransactionExplorerQuery } from '@/models/explorer.ts';

import {
    type BuildSnapshotPayloadOptions,
    SnapshotDifferenceChangeField,
    SnapshotDifferenceRemovedReason,
    buildSnapshotPayload,
    diffSnapshotWithCurrentTransactions,
    rehydrateSnapshotTransactions
} from '@/lib/explorerSnapshot.ts';
import { parseSnapshotPayload, INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION } from '@/models/explorer_snapshot.ts';

function createTestCategory(id: string, name: string, parentId = '0'): TransactionCategoryInfoResponse {
    return {
        id: id,
        name: name,
        parentId: parentId,
        type: 2,
        icon: '',
        iconType: 0,
        color: '000000',
        comment: '',
        displayOrder: 1,
        hidden: false
    };
}

function createTestAccount(id: string, name: string, currency = 'CNY'): AccountInfoResponse {
    return {
        id: id,
        name: name,
        parentId: '0',
        category: 1,
        type: 1,
        icon: '',
        iconType: 0,
        color: '000000',
        currency: currency,
        balance: '0',
        comment: '',
        displayOrder: 1,
        hidden: false
    };
}

function createTestTransaction(overrides: Partial<TransactionInsightDataItem> = {}): TransactionInsightDataItem {
    return {
        id: '1',
        timeSequenceId: '1',
        type: TransactionType.Expense,
        categoryId: 'c2',
        time: 1728000000,
        utcOffset: 0,
        sourceAccountId: 'a1',
        destinationAccountId: '0',
        sourceAmount: -1000,
        destinationAmount: 0,
        hideAmount: false,
        tagIds: [],
        comment: '',
        editable: true,
        primaryCategory: createTestCategory('c1', 'Food'),
        primaryCategoryName: 'Food',
        secondaryCategory: createTestCategory('c2', 'Meal', 'c1'),
        secondaryCategoryName: 'Meal',
        sourceAccount: createTestAccount('a1', 'Wallet'),
        sourceAccountName: 'Wallet',
        tags: [],
        pictures: [
            {
                pictureId: 'p1',
                originalUrl: 'https://example.com/expired-signed-url'
            }
        ],
        ...overrides
    };
}

function createBuildOptions(transactionsWithQueryIndexes: Array<TransactionInsightDataItem & { queryIndexes: number[] }>, queries?: TransactionExplorerQuery[]): BuildSnapshotPayloadOptions {
    return {
        defaultCurrency: 'CNY',
        filter: {
            dateRangeType: 2,
            startTime: 1727900000,
            endTime: 1728100000
        },
        timezoneUsedForDateRange: 0,
        explorerName: 'monthly',
        explorerData: {
            queries: (queries ?? []).map(query => query.toJsonObject())
        } as Record<string, string | number | object[]>,
        queries: queries ?? [],
        transactionsWithQueryIndexes: transactionsWithQueryIndexes,
        exchangeRates: {
            dataSource: 'ezbookkeeping',
            referenceUrl: '',
            updateTime: 1727900000,
            baseCurrency: 'CNY',
            exchangeRates: [
                {
                    currency: 'USD',
                    rate: '7.10'
                }
            ]
        },
        frozenTime: 1728000000
    };
}

describe('buildSnapshotPayload', () => {
    it('should freeze deduplicated transactions with query membership and slim fields', () => {
        const query1 = TransactionExplorerQuery.create('q1');
        const query2 = TransactionExplorerQuery.create('q2');

        const t1 = createTestTransaction({ id: '1' });
        const t2 = createTestTransaction({ id: '2', sourceAmount: -2000 });
        const t3 = createTestTransaction({ id: '3', sourceAmount: -3000 });

        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] },
            { ...t2, queryIndexes: [ 0, 1 ] },
            { ...t3, queryIndexes: [ 1 ] }
        ], [ query1, query2 ]));

        expect(payload.version).toBe(INSIGHTS_EXPLORER_SNAPSHOT_PAYLOAD_VERSION);
        expect(payload.frozenTime).toBe(1728000000);
        expect(payload.explorerName).toBe('monthly');
        expect(payload.exchangeRates.rates).toHaveLength(1);
        expect(payload.exchangeRates.rates[0]!.currency).toBe('USD');
        expect(payload.exchangeRates.rates[0]!.rate).toBe('7.10');
        expect(payload.queryTransactions).toHaveLength(2);
        expect(payload.queryTransactions[0]!.queryId).toBe('q1');
        expect(payload.queryTransactions[0]!.transactionIds).toEqual([ '1', '2' ]);
        expect(payload.queryTransactions[1]!.transactionIds).toEqual([ '2', '3' ]);
        expect(payload.transactions).toHaveLength(3);
        expect(payload.transactions.find(item => item.id === '2')!.sourceAmount).toBe(-2000);

        // pictures are stripped and editable flag reset
        for (const transaction of payload.transactions) {
            expect(transaction.pictures).toBeUndefined();
            expect(transaction.editable).toBe(false);
        }

        expect(parseSnapshotPayload(payload)).not.toBeNull();
    });
});

describe('rehydrateSnapshotTransactions', () => {
    it('should restore frozen transactions for aggregation replay', () => {
        const query = TransactionExplorerQuery.create('q1');
        const t1 = createTestTransaction({ id: '1' });
        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] }
        ], [ query ]));

        const restored = rehydrateSnapshotTransactions(payload);
        expect(restored).toHaveLength(1);
        expect(restored[0]!.id).toBe('1');
        expect(restored[0]!.primaryCategory.name).toBe('Food');
        expect(restored[0]!.secondaryCategory.name).toBe('Meal');
        expect(restored[0]!.sourceAccount.currency).toBe('CNY');
        expect(restored[0]!.sourceAccountName).toBe('Wallet');
    });
});

describe('diffSnapshotWithCurrentTransactions', () => {
    it('should report added, removed and modified transactions', () => {
        const query = TransactionExplorerQuery.create('q1'); // empty conditions match all

        const t1 = createTestTransaction({ id: '1', sourceAmount: -1000 });
        const t2 = createTestTransaction({ id: '2', sourceAmount: -2000 });
        const t3 = createTestTransaction({ id: '3', sourceAmount: -3000, comment: 'old note' });

        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] },
            { ...t2, queryIndexes: [ 0 ] },
            { ...t3, queryIndexes: [ 0 ] }
        ], [ query ]));

        const liveT1 = createTestTransaction({ id: '1', sourceAmount: -1000 });
        const liveT2 = createTestTransaction({ id: '2', sourceAmount: -2500 });
        const liveT4 = createTestTransaction({ id: '4', sourceAmount: -4000 });

        const difference = diffSnapshotWithCurrentTransactions(payload, [ liveT1, liveT2, liveT4 ]);

        expect(difference.hasDifference).toBe(true);
        expect(difference.added).toHaveLength(1);
        expect(difference.added[0]!.id).toBe('4');
        expect(difference.removed).toHaveLength(1);
        expect(difference.removed[0]!.transaction.id).toBe('3');
        expect(difference.removed[0]!.reason).toBe(SnapshotDifferenceRemovedReason.Deleted);

        expect(difference.modified).toHaveLength(1);
        expect(difference.modified[0]!.transaction.id).toBe('2');
        expect(difference.modified[0]!.renamedOnly).toBe(false);

        const amountChange = difference.modified[0]!.changes.find(change => change.field === SnapshotDifferenceChangeField.SourceAmount);
        expect(amountChange).toBeDefined();
        expect(amountChange!.oldValue).toBe(-2000);
        expect(amountChange!.newValue).toBe(-2500);
    });

    it('should detect category rename as rename-only modification', () => {
        const query = TransactionExplorerQuery.create('q1');
        const t1 = createTestTransaction({ id: '1' });

        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] }
        ], [ query ]));

        const liveT1 = createTestTransaction({
            id: '1',
            secondaryCategory: createTestCategory('c2', 'Dining', 'c1'),
            secondaryCategoryName: 'Dining'
        });

        const difference = diffSnapshotWithCurrentTransactions(payload, [ liveT1 ]);

        expect(difference.added).toHaveLength(0);
        expect(difference.removed).toHaveLength(0);
        expect(difference.modified).toHaveLength(1);
        expect(difference.modified[0]!.renamedOnly).toBe(true);

        const categoryChange = difference.modified[0]!.changes.find(change => change.field === SnapshotDifferenceChangeField.SecondaryCategory);
        expect(categoryChange).toBeDefined();
        expect(categoryChange!.oldValue).toBe('Meal');
        expect(categoryChange!.newValue).toBe('Dining');
    });

    it('should mark disappeared but still existing transaction as not matched', () => {
        const queryJsonObject = {
            id: 'q1',
            name: '',
            conditions: [
                {
                    condition: {
                        field: TransactionExplorerConditionField.TransactionType.value,
                        operator: TransactionExplorerConditionOperatorType.In,
                        value: [ TransactionType.Expense ]
                    },
                    relation: 'first'
                }
            ]
        };
        const query = TransactionExplorerQuery.parse(queryJsonObject)!;
        expect(query).not.toBeNull();

        const t1 = createTestTransaction({ id: '1', type: TransactionType.Expense });
        const t2 = createTestTransaction({ id: '2', type: TransactionType.Income, sourceAmount: 5000 });

        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] },
            { ...t2, queryIndexes: [ 0 ] } // frozen membership records it even if type later changes
        ], [ query ]));

        const difference = diffSnapshotWithCurrentTransactions(payload, [
            createTestTransaction({ id: '1', type: TransactionType.Expense }),
            createTestTransaction({ id: '2', type: TransactionType.Income, sourceAmount: 5000 })
        ]);

        expect(difference.added).toHaveLength(0);
        expect(difference.removed).toHaveLength(1);
        expect(difference.removed[0]!.transaction.id).toBe('2');
        expect(difference.removed[0]!.reason).toBe(SnapshotDifferenceRemovedReason.NotMatched);
        expect(difference.modified).toHaveLength(0);
    });

    it('should return no difference when everything stays unchanged', () => {
        const query = TransactionExplorerQuery.create('q1');
        const t1 = createTestTransaction({ id: '1' });

        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] }
        ], [ query ]));

        const difference = diffSnapshotWithCurrentTransactions(payload, [ createTestTransaction({ id: '1' }) ]);

        expect(difference.hasDifference).toBe(false);
        expect(difference.added).toHaveLength(0);
        expect(difference.removed).toHaveLength(0);
        expect(difference.modified).toHaveLength(0);
    });
});

describe('parseSnapshotPayload', () => {
    it('should reject unsupported version or malformed data', () => {
        const query = TransactionExplorerQuery.create('q1');
        const t1 = createTestTransaction({ id: '1' });
        const payload = buildSnapshotPayload(createBuildOptions([
            { ...t1, queryIndexes: [ 0 ] }
        ], [ query ]));

        expect(parseSnapshotPayload({ ...payload, version: 999 })).toBeNull();
        expect(parseSnapshotPayload(null)).toBeNull();
        expect(parseSnapshotPayload('invalid')).toBeNull();
        expect(parseSnapshotPayload({ ...payload, filter: {} })).toBeNull();
    });
});
