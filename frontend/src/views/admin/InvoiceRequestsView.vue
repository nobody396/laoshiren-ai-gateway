<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div class="card p-6">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.invoiceRequests.title') }}</h1>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.invoiceRequests.description') }}</p>
            </div>
            <button class="btn btn-primary" :disabled="exporting" @click="handleExport">
              {{ exporting ? t('admin.invoiceRequests.exporting') : t('admin.invoiceRequests.export') }}
            </button>
          </div>
        </div>

        <div class="card p-6">
          <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('admin.invoiceRequests.pendingCount') }}</p>
          <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ requests.pending_count || 0 }}</p>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model="filters.search" class="input min-w-[220px]" :placeholder="t('admin.invoiceRequests.searchPlaceholder')" @input="handleSearch" />
            <Select v-model="filters.status" :options="statusOptions" class="w-40" @change="reload" />
            <input v-model="filters.start_time" type="date" class="input w-44" @change="reload" />
            <input v-model="filters.end_time" type="date" class="input w-44" @change="reload" />
            <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
              <input v-model="includeExported" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.invoiceRequests.includeProcessed') }}
            </label>
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset') }}</button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="requests.items" :loading="loading" row-key="id">
            <template #cell-user_email="{ row }">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ row.user_email || '-' }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ row.user_name || `ID ${row.user_id}` }}</p>
              </div>
            </template>
            <template #cell-profile_snapshot="{ row }">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ row.profile_snapshot.title }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ row.profile_snapshot.email }}</p>
              </div>
            </template>
            <template #cell-total_amount_fen="{ value }">
              <span class="font-medium text-gray-900 dark:text-white">¥{{ formatFen(value) }}</span>
            </template>
            <template #cell-status="{ value }">
              <span class="badge" :class="requestStatusBadgeClass(value)">{{ requestStatusLabel(value) }}</span>
            </template>
            <template #cell-created_at="{ value }">
              {{ formatDateTime(value) }}
            </template>
            <template #cell-actions="{ row }">
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary" @click="toggleExpanded(row.id)">
                  {{ expandedIds.includes(row.id) ? t('admin.invoiceRequests.collapseDetails') : t('admin.invoiceRequests.viewDetails') }}
                </button>
                <button
                  v-if="row.status === 'pending' || row.status === 'exported'"
                  class="btn btn-secondary"
                  @click="completeRequest(row.id)"
                >
                  {{ t('admin.invoiceRequests.complete') }}
                </button>
                <button
                  v-if="row.status === 'pending' || row.status === 'exported'"
                  class="btn btn-secondary text-red-600"
                  @click="openRejectDialog(row.id)"
                >
                  {{ t('admin.invoiceRequests.reject') }}
                </button>
              </div>

              <div v-if="expandedIds.includes(row.id)" class="mt-3 rounded-xl bg-gray-50 p-4 text-left dark:bg-dark-800">
                <p class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('admin.invoiceRequests.orderDetails') }}</p>
                <div class="mt-2 flex flex-wrap gap-2">
                  <span v-for="order in row.orders" :key="order.id" class="rounded-full bg-white px-3 py-1 text-sm text-gray-700 shadow-sm dark:bg-dark-700 dark:text-dark-100">
                    {{ order.topup_order.order_no }} · ¥{{ formatFen(order.topup_order.amount_cny_fen) }}
                  </span>
                </div>
                <p v-if="row.reject_reason" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ t('admin.invoiceRequests.rejectReason') }}：{{ row.reject_reason }}</p>
                <p v-if="row.remark" class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ t('common.remark') }}：{{ row.remark }}</p>
              </div>
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="requests.total > 0"
            :page="requests.page"
            :total="requests.total"
            :page-size="requests.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </div>

    <Teleport to="body">
      <div v-if="rejectingRequestId !== null" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeRejectDialog"></div>
        <div class="relative z-10 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.invoiceRequests.rejectDialogTitle') }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.invoiceRequests.rejectDialogDescription') }}</p>
          <div class="mt-4">
            <label class="input-label">{{ t('admin.invoiceRequests.rejectReason') }}</label>
            <textarea v-model="rejectReason" rows="4" class="input min-h-[120px]"></textarea>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="closeRejectDialog">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="submittingReject" @click="submitReject">
              {{ submittingReject ? t('admin.invoiceRequests.rejecting') : t('admin.invoiceRequests.confirmReject') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores'
import type { InvoiceRequestListResponse, InvoiceRequestStatus } from '@/types'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'

const appStore = useAppStore()
const { t } = useI18n()

const loading = ref(false)
const exporting = ref(false)
const includeExported = ref(false)
const expandedIds = ref<number[]>([])
const rejectingRequestId = ref<number | null>(null)
const submittingReject = ref(false)
const rejectReason = ref('')

const requests = reactive<InvoiceRequestListResponse>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
  pending_count: 0,
})

const filters = reactive({
  search: '',
  status: '',
  start_time: '',
  end_time: '',
})

