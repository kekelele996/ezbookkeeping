<template>
    <v-dialog width="900" v-model="showState">
        <one-column-dialog-layout :title="tt('Snapshot Differences')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #content>
                <div class="d-flex align-center gap-2 py-2">
                    <v-chip color="success" variant="tonal" size="small" :prepend-icon="mdiPlus">
                        {{ tt('Added') }}: {{ difference?.added.length ?? 0 }}
                    </v-chip>
                    <v-chip color="error" variant="tonal" size="small" :prepend-icon="mdiMinus">
                        {{ tt('Removed') }}: {{ difference?.removed.length ?? 0 }}
                    </v-chip>
                    <v-chip color="warning" variant="tonal" size="small" :prepend-icon="mdiPencilOutline">
                        {{ tt('Modified') }}: {{ difference?.modified.length ?? 0 }}
                    </v-chip>
                </div>

                <v-alert v-if="difference && !difference.hasDifference" type="success" variant="tonal" density="comfortable" class="my-2">
                    {{ tt('The current data is exactly the same as when archived') }}
                </v-alert>

                <v-expansion-panels v-if="difference && difference.hasDifference" variant="accordion" multiple class="bg-transparent">
                    <v-expansion-panel v-if="difference.added.length > 0">
                        <v-expansion-panel-title>
                            <v-icon class="me-2 text-success" :icon="mdiPlus" size="18"/>
                            {{ tt('Added Transactions') }} ({{ difference.added.length }})
                        </v-expansion-panel-title>
                        <v-expansion-panel-text>
                            <div v-for="transaction in difference.added" :key="'added-' + transaction.id" class="snapshot-diff-row py-2">
                                <div class="d-flex align-center">
                                    <v-chip label variant="outlined" size="x-small"
                                            :class="{ 'text-income' : transaction.type === TransactionType.Income, 'text-expense': transaction.type === TransactionType.Expense }">
                                        {{ getTransactionTypeName(transaction.type) }}
                                    </v-chip>
                                    <span class="ms-2">{{ formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(transaction.time)) }}</span>
                                    <span class="ms-2">{{ transaction.secondaryCategoryName }}</span>
                                    <span class="ms-2 text-medium-emphasis">{{ transaction.sourceAccountName }}</span>
                                    <v-spacer/>
                                    <span :class="{ 'text-expense': transaction.type === TransactionType.Expense, 'text-income': transaction.type === TransactionType.Income }">
                                        {{ getDisplaySourceAmount(transaction) }}
                                    </span>
                                </div>
                                <div class="text-body-small text-medium-emphasis" v-if="transaction.comment">{{ transaction.comment }}</div>
                            </div>
                        </v-expansion-panel-text>
                    </v-expansion-panel>

                    <v-expansion-panel v-if="difference.removed.length > 0">
                        <v-expansion-panel-title>
                            <v-icon class="me-2 text-error" :icon="mdiMinus" size="18"/>
                            {{ tt('Removed Transactions') }} ({{ difference.removed.length }})
                        </v-expansion-panel-title>
                        <v-expansion-panel-text>
                            <div v-for="item in difference.removed" :key="'removed-' + item.transaction.id" class="snapshot-diff-row py-2">
                                <div class="d-flex align-center">
                                    <v-chip label variant="outlined" size="x-small"
                                            :class="{ 'text-income' : item.transaction.type === TransactionType.Income, 'text-expense': item.transaction.type === TransactionType.Expense }">
                                        {{ getTransactionTypeName(item.transaction.type) }}
                                    </v-chip>
                                    <span class="ms-2">{{ formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(item.transaction.time)) }}</span>
                                    <span class="ms-2">{{ item.transaction.secondaryCategoryName }}</span>
                                    <span class="ms-2 text-medium-emphasis">{{ item.transaction.sourceAccountName }}</span>
                                    <v-spacer/>
                                    <span>
                                        {{ getDisplaySourceAmount(item.transaction) }}
                                    </span>
                                </div>
                                <div class="text-body-small text-warning" v-if="item.reason === SnapshotDifferenceRemovedReason.NotMatched">
                                    {{ tt('No longer matches the archived queries') }}
                                </div>
                                <div class="text-body-small text-error" v-else>
                                    {{ tt('Deleted or moved out of archived time range') }}
                                </div>
                            </div>
                        </v-expansion-panel-text>
                    </v-expansion-panel>

                    <v-expansion-panel v-if="difference.modified.length > 0">
                        <v-expansion-panel-title>
                            <v-icon class="me-2 text-warning" :icon="mdiPencilOutline" size="18"/>
                            {{ tt('Modified Transactions') }} ({{ difference.modified.length }})
                        </v-expansion-panel-title>
                        <v-expansion-panel-text>
                            <div v-for="item in difference.modified" :key="'modified-' + item.transaction.id" class="snapshot-diff-row py-2">
                                <div class="d-flex align-center">
                                    <v-chip label variant="outlined" size="x-small"
                                            :class="{ 'text-income' : item.transaction.type === TransactionType.Income, 'text-expense': item.transaction.type === TransactionType.Expense }">
                                        {{ getTransactionTypeName(item.transaction.type) }}
                                    </v-chip>
                                    <v-chip color="info" variant="tonal" size="x-small" class="ms-2"
                                           v-if="item.renamedOnly">{{ tt('Renamed') }}</v-chip>
                                    <span class="ms-2">{{ formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(item.transaction.time)) }}</span>
                                    <span class="ms-2">{{ item.transaction.secondaryCategoryName }}</span>
                                    <span class="ms-2 text-medium-emphasis">{{ item.transaction.sourceAccountName }}</span>
                                </div>
                                <div v-for="change in item.changes" :key="item.transaction.id + '-' + change.field"
                                     class="d-flex align-center text-body-small py-1 snapshot-diff-change">
                                    <strong class="snapshot-diff-field-label">{{ getFieldLabel(change.field) }}</strong>
                                    <span class="text-decoration-line-through text-medium-emphasis ms-2">
                                        {{ displayFieldValue(change.field, change.oldValue, item.frozenTransaction) }}
                                    </span>
                                    <v-icon class="mx-1" :icon="mdiArrowRight" size="13"/>
                                    <span class="text-primary">
                                        {{ displayFieldValue(change.field, change.newValue, item.transaction) }}
                                    </span>
                                </div>
                            </div>
                        </v-expansion-panel-text>
                    </v-expansion-panel>
                </v-expansion-panels>
            </template>
        </one-column-dialog-layout>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { TransactionType } from '@/core/transaction.ts';

