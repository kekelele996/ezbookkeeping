<template>
    <f7-page ptr @ptr:refresh="generate()">
        <f7-navbar :title="tt('Annual Tax Summary')">
            <template #right>
                <f7-link @click="generate()">{{ tt('Refresh') }}</f7-link>
            </template>
        </f7-navbar>

        <!-- Parameters -->
        <f7-block-title>{{ tt('Parameters') }}</f7-block-title>
        <f7-list strong inset dividers>
            <f7-list-input label="" type="number" input-info="">
                <template #input>
                    <input type="number" v-model.number="year" :placeholder="tt('Year')" />
                </template>
            </f7-list-input>
            <f7-list-item @click="showFiscalYearStartPopup = true">
                <f7-row>
                    <f7-col>{{ tt('Fiscal Year Start') }}</f7-col>
                    <f7-col class="text-align-right">{{ fiscalYearStartDisplayText }}</f7-col>
                </f7-row>
                <list-item-selection-popup value-type="item"
                                           key-field="value" value-field="value" title-field="title"
                                           :title="tt('Fiscal Year Start')" :items="fiscalYearStartOptions"
                                           v-model:show="showFiscalYearStartPopup" v-model="fiscalYearStart">
                </list-item-selection-popup>
            </f7-list-item>
            <f7-list-item @click="showCurrencyPopup = true">
                <f7-row>
                    <f7-col>{{ tt('Base Currency') }}</f7-col>
                    <f7-col class="text-align-right">{{ baseCurrency }}</f7-col>
                </f7-row>
                <list-item-selection-popup value-type="item"
                                           key-field="currencyCode" value-field="currencyCode"
                                           title-field="displayName" :enable-filter="true"
                                           :filter-placeholder="tt('Currency')"
                                           :title="tt('Base Currency')" :items="allCurrencies"
                                           v-model:show="showCurrencyPopup" v-model="baseCurrency">
                </list-item-selection-popup>
            </f7-list-item>
            <f7-list-button :class="{ 'disabled': loading }" @click="generate()">{{ tt('Generate') }}</f7-list-button>
        </f7-list>

        <template v-if="summary">
            <!-- Totals -->
            <f7-block strong inset class="total-block">
                <div class="total-row">
                    <span>{{ tt('Total Deductible Amount') }}</span>
                    <span>{{ formatAmountWithCurrency(summary.totalDeductibleAmount, summary.baseCurrency) }}</span>
                </div>
                <div class="total-row refund">
                    <span>{{ tt('Total Refund Amount') }}</span>
                    <span>− {{ formatAmountWithCurrency(summary.totalRefundAmount, summary.baseCurrency) }}</span>
                </div>
                <div class="total-row net">
                    <span>{{ tt('Net Deductible Amount') }}</span>
                    <span>{{ formatAmountWithCurrency(summary.totalNetDeductibleAmount, summary.baseCurrency) }}</span>
                </div>
                <div class="period-text">
                    {{ formatDate(summary.startTime) }} – {{ formatDate(summary.endTime) }}
                </div>
            </f7-block>

            <!-- Warnings -->
            <f7-block-title v-if="summary.rateWarnings.length">{{ tt('Exchange Rate Warnings') }}</f7-block-title>
            <f7-list v-if="summary.rateWarnings.length" strong inset>
                <f7-list-item v-for="(warning, index) in summary.rateWarnings" :key="index" link="#">
                    <f7-row>
                        <f7-col>{{ warning.currency }} · {{ warning.date }}</f7-col>
                        <f7-col v-if="warning.reason === TaxExchangeRateWarningReason.DailyNotFound"
                                class="text-align-right" @click="openDailyRatePopup(warning.currency, warning.date)">
                            {{ tt('Add Daily Rate') }}
                        </f7-col>
                    </f7-row>
                    <template #subtitle>{{ getWarningReasonText(warning.reason) }}</template>
                </f7-list-item>
            </f7-list>

            <!-- Deductible categories -->
            <f7-block-title>{{ tt('Deductible Categories') }}</f7-block-title>
            <f7-list v-if="summary.categories.length" strong inset accordion-list>
                <f7-accordion-item v-for="category in summary.categories" :key="category.categoryId">
                    <f7-accordion-toggle>
                        <f7-row>
                            <f7-col>
                                {{ category.categoryName }}
                                <span v-if="category.missingReceiptCount" class="missing-text">
                                    · {{ tt('Missing Receipt') }} {{ category.missingReceiptCount }}
                                </span>
                            </f7-col>
                            <f7-col class="text-align-right">
                                {{ formatAmountWithCurrency(category.amount, summary.baseCurrency) }}
                            </f7-col>
                        </f7-row>
                    </f7-accordion-toggle>
                    <f7-accordion-content>
                        <f7-block v-for="transaction in category.transactions" :key="transaction.id" class="transaction-block">
                            <f7-row>
                                <f7-col>{{ formatDate(transaction.time) }} · {{ transaction.accountName }}</f7-col>
                                <f7-col class="text-align-right">
                                    {{ formatPlainAmount(transaction.convertedAmount) }}
                                </f7-col>
                            </f7-row>
                            <f7-row v-if="transaction.comment">
                                <f7-col class="text-small text-gray">{{ transaction.comment }}</f7-col>
                            </f7-row>
                            <f7-row>
                                <f7-col class="text-small">
                                    {{ formatPlainAmount(transaction.originalAmount) }} {{ transaction.originalCurrency }}
                                    · {{ transaction.exchangeRateDate }} · {{ transaction.exchangeRate }}
                                    · {{ transaction.hasReceipt ? tt('Has Receipt') : tt('No Receipt') }}
                                </f7-col>
                            </f7-row>
                        </f7-block>
                    </f7-accordion-content>
                </f7-accordion-item>
            </f7-list>
            <f7-block v-if="!summary.categories.length" strong inset class="text-gray">
                {{ tt('No deductible category, please mark categories in category management') }}
            </f7-block>

            <!-- Refunds -->
            <f7-block-title>{{ tt('Refunds') }}</f7-block-title>
            <f7-list v-if="summary.refunds.length" strong inset>
                <f7-list-item v-for="refund in summary.refunds" :key="refund.categoryId">
                    <f7-row>
                        <f7-col>{{ refund.categoryName }} · {{ refund.transactionCount }}</f7-col>
                        <f7-col class="text-align-right">− {{ formatAmountWithCurrency(refund.amount, summary.baseCurrency) }}</f7-col>
                    </f7-row>
                </f7-list-item>
            </f7-list>
            <f7-block v-if="!summary.refunds.length" strong inset class="text-gray">{{ tt('No refund') }}</f7-block>

            <!-- Excluded -->
            <f7-block-title>{{ tt('Excluded Transactions') }}</f7-block-title>
            <f7-list strong inset accordion-list>
                <f7-accordion-item>
                    <f7-accordion-toggle>
                        <f7-row>
                            <f7-col>{{ tt('Transfers') }} · {{ summary.excludedTransactions.transferTransactions.length }}</f7-col>
                            <f7-col class="text-align-right">{{ formatAmountWithCurrency(summary.excludedTransactions.totalTransferAmount, summary.baseCurrency) }}</f7-col>
                        </f7-row>
                    </f7-accordion-toggle>
                    <f7-accordion-content>
                        <f7-block v-for="item in summary.excludedTransactions.transferTransactions" :key="item.id">
                            <f7-row>
                                <f7-col>{{ formatDate(item.time) }} · {{ item.accountName }} → {{ item.destinationAccountName }}</f7-col>
                                <f7-col class="text-align-right">{{ formatPlainAmount(item.originalAmount) }} {{ item.originalCurrency }}</f7-col>
                            </f7-row>
                        </f7-block>
                    </f7-accordion-content>
                </f7-accordion-item>
                <f7-accordion-item>
                    <f7-accordion-toggle>
                        <f7-row>
                            <f7-col>{{ tt('Account Repayments') }} · {{ summary.excludedTransactions.accountRepaymentTransactions.length }}</f7-col>
                            <f7-col class="text-align-right">{{ formatAmountWithCurrency(summary.excludedTransactions.totalAccountRepaymentAmount, summary.baseCurrency) }}</f7-col>
                        </f7-row>
                    </f7-accordion-toggle>
                    <f7-accordion-content>
                        <f7-block v-for="item in summary.excludedTransactions.accountRepaymentTransactions" :key="item.id">
                            <f7-row>
                                <f7-col>{{ formatDate(item.time) }} · {{ item.accountName }} → {{ item.destinationAccountName }}</f7-col>
                                <f7-col class="text-align-right">{{ formatPlainAmount(item.originalAmount) }} {{ item.originalCurrency }}</f7-col>
                            </f7-row>
                        </f7-block>
                    </f7-accordion-content>
                </f7-accordion-item>
            </f7-list>

            <!-- Changes -->
            <template v-if="summary.changes">
                <f7-block-title>{{ tt('Changes Compared To Snapshot') }} #{{ summary.comparedSnapshotId }}</f7-block-title>
                <f7-list strong inset>
                    <f7-list-item v-for="change in summary.changes.categoryChanges"
                                  :key="change.type + '_' + change.categoryId">
                        <f7-row>
                            <f7-col>{{ change.categoryName }}</f7-col>
                            <f7-col class="text-align-right" :class="getDeltaClass(change.delta)">
                                {{ formatSignedPlainAmount(change.delta) }}
                            </f7-col>
                        </f7-row>
                        <template #subtitle>
                            {{ formatPlainAmount(change.beforeAmount) }} → {{ formatPlainAmount(change.afterAmount) }}
                        </template>
                    </f7-list-item>
                </f7-list>
            </template>

            <!-- Snapshots -->
            <f7-block-title>{{ tt('Snapshots') }}</f7-block-title>
            <f7-list strong inset>
                <f7-list-button @click="saveSnapshot()">{{ tt('Save Snapshot') }}</f7-list-button>
                <f7-list-item v-for="snapshotItem in snapshots" :key="snapshotItem.snapshotId">
                    <f7-row>
                        <f7-col>
                            #{{ snapshotItem.snapshotId }} · {{ snapshotItem.year }} · {{ snapshotItem.baseCurrency }}
                        </f7-col>
                        <f7-col class="text-align-right">
                            <f7-link class="margin-right" @click="compareWithSnapshot(snapshotItem)">{{ tt('Compare') }}</f7-link>
                            <f7-link @click="deleteSnapshot(snapshotItem)">{{ tt('Delete') }}</f7-link>
                        </f7-col>
                    </f7-row>
                </f7-list-item>
            </f7-list>
        </template>

        <!-- Daily rate popup -->
        <f7-popup v-model:opened="dailyRatePopup.show" class="daily-rate-popup">
            <f7-view>
                <f7-page>
                    <f7-navbar :title="tt('Add Daily Rate')">
                        <template #right>
                            <f7-link @click="dailyRatePopup.show = false">{{ tt('Close') }}</f7-link>
                        </template>
                    </f7-navbar>
                    <f7-list strong inset dividers>
                        <f7-list-input :label="tt('Currency')">
                            <template #input><input v-model="dailyRatePopup.currency" readonly /></template>
                        </f7-list-input>
                        <f7-list-input :label="tt('Rate Date')">
                            <template #input><input v-model="dailyRatePopup.date" readonly /></template>
                        </f7-list-input>
                        <f7-list-input :label="tt('Exchange Rate')">
                            <template #input><input v-model="dailyRatePopup.rate" /></template>
                        </f7-list-input>
                    </f7-list>
                    <f7-block strong inset class="text-small text-gray">
                        {{ tt('Exchange rate is amount of base currency per 1 unit') }}
                    </f7-block>
                    <f7-block>
                        <f7-button fill raised @click="submitDailyRate()">{{ tt('Save') }}</f7-button>
                    </f7-block>
                </f7-page>
            </f7-view>
        </f7-popup>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';

