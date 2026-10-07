<template>
    <v-dialog width="720" v-model="showState">
        <one-column-dialog-layout content-class="pa-0" :disabled="loading"
                                  :title="tt('Archived Snapshots')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #content>
                <v-table hover density="comfortable" class="explorer-snapshots-table w-100 table-striped">
                    <tbody v-if="loading && noAvailableSnapshot">
                    <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3, 4, 5 ]">
                        <td class="px-0">
                            <v-skeleton-loader type="text" :loading="true"></v-skeleton-loader>
                        </td>
                    </tr>
                    </tbody>

                    <tbody v-if="!loading && noAvailableSnapshot">
                    <tr>
                        <td>{{ tt('No archived snapshots') }}</td>
                    </tr>
                    </tbody>

                    <tbody v-if="!loading && !noAvailableSnapshot">
                    <tr v-for="snapshot in allSnapshots" :key="snapshot.id">
                        <td>
                            <div class="d-flex align-center">
                                <div class="d-flex flex-column snapshot-name-and-time">
                                    <span class="text-subtitle-2">{{ snapshot.name }}</span>
                                    <span class="text-body-small text-medium-emphasis">
                                        {{ tt('Frozen at') }} {{ formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(snapshot.createdUnixTime)) }}
                                    </span>
                                </div>

                                <v-spacer/>

                                <v-btn class="px-2" color="default" density="compact" variant="text"
                                       :prepend-icon="mdiEyeOutline" :disabled="loading"
                                       @click="viewSnapshot(snapshot)">
                                    {{ tt('View') }}
                                </v-btn>
                                <v-btn class="px-2" color="default" density="compact" variant="text"
                                       :prepend-icon="mdiCompareHorizontal" :disabled="loading"
                                       @click="compareSnapshot(snapshot)">
                                    {{ tt('Compare') }}
                                </v-btn>
                                <v-btn class="px-2" color="error" density="compact" variant="text"
                                       :prepend-icon="mdiDeleteOutline" :disabled="loading"
                                       :loading="snapshotDeleting[snapshot.id] === true"
                                       @click="removeSnapshot(snapshot)">
                                    {{ tt('Delete') }}
                                </v-btn>
                            </div>
                        </td>
                    </tr>
                    </tbody>
                </v-table>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useExplorersStore } from '@/stores/explorer.ts';

import type { InsightsExplorerSnapshot } from '@/models/explorer_snapshot.ts';

import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import {
    mdiEyeOutline,
    mdiDeleteOutline,
    mdiCompareHorizontal
} from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const emit = defineEmits<{
    (e: 'view', snapshotId: string): void;
    (e: 'compare', snapshotId: string): void;
}>();

const { tt, formatDateTimeToLongDateTime } = useI18n();

const explorersStore = useExplorersStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const loading = ref<boolean>(true);
const snapshotDeleting = ref<Record<string, boolean>>({});

const allSnapshots = computed<InsightsExplorerSnapshot[]>(() => explorersStore.allSnapshotBasicInfos);

const noAvailableSnapshot = computed<boolean>(() => {
    return !allSnapshots.value || allSnapshots.value.length < 1;
});

function open(explorerId?: string): void {
    showState.value = true;
    loading.value = true;

    explorersStore.loadAllSnapshots(explorerId).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function viewSnapshot(snapshot: InsightsExplorerSnapshot): void {
    showState.value = false;
    emit('view', snapshot.id);
}

function compareSnapshot(snapshot: InsightsExplorerSnapshot): void {
    showState.value = false;
    emit('compare', snapshot.id);
}

function removeSnapshot(snapshot: InsightsExplorerSnapshot): void {
    confirmDialog.value?.open(tt('Are you sure you want to delete this archived snapshot?')).then(() => {
        snapshotDeleting.value[snapshot.id] = true;

        explorersStore.deleteSnapshot({
            snapshotId: snapshot.id
        }).then(() => {
            snapshotDeleting.value[snapshot.id] = false;
        }).catch(error => {
            snapshotDeleting.value[snapshot.id] = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>
