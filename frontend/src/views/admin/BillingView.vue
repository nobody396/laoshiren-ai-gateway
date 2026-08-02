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

      <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-100">
        <div class="flex gap-3">
          <Icon name="dollar" size="md" class="mt-0.5 shrink-0 text-emerald-600 dark:text-emerald-300" />
          <div>
            <div class="font-semibold">{{ t('admin.billing.revenueScopeTitle') }}</div>
            <div class="mt-1 text-emerald-800 dark:text-emerald-200">
              {{ t('admin.billing.revenueScopeDescription') }}
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-12">
          <div class="xl:col-span-5">
            <label class="input-label">{{ t('admin.billing.filters.search') }}</label>
            <input
              v-model.trim="filters.search"
              class="input"
              type="text"
              :placeholder="t('admin.billing.filters.searchPlaceholder')"
              @input="handleSearch"
            />
          </div>
          <div class="xl:col-span-2">
            <label class="input-label">{{ t('admin.billing.filters.exactAmount') }}</label>
            <input
              v-model.number="filters.amount_exact"
              class="input"
              type="number"
              step="0.01"
              min="0"
              @change="reloadFromFirstPage"
            />
          </div>
          <div class="xl:col-span-2">
            <label class="input-label">{{ t('admin.billing.filters.redeemStatus') }}</label>
            <Select
              v-model="filters.redeem_status"
              :options="redeemStatusOptions"
              @change="reloadFromFirstPage"
            />
          </div>
          <div class="flex items-end gap-2 xl:col-span-3">
            <button type="button" class="btn btn-secondary" @click="resetFilters">
              {{ t('common.reset') }}
            </button>
            <button type="button" class="btn btn-primary" :disabled="loading" @click="reloadFromFirstPage">
              {{ t('admin.billing.filters.apply') }}
            </button>
          </div>
        </div>

        <div class="mt-3">
          <button
            type="button"
            class="inline-flex items-center gap-2 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
            @click="showAdvancedFilters = !showAdvancedFilters"
          >
            <Icon name="filter" size="sm" />
            {{ showAdvancedFilters ? t('admin.billing.filters.hideAdvanced') : t('admin.billing.filters.showAdvanced') }}
          </button>
        </div>

        <div v-if="showAdvancedFilters" class="mt-3 grid grid-cols-1 gap-3 border-t border-gray-100 pt-3 md:grid-cols-2 xl:grid-cols-6 dark:border-dark-700">
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
            <label class="input-label">{{ t('admin.billing.filters.usedStart') }}</label>
            <input v-model="filters.used_start_time" class="input" type="date" @change="reloadFromFirstPage" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.billing.filters.usedEnd') }}</label>
            <input v-model="filters.used_end_time" class="input" type="date" @change="reloadFromFirstPage" />
          </div>
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
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th class="table-th">{{ t('admin.billing.columns.paidCard') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.amount') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.purchase') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.redeemAccount') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.paymentStatus') }}</th>
                <th class="table-th">{{ t('admin.billing.columns.time') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="items.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
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
                    <div class="font-medium text-gray-900 dark:text-white">{{ formatMoney(item.paid_value) }}</div>
                    <div v-if="item.value > item.paid_value" class="mt-1 text-xs text-emerald-600 dark:text-emerald-400">
                      到账 {{ formatMoney(item.value) }}
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="max-w-56 truncate text-sm text-gray-900 dark:text-white">
                      {{ item.sold_to_note || t('admin.billing.noBuyerNote') }}
                    </div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      <a
                        v-if="item.external_order_url"
                        :href="item.external_order_url"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="inline-flex items-center gap-1 font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                      >
                        {{ item.external_order_no || t('admin.billing.openOrder') }}
                        <Icon name="externalLink" size="xs" />
                      </a>
                      <span v-else>
                        {{ item.external_order_no || t('admin.billing.noOrder') }}
                      </span>
                    </div>
                    <div v-if="item.internal_notes" class="mt-1 max-w-56 truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ item.internal_notes }}
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
                    <span :class="paymentStatusBadgeClass(item)">
                      {{ paymentStatusLabel(item) }}
                    </span>
                    <div v-if="item.ledger_delta !== null && item.ledger_delta !== undefined" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      {{ formatMoney(item.ledger_delta) }}
                    </div>
                  </td>
                  <td class="table-td">
                    <div class="text-sm text-gray-700 dark:text-dark-300">
                      {{ item.sold_at ? formatDateTime(item.sold_at) : formatDateTime(item.created_at) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">
                      {{ item.sold_at ? t('admin.billing.time.soldAt') : t('admin.billing.time.createdAt') }}
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
const showAdvancedFilters = ref(false)
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 0
})
const filters = reactive({
  search: '',
  purpose: 'sale_recharge' as RedeemCodePurpose | '',
  sales_status: 'sold' as RedeemCodeSalesStatus | '',
  redeem_status: '' as 'unused' | 'used' | 'expired' | '',
  amount_exact: undefined as number | undefined,
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
    key: 'paid_recharge_total',
    label: t('admin.billing.summary.paidRechargeTotal'),
    value: formatMoney(summary.value.sold_face_value)
  },
  {
    key: 'paid_card_count',
    label: t('admin.billing.summary.paidCardCount'),
    value: summary.value.total_codes.toString()
  },
  {
    key: 'redeemed_sale_amount',
    label: t('admin.billing.summary.redeemedSaleAmount'),
    value: formatMoney(summary.value.redeemed_sale_amount)
  },
  {
    key: 'sold_unredeemed_face_value',
    label: t('admin.billing.summary.soldUnredeemedFaceValue'),
    value: formatMoney(summary.value.sold_unredeemed_face_value)
  }
])

function formatMoney(value: number | null | undefined) {
  return formatCurrency(value ?? 0, 'USD')
}

function redeemStatusLabel(value: string) {
  return t(`admin.redeem.status.${value || 'unused'}`)
}

function usedByFallback(userID?: number | null) {
  return userID ? t('admin.redeem.userPrefix', { id: userID }) : '-'
}

function paymentStatusLabel(item: RedeemCodeBillingItem) {
  if (item.redeem_status !== 'used') return t('admin.billing.status.paidNotRedeemed')
  if (item.ledger_matched) return t('admin.billing.status.paidAndPosted')
  return t('admin.billing.status.needsReview')
}

function paymentStatusBadgeClass(item: RedeemCodeBillingItem) {
  if (item.redeem_status !== 'used') return 'badge badge-warning'
  if (item.ledger_matched) return 'badge badge-success'
  return 'badge badge-danger'
}

function normalizeOptionalNumber(value: number | undefined | null) {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
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
  const exactAmount = normalizeOptionalNumber(filters.amount_exact)
  if (exactAmount !== undefined) {
    query.amount_min = exactAmount
    query.amount_max = exactAmount
  } else {
    const minAmount = normalizeOptionalNumber(filters.amount_min)
    const maxAmount = normalizeOptionalNumber(filters.amount_max)
    if (minAmount !== undefined) query.amount_min = minAmount
    if (maxAmount !== undefined) query.amount_max = maxAmount
  }
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
  filters.sales_status = 'sold'
  filters.redeem_status = ''
  filters.amount_exact = undefined
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