import services from '@/lib/services.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import {
    TaxExchangeRateWarningReason,
    type TaxSummary,
    type TaxReportSnapshotInfo
} from '@/models/tax_report.ts';

const now = new Date();

const year = ref<number>(now.getFullYear());
const fiscalYearStart = ref<string>('');
const baseCurrency = ref<string>('');

const loading = ref<boolean>(false);
const summary = ref<TaxSummary | null>(null);
const snapshots = ref<TaxReportSnapshotInfo[]>([]);

const showFiscalYearStartPopup = ref<boolean>(false);
const showCurrencyPopup = ref<boolean>(false);

const dailyRatePopup = ref({
    show: false,
    currency: '',
    date: '',
    rate: ''
});

const {
    tt,
    getDefaultCurrency,
    getAllCurrencies,
    formatDateTimeToLongDate,
    formatAmountToLocalizedNumerals,
    formatAmountToLocalizedNumeralsWithCurrency
} = useI18n();

const { showToast } = useI18nUIComponents();

baseCurrency.value = getDefaultCurrency();

const fiscalYearStartOptions = [
    { title: tt('Default (January 1)'), value: '' },
    { title: '01-01', value: '01-01' },
    { title: '04-01', value: '04-01' },
    { title: '04-06', value: '04-06' },
    { title: '07-01', value: '07-01' },
    { title: '10-01', value: '10-01' }
];