import type { TransactionInsightDataItem } from '@/models/transaction.ts';
import {
    SnapshotDifferenceChangeField,
    SnapshotDifferenceRemovedReason,
    type SnapshotDifference
} from '@/lib/explorerSnapshot.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    parseDateTimeFromUnixTime,
    getUtcOffsetByUtcOffsetMinutes
} from '@/lib/datetime.ts';

import {
    mdiPlus,
    mdiMinus,
    mdiPencilOutline,
    mdiArrowRight
} from '@mdi/js';

const { tt, formatDateTimeToLongDateTime, formatAmountToWesternArabicNumeralsWithoutDigitGrouping } = useI18n();

const showState = ref<boolean>(false);
const difference = ref<SnapshotDifference | null>(null);

function getTransactionTypeName(type: number): string {
    if (type === TransactionType.ModifyBalance) {
        return tt('Modify Balance');
    } else if (type === TransactionType.Income) {
        return tt('Income');
    } else if (type === TransactionType.Expense) {
        return tt('Expense');
    } else if (type === TransactionType.Transfer) {
        return tt('Transfer');
    }

    return tt('Unknown');
}

function getDisplaySourceAmount(transaction: TransactionInsightDataItem): string {
    return formatAmountToWesternArabicNumeralsWithoutDigitGrouping(parseBigDecimal(transaction.sourceAmount), transaction.sourceAccount?.currency);
}

function getFieldLabel(field: SnapshotDifferenceChangeField): string {
    switch (field) {
        case SnapshotDifferenceChangeField.Type:
            return tt('Transaction Type');
        case SnapshotDifferenceChangeField.Time:
            return tt('Transaction Time');
        case SnapshotDifferenceChangeField.UtcOffset:
            return tt('Timezone');
        case SnapshotDifferenceChangeField.SourceAmount:
            return tt('Source Amount');
        case SnapshotDifferenceChangeField.DestinationAmount:
            return tt('Destination Amount');
        case SnapshotDifferenceChangeField.PrimaryCategory:
            return tt('Primary Category');
        case SnapshotDifferenceChangeField.SecondaryCategory:
            return tt('Category');
        case SnapshotDifferenceChangeField.SourceAccount:
            return tt('Source Account');
        case SnapshotDifferenceChangeField.DestinationAccount:
            return tt('Destination Account');
        case SnapshotDifferenceChangeField.Tags:
            return tt('Tags');
        case SnapshotDifferenceChangeField.Comment:
            return tt('Description');
        case SnapshotDifferenceChangeField.HideAmount:
            return tt('Hide Amount');
        case SnapshotDifferenceChangeField.GeoLocation:
            return tt('Geo Location');
        default:
            return field;
    }
}

function displayFieldValue(field: SnapshotDifferenceChangeField, value: string | number | boolean, transaction: TransactionInsightDataItem): string {
    switch (field) {
        case SnapshotDifferenceChangeField.Type:
            return getTransactionTypeName(Number(value));
        case SnapshotDifferenceChangeField.Time:
            return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(Number(value)));
        case SnapshotDifferenceChangeField.UtcOffset:
            return getUtcOffsetByUtcOffsetMinutes(Number(value));
        case SnapshotDifferenceChangeField.SourceAmount:
            return formatAmountToWesternArabicNumeralsWithoutDigitGrouping(parseBigDecimal(Number(value)), transaction.sourceAccount?.currency);
        case SnapshotDifferenceChangeField.DestinationAmount:
            return formatAmountToWesternArabicNumeralsWithoutDigitGrouping(parseBigDecimal(Number(value)), transaction.destinationAccount?.currency || '');
        case SnapshotDifferenceChangeField.HideAmount:
            return value ? tt('Yes') : tt('No');
        default:
            return value === '' ? tt('None') : String(value);
    }
}

function open(snapshotDifference: SnapshotDifference): void {
    difference.value = snapshotDifference;
    showState.value = true;
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>
