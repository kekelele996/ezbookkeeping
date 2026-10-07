<template>
    <main-page-layout>
        <template #content>
            <div class="tax-report-page px-3 py-3">
                <!-- Parameter bar -->
                <v-card class="parameter-card pa-3">
                    <div class="d-flex flex-wrap align-center gap-2">
                        <v-text-field v-model.number="year" class="year-input" density="compact" variant="outlined"
                                      type="number" hide-details :label="tt('Year')" :min="1900" :max="2999" />
                        <v-select v-model="fiscalYearStart" class="fiscal-year-start-select" density="compact"
                                  variant="outlined" hide-details :label="tt('Fiscal Year Start')"
                                  :items="fiscalYearStartOptions" item-title="title" item-value="value" />
                        <currency-select v-model="baseCurrency" density="compact" variant="outlined"
                                         :label="tt('Base Currency')" />
                        <v-spacer />
                        <v-btn color="primary" :loading="loading" :prepend-icon="mdiCalculatorVariant"
                               @click="generate()">
                            {{ tt('Generate') }}
                        </v-btn>
                        <v-btn color="default" variant="tonal" :prepend-icon="mdiContentSave"
                               :disabled="!summary" @click="saveSnapshot()">
                            {{ tt('Save Snapshot') }}
                        </v-btn>
                    </div>
                </v-card>

                <template v-if="summary">
                    <!-- Period & method -->
                    <div class="text-body-2 text-medium-emphasis mt-2">
                        {{ tt('Tax Period') }}: {{ formatDate(summary.startTime) }} – {{ formatDate(summary.endTime) }}
                        <v-tooltip activator="parent" location="bottom">{{ tt('Tax Period Method Tooltip') }}</v-tooltip>
                    </div>

                    <!-- Total cards -->
                    <v-row class="mt-1">
                        <v-col cols="12" md="4">
                            <v-card class="total-card" :prepend-icon="mdiReceiptText">
                                <v-card-item>
                                    <v-card-title>{{ tt('Total Deductible Amount') }}</v-card-title>
                                    <v-card-subtitle>{{ formatAmountWithCurrency(summary.totalDeductibleAmount, summary.baseCurrency) }}</v-card-subtitle>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card class="total-card refund" :prepend-icon="mdiCashRefund">
                                <v-card-item>
                                    <v-card-title>{{ tt('Total Refund Amount') }}</v-card-title>
                                    <v-card-subtitle>− {{ formatAmountWithCurrency(summary.totalRefundAmount, summary.baseCurrency) }}</v-card-subtitle>
                                </v-card-item>
                            </v-card>
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-card class="total-card net" :prepend-icon="mdiCalculatorVariant">
                                <v-card-item>
                                    <v-card-title>{{ tt('Net Deductible Amount') }}</v-card-title>
                                    <v-card-subtitle>{{ formatAmountWithCurrency(summary.totalNetDeductibleAmount, summary.baseCurrency) }}</v-card-subtitle>
                                </v-card-item>
                            </v-card>
                        </v-col>
                    </v-row>

                    <!-- Exchange rate warnings -->
                    <v-card v-if="summary.rateWarnings.length" class="warning-card mt-3 pa-3" variant="tonal" color="warning">
                        <div class="d-flex align-center mb-1">
                            <v-icon :icon="mdiAlert" class="me-2" />
                            <span class="text-subtitle-1">{{ tt('Exchange Rate Warnings') }}</span>
                        </div>
                        <v-list density="compact" lines="two">
                            <v-list-item v-for="(warning, index) in summary.rateWarnings" :key="index">
                                <v-list-item-title>
                                    {{ warning.currency }} · {{ warning.date }} · {{ getWarningReasonText(warning.reason) }}
                                </v-list-item-title>
                                <template v-if="warning.reason === TaxExchangeRateWarningReason.DailyNotFound">
                                    <v-btn class="ms-2" size="small" variant="outlined" :prepend-icon="mdiPlus"
                                           @click="openDailyRateDialog(warning.currency, warning.date)">
                                        {{ tt('Add Daily Rate') }}
                                    </v-btn>
                                </template>
                            </v-list-item>
                        </v-list>
                    </v-card>

                    <!-- Deductible categories -->
                    <v-card class="mt-3">
                        <v-card-item>
                            <v-card-title>{{ tt('Deductible Categories') }}</v-card-title>
                            <v-card-subtitle>{{ tt('Deductible Categories Subtitle') }}</v-card-subtitle>
                        </v-card-item>
                        <v-divider />
                        <v-list v-if="summary.categories.length" density="compact" :lines="false">
                            <v-list-item v-for="category in summary.categories" :key="category.categoryId">
                                <template #prepend>
                                    <v-btn :icon="expandedCategories[category.categoryId] ? mdiChevronUp : mdiChevronDown"
                                           variant="text" size="small"
                                           @click="toggleCategory(category.categoryId)" />
                                </template>

                                <v-list-item-title class="d-flex align-center">
                                    <span>{{ category.categoryName }}</span>
                                    <v-chip v-if="category.missingReceiptCount" class="ms-2" size="small"
                                            color="warning" variant="tonal">
                                        {{ tt('Missing Receipt') }} {{ category.missingReceiptCount }}
                                    </v-chip>
                                </v-list-item-title>
                                <v-list-item-subtitle>{{ category.transactionCount }} {{ tt('Transaction') }}</v-list-item-subtitle>
                                <template #append>
                                    <span class="text-subtitle-2">{{ formatAmountWithCurrency(category.amount, summary.baseCurrency) }}</span>
                                </template>
                            </v-list-item>
                            <v-expand-transition>
                                <div v-for="category in expandedCategoryItems" :key="'detail_' + category.categoryId">
                                    <v-simple-table class="source-transaction-table">
                                        <thead>
                                            <tr>
                                                <th>{{ tt('Date') }}</th>
                                                <th>{{ tt('Account') }}</th>
                                                <th>{{ tt('Comment') }}</th>
                                                <th class="text-end">{{ tt('Original Amount') }}</th>
                                                <th>{{ tt('Rate Date') }}</th>
                                                <th>{{ tt('Exchange Rate') }}</th>
                                                <th class="text-end">{{ tt('Converted Amount') }}</th>
                                                <th>{{ tt('Receipt') }}</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr v-for="transaction in category.transactions" :key="transaction.id">
                                                <td>{{ formatDate(transaction.time) }}</td>
                                                <td>{{ transaction.accountName }}</td>
                                                <td>{{ transaction.comment }}</td>
                                                <td class="text-end">{{ formatPlainAmount(transaction.originalAmount) }} {{ transaction.originalCurrency }}</td>
                                                <td>{{ transaction.exchangeRateDate }}</td>
                                                <td>
                                                    <span :class="{ 'rate-fallback': transaction.exchangeRateSource > TaxExchangeRateSource.DailyExact }">
                                                        {{ transaction.exchangeRate }}
                                                    </span>
                                                </td>
                                                <td class="text-end">{{ formatPlainAmount(transaction.convertedAmount) }}</td>
                                                <td>
                                                    <v-icon size="18" :icon="transaction.hasReceipt ? mdiImageMultiple : mdiImageOff"
                                                            :color="transaction.hasReceipt ? 'success' : 'warning'" />
                                                </td>
                                            </tr>
                                        </tbody>
                                    </v-simple-table>
                                </div>
                            </v-expand-transition>
                        </v-list>
                        <v-card-text v-if="!summary.categories.length" class="text-medium-emphasis">
                            {{ tt('No deductible category, please mark categories in category management') }}
                        </v-card-text>
                    </v-card>

                    <!-- Refunds -->
                    <v-card class="mt-3">
                        <v-card-item>
                            <v-card-title>{{ tt('Refunds') }}</v-card-title>
                            <v-card-subtitle>{{ tt('Refunds Subtitle') }}</v-card-subtitle>
                        </v-card-item>
                        <v-divider />
                        <v-list v-if="summary.refunds.length" density="compact" :lines="false">
                            <v-list-item v-for="refund in summary.refunds" :key="refund.categoryId">
                                <v-list-item-title>{{ refund.categoryName }}</v-list-item-title>
                                <v-list-item-subtitle>{{ refund.transactionCount }} {{ tt('Transaction') }}</v-list-item-subtitle>
                                <template #append>
                                    <span class="text-subtitle-2">− {{ formatAmountWithCurrency(refund.amount, summary.baseCurrency) }}</span>
                                </template>
                            </v-list-item>
                        </v-list>
                        <v-card-text v-if="!summary.refunds.length" class="text-medium-emphasis">
                            {{ tt('No refund') }}
                        </v-card-text>
                    </v-card>

                    <!-- Excluded transactions -->
                    <v-card class="mt-3">
                        <v-card-item>
                            <v-card-title>{{ tt('Excluded Transactions') }}</v-card-title>
                            <v-card-subtitle>{{ tt('Excluded Transactions Subtitle') }}</v-card-subtitle>
                        </v-card-item>
                        <v-divider />
                        <v-card-text>
                            <div class="d-flex align-center mb-2">
                                <v-icon :icon="mdiBankTransfer" class="me-2" />
                                <span class="text-subtitle-2 me-2">{{ tt('Transfers') }}</span>
                                <span class="text-body-2 text-medium-emphasis">{{ summary.excludedTransactions.transferTransactions.length }} · {{ formatAmountWithCurrency(summary.excludedTransactions.totalTransferAmount, summary.baseCurrency) }}</span>
                            </div>
                            <v-simple-table v-if="summary.excludedTransactions.transferTransactions.length" class="excluded-table mb-3">
                                <thead>
                                    <tr>
                                        <th>{{ tt('Date') }}</th>
                                        <th>{{ tt('From Account') }}</th>
                                        <th>{{ tt('To Account') }}</th>
                                        <th class="text-end">{{ tt('Amount') }}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="item in summary.excludedTransactions.transferTransactions" :key="item.id">
                                        <td>{{ formatDate(item.time) }}</td>
                                        <td>{{ item.accountName }}</td>
                                        <td>{{ item.destinationAccountName }}</td>
                                        <td class="text-end">{{ formatPlainAmount(item.originalAmount) }} {{ item.originalCurrency }}</td>
                                    </tr>
                                </tbody>
                            </v-simple-table>

                            <div class="d-flex align-center mb-2">
                                <v-icon :icon="mdiCreditCardOutline" class="me-2" />
                                <span class="text-subtitle-2 me-2">{{ tt('Account Repayments') }}</span>
                                <span class="text-body-2 text-medium-emphasis">{{ summary.excludedTransactions.accountRepaymentTransactions.length }} · {{ formatAmountWithCurrency(summary.excludedTransactions.totalAccountRepaymentAmount, summary.baseCurrency) }}</span>
                            </div>
                            <v-simple-table v-if="summary.excludedTransactions.accountRepaymentTransactions.length" class="excluded-table">
                                <thead>
                                    <tr>
                                        <th>{{ tt('Date') }}</th>
                                        <th>{{ tt('From Account') }}</th>
                                        <th>{{ tt('To Account') }}</th>
                                        <th class="text-end">{{ tt('Amount') }}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="item in summary.excludedTransactions.accountRepaymentTransactions" :key="item.id">
                                        <td>{{ formatDate(item.time) }}</td>
                                        <td>{{ item.accountName }}</td>
                                        <td>{{ item.destinationAccountName }}</td>
                                        <td class="text-end">{{ formatPlainAmount(item.originalAmount) }} {{ item.originalCurrency }}</td>
                                    </tr>
                                </tbody>
                            </v-simple-table>
                        </v-card-text>
                    </v-card>

                    <!-- Changes vs snapshot -->
                    <v-card v-if="summary.changes" class="mt-3" color="info" variant="tonal">
                        <v-card-item>
                            <v-card-title>
                                <v-icon :icon="mdiSwapHorizontal" class="me-2" />
                                {{ tt('Changes Compared To Snapshot') }} #{{ summary.comparedSnapshotId }}
                            </v-card-title>
                            <v-card-subtitle>{{ formatDate(summary.changes.snapshotCreatedUnixTime) }}</v-card-subtitle>
                        </v-card-item>
                        <v-simple-table class="changes-table">
                            <thead>
                                <tr>
                                    <th>{{ tt('Category') }}</th>
                                    <th class="text-end">{{ tt('Before') }}</th>
                                    <th class="text-end">{{ tt('After') }}</th>
                                    <th class="text-end">{{ tt('Difference') }}</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="change in summary.changes.categoryChanges" :key="change.type + '_' + change.categoryId">
                                    <td>
                                        <v-chip size="x-small" class="me-1" variant="outlined">
                                            {{ change.type === TaxReportCategoryChangeType.Deductible ? tt('Deductible') : tt('Refund') }}
                                        </v-chip>
                                        {{ change.categoryName }}
                                    </td>
                                    <td class="text-end">{{ formatPlainAmount(change.beforeAmount) }}</td>
                                    <td class="text-end">{{ formatPlainAmount(change.afterAmount) }}</td>
                                    <td class="text-end" :class="getDeltaClass(change.delta)">
                                        {{ formatSignedPlainAmount(change.delta) }}
                                    </td>
                                </tr>
                            </tbody>
                        </v-simple-table>
                        <v-card-text class="pt-0">
                            <span class="me-3">{{ tt('Net Deductible Amount') }} {{ formatSignedPlainAmount(summary.changes.totalNetDeductibleAmountDelta) }}</span>
                        </v-card-text>
                    </v-card>

                    <!-- Snapshots -->
                    <v-card class="mt-3 mb-4">
                        <v-card-item>
                            <v-card-title>{{ tt('Snapshots') }}</v-card-title>
                        </v-card-item>
                        <v-divider />
                        <v-list v-if="snapshots.length" density="compact">
                            <v-list-item v-for="snapshot in snapshots" :key="snapshot.snapshotId">
                                <v-list-item-title>
                                    #{{ snapshot.snapshotId }} · {{ snapshot.year }} · {{ snapshot.fiscalYearStart }} · {{ snapshot.baseCurrency }}
                                </v-list-item-title>
                                <v-list-item-subtitle>{{ formatDate(snapshot.createdUnixTime) }}</v-list-item-subtitle>
                                <template #append>
                                    <v-btn size="small" variant="text" :prepend-icon="mdiSwapHorizontal" @click="compareWithSnapshot(snapshot)">
                                        {{ tt('Compare') }}
                                    </v-btn>
                                    <v-btn size="small" variant="text" :prepend-icon="mdiDelete" @click="deleteSnapshot(snapshot)">
                                        {{ tt('Delete') }}
                                    </v-btn>
                                </template>
                            </v-list-item>
                        </v-list>
                        <v-card-text v-if="!snapshots.length" class="text-medium-emphasis">
                            {{ tt('No snapshot') }}
                        </v-card-text>
                    </v-card>
                </template>
            </div>
        </template>
    </main-page-layout>

    <!-- Daily rate dialog -->
    <v-dialog v-model="dailyRateDialog.show" max-width="420">
        <v-card>
            <v-card-item>
                <v-card-title>{{ tt('Add Daily Rate') }}</v-card-title>
            </v-card-item>
            <v-card-text>
                <v-text-field v-model="dailyRateDialog.currency" :label="tt('Currency')" readonly density="compact" class="mb-2" />
                <v-text-field v-model="dailyRateDialog.date" :label="tt('Rate Date')" readonly density="compact" class="mb-2" />
                <v-text-field v-model="dailyRateDialog.rate" :label="tt('Exchange Rate')" density="compact"
                              :hint="tt('Exchange rate is amount of base currency per 1 unit')" persistent-hint />
            </v-card-text>
            <v-card-actions>
                <v-spacer />
                <v-btn @click="dailyRateDialog.show = false">{{ tt('Cancel') }}</v-btn>
                <v-btn color="primary" @click="submitDailyRate()">{{ tt('Save') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import MainPageLayout from '@/components/desktop/MainPageLayout.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import CurrencySelect from '@/components/desktop/CurrencySelect.vue';

import { ref, reactive, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import services from '@/lib/services.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import {
    TaxExchangeRateSource,
    TaxExchangeRateWarningReason,
    TaxReportCategoryChangeType,
    type TaxSummary,
    type TaxCategorySummary,
    type TaxReportSnapshotInfo
} from '@/models/tax_report.ts';

import {
    mdiCalculatorVariant,
    mdiContentSave,
    mdiReceiptText,
    mdiCashRefund,
    mdiAlert,
    mdiBankTransfer,
    mdiCreditCardOutline,
    mdiSwapHorizontal,
    mdiDelete,
    mdiChevronDown,
    mdiChevronUp,
    mdiPlus,
    mdiImageMultiple,
    mdiImageOff
} from '@mdi/js';

const {
    tt,
    getDefaultCurrency,
    formatDateTimeToLongDate,
    formatAmountToLocalizedNumerals,
    formatAmountToLocalizedNumeralsWithCurrency
} = useI18n();

const now = new Date();

const year = ref<number>(now.getFullYear());
const fiscalYearStart = ref<string>('');
const baseCurrency = ref<string>(getDefaultCurrency());

const loading = ref<boolean>(false);
const summary = ref<TaxSummary | null>(null);
const snapshots = ref<TaxReportSnapshotInfo[]>([]);
const expandedCategories = reactive<Record<string, boolean>>({});

const snackbar = useTemplateRef<InstanceType<typeof SnackBar>>('snackbar');

const dailyRateDialog = reactive({
    show: false,
    currency: '',
    date: '',
    rate: ''
});

const fiscalYearStartOptions = [
    { title: tt('Default (January 1)'), value: '' },
    { title: '01-01', value: '01-01' },
    { title: '04-01', value: '04-01' },
    { title: '04-06', value: '04-06' },
    { title: '07-01', value: '07-01' },
    { title: '10-01', value: '10-01' }
];

const expandedCategoryItems = computed<TaxCategorySummary[]>(() => {
    if (!summary.value) {
        return [];
    }

    return summary.value.categories.filter(category => !!expandedCategories[category.categoryId]);
});

function formatDate(unixTime: number): string {
    return formatDateTimeToLongDate(parseDateTimeFromUnixTime(unixTime));
}

function formatPlainAmount(amount: string): string {
    return formatAmountToLocalizedNumerals(parseBigDecimal(amount));
}

function formatSignedPlainAmount(amount: string): string {
    const value = parseBigDecimal(amount);
    const text = formatAmountToLocalizedNumerals(value);

    if (value.greaterThan(0)) {
        return '+' + text;
    }

    return text;
}

function formatAmountWithCurrency(amount: string, currency: string): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
}

function getWarningReasonText(reason: TaxExchangeRateWarningReason): string {
    if (reason === TaxExchangeRateWarningReason.AllNotFound) {
        return tt('No available exchange rate');
    }

    return tt('No exact daily rate, fallback rate used');
}

function getDeltaClass(delta: string): string {
    const value = parseBigDecimal(delta);

    if (value.greaterThan(0)) {
        return 'delta-positive';
    } else if (value.lessThan(0)) {
        return 'delta-negative';
    }

    return '';
}

function toggleCategory(categoryId: string): void {
    expandedCategories[categoryId] = !expandedCategories[categoryId];
}

function generate(snapshotId?: string): void {
    loading.value = true;

    services.getTaxSummary({
        year: year.value,
        fiscalYearStart: fiscalYearStart.value,
        baseCurrency: baseCurrency.value,
        snapshotId: snapshotId || ''
    }).then(response => {
        summary.value = response.data.result;
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    }).finally(() => {
        loading.value = false;
    });
}

function loadSnapshots(): void {
    services.getTaxReportSnapshots().then(response => {
        snapshots.value = response.data.result.items;
    }).catch(() => {
        // ignore
    });
}

function saveSnapshot(): void {
    services.createTaxReportSnapshot({
        year: year.value,
        fiscalYearStart: fiscalYearStart.value,
        baseCurrency: baseCurrency.value
    }).then(() => {
        snackbar.value?.showMessage(tt('Snapshot saved'));
        loadSnapshots();
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

function deleteSnapshot(snapshot: TaxReportSnapshotInfo): void {
    services.deleteTaxReportSnapshot({ snapshotId: snapshot.snapshotId }).then(() => {
        snackbar.value?.showMessage(tt('Snapshot deleted'));
        loadSnapshots();

        if (summary.value?.comparedSnapshotId === snapshot.snapshotId) {
            generate();
        }
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

function compareWithSnapshot(snapshot: TaxReportSnapshotInfo): void {
    year.value = snapshot.year;
    fiscalYearStart.value = snapshot.fiscalYearStart === '01-01' ? '' : snapshot.fiscalYearStart;
    baseCurrency.value = snapshot.baseCurrency;
    generate(snapshot.snapshotId);
}

function openDailyRateDialog(currency: string, date: string): void {
    dailyRateDialog.currency = currency;
    dailyRateDialog.date = date;
    dailyRateDialog.rate = '';
    dailyRateDialog.show = true;
}

function submitDailyRate(): void {
    services.updateTaxDailyExchangeRate({
        currency: dailyRateDialog.currency,
        date: dailyRateDialog.date,
        rate: dailyRateDialog.rate
    }).then(() => {
        dailyRateDialog.show = false;
        snackbar.value?.showMessage(tt('Daily rate saved'));

        if (summary.value?.comparedSnapshotId) {
            generate(summary.value.comparedSnapshotId);
        } else {
            generate();
        }
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

loadSnapshots();
</script>

<style scoped>
.tax-report-page .parameter-card {
    inline-size: 100%;
}

.tax-report-page .year-input {
    inline-size: 7rem;
}

.tax-report-page .fiscal-year-start-select {
    inline-size: 11rem;
}

.tax-report-page .total-card .v-card-subtitle {
    font-size: 1.35rem;
    color: rgb(var(--v-theme-on-surface));
}

.tax-report-page .rate-fallback {
    color: rgb(var(--v-theme-warning));
}

.tax-report-page :deep(.source-transaction-table),
.tax-report-page :deep(.excluded-table),
.tax-report-page :deep(.changes-table) {
    padding-inline: 1rem;
}

.tax-report-page .delta-positive {
    color: rgb(var(--v-theme-success));
}

.tax-report-page .delta-negative {
    color: rgb(var(--v-theme-error));
}
</style>