const allCurrencies = computed(() => getAllCurrencies(false));

const fiscalYearStartDisplayText = computed<string>(() => {
    return fiscalYearStart.value || tt('Default (January 1)');
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
            showToast(error.message || error);
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
        showToast(tt('Snapshot saved'));
        loadSnapshots();
    }).catch(error => {
        showToast(error.message || error);
    });
}

function deleteSnapshot(snapshot: TaxReportSnapshotInfo): void {
    services.deleteTaxReportSnapshot({ snapshotId: snapshot.snapshotId }).then(() => {
        showToast(tt('Snapshot deleted'));
        loadSnapshots();

        if (summary.value?.comparedSnapshotId === snapshot.snapshotId) {
            generate();
        }
    }).catch(error => {
        showToast(error.message || error);
    });
}

function compareWithSnapshot(snapshot: TaxReportSnapshotInfo): void {
    year.value = snapshot.year;
    fiscalYearStart.value = snapshot.fiscalYearStart === '01-01' ? '' : snapshot.fiscalYearStart;
    baseCurrency.value = snapshot.baseCurrency;
    generate(snapshot.snapshotId);
}

function openDailyRatePopup(currency: string, date: string): void {
    dailyRatePopup.value = {
        show: true,
        currency: currency,
        date: date,
        rate: ''
    };
}