const statusOptions = computed(() => [
  { value: '', label: t('admin.invoiceRequests.statusAll') },
  { value: 'pending', label: t('admin.invoiceRequests.status.pending') },
  { value: 'exported', label: t('admin.invoiceRequests.status.exported') },
  { value: 'completed', label: t('admin.invoiceRequests.status.completed') },
  { value: 'rejected', label: t('admin.invoiceRequests.status.rejected') },
])

const columns = computed<Column[]>(() => [
  { key: 'serial_no', label: t('admin.invoiceRequests.columns.serialNo') },
  { key: 'user_email', label: t('admin.invoiceRequests.columns.user') },
  { key: 'profile_snapshot', label: t('admin.invoiceRequests.columns.profile') },
  { key: 'total_amount_fen', label: t('admin.invoiceRequests.columns.amount') },
  { key: 'status', label: t('admin.invoiceRequests.columns.status') },
  { key: 'created_at', label: t('admin.invoiceRequests.columns.createdAt') },
  { key: 'actions', label: t('admin.invoiceRequests.columns.actions') },
])

const formatFen = (value: number) => (value / 100).toFixed(2)

const requestStatusLabel = (status: InvoiceRequestStatus) =>
  ({
    pending: t('admin.invoiceRequests.status.pending'),
    exported: t('admin.invoiceRequests.status.exported'),
    completed: t('admin.invoiceRequests.status.completed'),
    rejected: t('admin.invoiceRequests.status.rejected'),
  })[status] || status

const requestStatusBadgeClass = (status: InvoiceRequestStatus) =>
  status === 'completed' ? 'badge-success' : status === 'rejected' ? 'badge-danger' : 'badge-warning'

const getInvoiceRequestErrorMessage = (error: any, fallbackKey: string) => {
  const reason = error?.reason || error?.code
  if (reason === 'INVOICE_EXPORT_EMPTY') {
    return t('admin.invoiceRequests.exportEmpty')
  }
  return error?.message || t(fallbackKey)
}

const loadRequests = async () => {
  loading.value = true
  try {
    const data = await adminAPI.invoice.listInvoiceRequests({
      page: requests.page,
      page_size: requests.page_size,
      search: filters.search || undefined,
      status: filters.status || undefined,
      start_time: filters.start_time || undefined,
      end_time: filters.end_time || undefined,
    })
    Object.assign(requests, data)
  } catch (error: any) {
    appStore.showError(getInvoiceRequestErrorMessage(error, 'admin.invoiceRequests.failedToLoad'))
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | null = null
const handleSearch = () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void reload()
  }, 250)
}

const reload = async () => {
  requests.page = 1
  await loadRequests()
}

const resetFilters = async () => {
  filters.search = ''
  filters.status = ''
  filters.start_time = ''
  filters.end_time = ''
  await reload()
}

const handlePageChange = async (page: number) => {
  requests.page = page
  await loadRequests()
}

const handlePageSizeChange = async (pageSize: number) => {
  requests.page_size = pageSize
  requests.page = 1
  await loadRequests()
}

const toggleExpanded = (id: number) => {
  expandedIds.value = expandedIds.value.includes(id)
    ? expandedIds.value.filter((item) => item !== id)
    : [...expandedIds.value, id]
}

const completeRequest = async (id: number) => {
  try {
    await adminAPI.invoice.completeInvoiceRequest(id)
    appStore.showSuccess(t('admin.invoiceRequests.completeSuccess'))
    await loadRequests()
  } catch (error: any) {
    appStore.showError(getInvoiceRequestErrorMessage(error, 'admin.invoiceRequests.completeFailed'))
  }
}

const openRejectDialog = (id: number) => {
  rejectingRequestId.value = id
  rejectReason.value = ''
}

const closeRejectDialog = () => {
  rejectingRequestId.value = null
  rejectReason.value = ''
}

const submitReject = async () => {
  if (rejectingRequestId.value === null) return
  submittingReject.value = true
  try {
    await adminAPI.invoice.rejectInvoiceRequest(rejectingRequestId.value, rejectReason.value)
    appStore.showSuccess(t('admin.invoiceRequests.rejectSuccess'))
    closeRejectDialog()
    await loadRequests()
  } catch (error: any) {
    appStore.showError(getInvoiceRequestErrorMessage(error, 'admin.invoiceRequests.rejectFailed'))
  } finally {
    submittingReject.value = false
  }
}

const handleExport = async () => {
  if (requests.total === 0) {
    appStore.showWarning(t('admin.invoiceRequests.noRowsToExport'))
    return
  }

  if (!filters.status && !includeExported.value && (requests.pending_count || 0) === 0) {
    appStore.showWarning(t('admin.invoiceRequests.noPendingToExport'))
    return
  }

  exporting.value = true
  try {
    const blob = await adminAPI.invoice.exportInvoiceRequests({
      status: filters.status || undefined,
      search: filters.search || undefined,
      start_time: filters.start_time || undefined,
      end_time: filters.end_time || undefined,
      include_exported: includeExported.value,
    })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `invoice-requests-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.xlsx`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    appStore.showSuccess(t('admin.invoiceRequests.exportSuccess'))
    await loadRequests()
  } catch (error: any) {
    appStore.showError(getInvoiceRequestErrorMessage(error, 'admin.invoiceRequests.exportFailed'))
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  await loadRequests()
})
</script>
