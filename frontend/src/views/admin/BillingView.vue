<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.billing.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.billing.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="loadBilling"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="card in summaryCards"
          :key="card.key"
          class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="text-sm text-gray-500 dark:text-dark-400">{{ card.label }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ card.value }}
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-6">
          <div class="xl:col-span-2">
            <label class="input-label">{{ t('admin.billing.filters.search') }}</label>
            <input
              v-model.trim="filters.search"
              class="input"
              type="text"
              :placeholder="t('admin.billing.filters.searchPlaceholder')"
              @input="handleSearch"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.purpose') }}</label>
            <Select v-model="filters.purpose" :options="purposeOptions" @change="reloadFromFirstPage" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.salesStatus') }}</label>
            <Select
              v-model="filters.sales_status"
              :options="salesStatusOptions"
              @change="reloadFromFirstPage"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.redeemStatus') }}</label>
            <Select
              v-model="filters.redeem_status"
              :options="redeemStatusOptions"
              @change="reloadFromFirstPage"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.usedStart') }}</label>
            <input v-model="filters.used_start_time" class="input" type="date" @change="reloadFromFirstPage" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.usedEnd') }}</label>
            <input v-model="filters.used_end_time" class="input" type="date" @change="reloadFromFirstPage" />
          </div>
        </div>
        <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-4">
          <div>
            <label class="input-label">{{ t('admin.billing.filters.amountMin') }}</label>
            <input
              v-model.number="filters.amount_min"
              class="input"
              type="number"
              step="0.01"
              min="0"
              @change="reloadFromFirstPage"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.amountMax') }}</label>
            <input
              v-model.number="filters.amount_max"
              class="input"
              type="number"
              step="0.01"
              min="0"
              @change="reloadFromFirstPage"
            />
          </div>
          <div class="flex items-end gap-2 md:col-span-2">
            <button type="button" class="btn btn-secondary" @click="resetFilters">
              {{ t('common.reset') }}
            </button>
            <button type="button" class="btn btn-primary" :disabled="loading" @click="reloadFromFirstPage">
              {{ t('admin.billing.filters.apply') }}
            </button>
          </div>
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th class="table-th">{{ t('admin.billing.columns.code') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.amount') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.classification') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.buyer') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.redeemedBy') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.ledger') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.order') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.time') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="items.length === 0">
                <td colspan="8" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ t('admin.billing.empty') }}
                </td>
              </tr>
              <template v-else>
                <tr
                  v-for="item in items"
                  :key="item.id"
                  class="hover:bg-gray-50 dark:hover:bg-dark-700/40"
                >
                  <td class="table-td">
                    <div class="font-mono text-sm text-gray-900 dark:text-white">{{ item.code }}</div>
                    <div class="mt-1 max-w-48 truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ item.batch_name || t('admin.billing.noBatch') }}
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="font-medium text-gray-900 dark:text-white">{{ formatMoney(item.value) }}</div>
                  </td>
                  <td class="table-td">
                    <div class="flex flex-col gap-1">
                      <span class="badge badge-primary">{{ purposeLabel(item.purpose) }}</span>
                      <span :class="salesStatusBadgeClass(item.sales_status)">{{ salesStatusLabel(item.sales_status) }}</span>
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="max-w-44 truncate text-sm text-gray-700 dark:text-dark-300">
                      {{ item.sold_to_note || '-' }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">
                      {{ item.sold_at ? formatDateTime(item.sold_at) : '-' }}
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="max-w-48 truncate text-sm text-gray-900 dark:text-white">
                      {{ item.used_by_email || item.used_by_username || usedByFallback(item.used_by) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">
                      {{ item.used_at ? formatDateTime(item.used_at) : redeemStatusLabel(item.redeem_status) }}
                    </div>
                  </td>
                  <td class="table-td">
                    <span v-if="item.redeem_status !== 'used'" class="badge badge-gray">
                      {{ t('admin.billing.ledger.notRedeemed') }}
                    </span>
                    <span v-else-if="item.ledger_matched" class="badge badge-success">
                      {{ t('admin.billing.ledger.matched') }}
                    </span>
                    <span v-else class="badge badge-danger">
                      {{ t('admin.billing.ledger.missing') }}
                    </span>
                    <div v-if="item.ledger_delta !== null && item.ledger_delta !== undefined" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      {{ formatMoney(item.ledger_delta) }}
                    </div>
                  </td>
                  <td class="table-td">
                    <a
                      v-if="item.external_order_url"
                      :href="item.external_order_url"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                    >
                      {{ item.external_order_no || t('admin.billing.openOrder') }}
                    </a>
                    <span v-else class="text-sm text-gray-500 dark:text-dark-400">
                      {{ item.external_order_no || '-' }}
                    </span>
                    <div v-if="item.internal_notes" class="mt-1 max-w-48 truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ item.internal_notes }}
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="text-sm text-gray-700 dark:text-dark-300">
                      {{ formatDateTime(item.created_at) }}
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div class="border-t border-gray-200 p-4 dark:border-dark-700">
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatCurrency, formatDateTime } from '@/utils/format'
import type {
  RedeemCodeBillingFilters,
  RedeemCodeBillingItem,
  RedeemCodeBillingSummary,
  RedeemCodePurpose,
  RedeemCodeSalesStatus
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const emptySummary = (): RedeemCodeBillingSummary => ({
  sale_face_value: 0,
  sold_face_value: 0,
  redeemed_sale_amount: 0,
  sold_unredeemed_face_value: 0,
  gift_redeemed_amount: 0,
  compensation_redeemed_amount: 0,
  internal_test_redeemed_amount: 0,
  ledger_missing_count: 0,
  total_codes: 0,
  used_codes: 0,
  unused_codes: 0
})

const items = ref<RedeemCodeBillingItem[]>([])
const summary = ref<RedeemCodeBillingSummary>(emptySummary())
const loading = ref(false)
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 0
})
const filters = reactive({
  search: '',
  purpose: 'sale_recharge' as RedeemCodePurpose | '',
  sales_status: '' as RedeemCodeSalesStatus | '',
  redeem_status: '' as 'unused' | 'used' | 'expired' | '',
  amount_min: undefined as number | undefined,
  amount_max: undefined as number | undefined,
  used_start_time: '',
  used_end_time: ''
})

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null

const purposeOptions = computed(() => [
  { value: '', label: t('admin.billing.filters.allPurposes') },
  { value: 'sale_recharge', label: t('admin.redeem.purposes.sale_recharge') },
  { value: 'gift', label: t('admin.redeem.purposes.gift') },
  { value: 'compensation', label: t('admin.redeem.purposes.compensation') },
  { value: 'internal_test', label: t('admin.redeem.purposes.internal_test') },
  { value: 'migration', label: t('admin.redeem.purposes.migration') }
])

const salesStatusOptions = computed(() => [
  { value: '', label: t('admin.billing.filters.allSalesStatuses') },
  { value: 'sold', label: t('admin.redeem.salesStatus.sold') },
  { value: 'inventory', label: t('admin.redeem.salesStatus.inventory') },
  { value: 'gifted', label: t('admin.redeem.salesStatus.gifted') },
  { value: 'void', label: t('admin.redeem.salesStatus.void') }
])

const redeemStatusOptions = computed(() => [
  { value: '', label: t('admin.billing.filters.allRedeemStatuses') },
  { value: 'used', label: t('admin.redeem.status.used') },
  { value: 'unused', label: t('admin.redeem.status.unused') },
  { value: 'expired', label: t('admin.redeem.status.expired') }
])

const summaryCards = computed(() => [
  {
    key: 'redeemed_sale_amount',
    label: t('admin.billing.summary.redeemedSaleAmount'),
    value: formatMoney(summary.value.redeemed_sale_amount)
  },
  {
    key: 'sold_unredeemed_face_value',
    label: t('admin.billing.summary.soldUnredeemedFaceValue'),
    value: formatMoney(summary.value.sold_unredeemed_face_value)
  },
  {
    key: 'sold_face_value',
    label: t('admin.billing.summary.soldFaceValue'),
    value: formatMoney(summary.value.sold_face_value)
  },
  {
    key: 'ledger_missing_count',
    label: t('admin.billing.summary.ledgerMissingCount'),
    value: summary.value.ledger_missing_count.toString()
  }
])

function formatMoney(value: number | null | undefined) {
  return formatCurrency(value ?? 0, 'USD')
}

function purposeLabel(value: RedeemCodePurpose) {
  return t(`admin.redeem.purposes.${value || 'sale_recharge'}`)
}

function salesStatusLabel(value: RedeemCodeSalesStatus) {
  return t(`admin.redeem.salesStatus.${value || 'inventory'}`)
}

function redeemStatusLabel(value: string) {
  return t(`admin.redeem.status.${value || 'unused'}`)
}

function salesStatusBadgeClass(value: RedeemCodeSalesStatus) {
  if (value === 'sold') return 'badge badge-success'
  if (value === 'gifted') return 'badge badge-warning'
  if (value === 'void') return 'badge badge-danger'
  return 'badge badge-gray'
}

function usedByFallback(userID?: number | null) {
  return userID ? t('admin.redeem.userPrefix', { id: userID }) : '-'
}

function buildQuery(): RedeemCodeBillingFilters {
  const query: RedeemCodeBillingFilters = {
    page: pagination.page,
    page_size: pagination.page_size
  }
  if (filters.search) query.search = filters.search
  if (filters.purpose) query.purpose = filters.purpose
  if (filters.sales_status) query.sales_status = filters.sales_status
  if (filters.redeem_status) query.redeem_status = filters.redeem_status
  if (filters.amount_min !== undefined && filters.amount_min !== null) query.amount_min = filters.amount_min
  if (filters.amount_max !== undefined && filters.amount_max !== null) query.amount_max = filters.amount_max
  if (filters.used_start_time) query.used_start_time = filters.used_start_time
  if (filters.used_end_time) query.used_end_time = filters.used_end_time
  return query
}

async function loadBilling() {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.listBilling(buildQuery(), {
      signal: currentController.signal
    })
    if (currentController.signal.aborted) return
    items.value = response.items
    summary.value = response.summary || emptySummary()
    pagination.total = response.total
    pagination.page = response.page
    pagination.page_size = response.page_size
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(error.response?.data?.detail || t('admin.billing.loadFailed'))
    console.error('Error loading billing:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  loadBilling()
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(reloadFromFirstPage, 300)
}

function resetFilters() {
  filters.search = ''
  filters.purpose = 'sale_recharge'
  filters.sales_status = ''
  filters.redeem_status = ''
  filters.amount_min = undefined
  filters.amount_max = undefined
  filters.used_start_time = ''
  filters.used_end_time = ''
  reloadFromFirstPage()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadBilling()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadBilling()
}

onMounted(loadBilling)

onUnmounted(() => {
  if (abortController) abortController.abort()
  if (searchTimeout) clearTimeout(searchTimeout)
})
</script>

<style scoped>
.table-th {
  @apply whitespace-nowrap px-4 py-3 text-left text-xs font-semibold uppercase tracking-normal text-gray-500 dark:text-dark-400;
}

.table-td {
  @apply whitespace-nowrap px-4 py-3 align-top;
}
</style>