function submitDailyRate(): void {
    services.updateTaxDailyExchangeRate({
        currency: dailyRatePopup.value.currency,
        date: dailyRatePopup.value.date,
        rate: dailyRatePopup.value.rate
    }).then(() => {
        dailyRatePopup.value.show = false;
        showToast(tt('Daily rate saved'));

        if (summary.value?.comparedSnapshotId) {
            generate(summary.value.comparedSnapshotId);
        } else {
            generate();
        }
    }).catch(error => {
        showToast(error.message || error);
    });
}

loadSnapshots();
</script>

<style scoped>
.total-block .total-row {
    display: flex;
    justify-content: space-between;
    padding-block: 0.25rem;
}

.total-block .total-row.net {
    font-weight: 600;
    border-block-start: 1px solid var(--f7-list-item-border-color, rgba(0, 0, 0, 0.12));
    margin-block-start: 0.25rem;
    padding-block-start: 0.5rem;
}

.total-block .period-text {
    margin-block-start: 0.5rem;
    font-size: 0.8rem;
    opacity: 0.6;
}

.missing-text {
    color: var(--f7-theme-color-orange, orange);
}

.text-gray {
    opacity: 0.6;
}

.text-small {
    font-size: 0.8rem;
}

.transaction-block {
    margin-block: 0.25rem;
}

.delta-positive {
    color: var(--f7-color-green);
}

.delta-negative {
    color: var(--f7-color-red);
}

.daily-rate-popup {
    overflow-y: auto;
}
</style>
