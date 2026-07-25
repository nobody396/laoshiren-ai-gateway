<template>
  <AppLayout>
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <StatCard
        :title="t('admin.financeTransactions.summary.income')"
        :value="formatCurrency(summary.total_income_fen / 100, 'CNY')"
        :icon="TrendingUpIconRaw"
        icon-variant="success"
      />
      <StatCard
        :title="t('admin.financeTransactions.summary.expense')"
        :value="formatCurrency(summary.total_expense_fen / 100, 'CNY')"
        :icon="TrendingDownIconRaw"
        icon-variant="danger"
      />
      <StatCard
        :title="t('admin.financeTransactions.summary.netProfit')"
        :value="formatCurrency(summary.net_profit_fen / 100, 'CNY')"
        :icon="DollarIconRaw"
        :icon-variant="summary.net_profit_fen >= 0 ? 'success' : 'danger'"
      />
      <StatCard
        :title="t('admin.financeTransactions.summary.margin')"
        :value="`${summary.margin_percent.toFixed(1)}%`"
        :icon="ChartIconRaw"
        :icon-variant="summary.margin_percent >= 0 ? 'primary' : 'danger'"
      />
    </div>

    <div class="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
      <div class="card p-4 lg:col-span-1">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.financeTransactions.summary.rangeTitle') }}
          </h3>
          <div class="flex items-center gap-1">
            <button class="btn btn-secondary !px-2 !py-1" @click="shiftMonth(-1)">
              <Icon name="chevronLeft" size="sm" />
            </button>
            <span class="min-w-[7rem] text-center text-sm text-gray-600 dark:text-gray-300">{{ rangeLabel }}</span>
            <button class="btn btn-secondary !px-2 !py-1" @click="shiftMonth(1)">
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
        </div>
        <div v-if="summary.by_category.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.noData') }}
        </div>
        <div v-else class="h-40 w-40 mx-auto">
          <Doughnut :data="categoryChartData" :options="categoryChartOptions" />
        </div>
      </div>

      <div class="card p-4 lg:col-span-2">
        <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.financeTransactions.summary.byCategory') }}
        </h3>
        <div v-if="summary.by_category.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.noData') }}
        </div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
              <th class="pb-2">{{ t('admin.financeTransactions.columns.category') }}</th>
              <th class="pb-2 text-right">{{ t('admin.financeTransactions.columns.txCount') }}</th>
              <th class="pb-2 text-right">{{ t('admin.financeTransactions.columns.amount') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in summary.by_category" :key="`${c.type}:${c.category}`" class="border-t border-gray-100 dark:border-gray-700">
              <td class="py-1.5">
                <span :class="['badge', c.type === 'income' ? 'badge-success' : 'badge-gray']">
                  {{ categoryLabel(c.category) }}
                </span>
              </td>
              <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ c.tx_count }}</td>
              <td class="py-1.5 text-right" :class="c.type === 'income' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                {{ c.type === 'income' ? '+' : '-' }}{{ formatCurrency(c.total_fen / 100, 'CNY') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.financeTransactions.searchPlaceholder')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select v-model="filters.type" :options="typeFilterOptions" class="w-36" @change="handleFilterChange" />
          <Select v-model="filters.category" :options="categoryFilterOptions" class="w-44" @change="handleFilterChange" />

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button @click="loadTransactions" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreateDialog" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.financeTransactions.record') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="transactions" :loading="loading">
          <template #cell-occurredAt="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ formatDateTime(row.occurred_at) }}</span>
          </template>

          <template #cell-type="{ row }">
            <span :class="['badge', row.type === 'income' ? 'badge-success' : 'badge-gray']">
              {{ typeLabel(row.type) }}
            </span>
          </template>

          <template #cell-category="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ categoryLabel(row.category) }}</span>
          </template>

          <template #cell-amount="{ row }">
            <span
              class="font-medium"
              :class="row.type === 'income' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
            >
              {{ row.type === 'income' ? '+' : '-' }}{{ formatCurrency(row.amount_fen / 100, 'CNY') }}
            </span>
          </template>

          <template #cell-note="{ row }">
            <span class="block max-w-xs truncate text-sm text-gray-600 dark:text-gray-300" :title="row.note || ''">
              {{ row.note || '-' }}
            </span>
          </template>

          <template #cell-source="{ row }">
            <span :class="['badge', row.source === 'skill' ? 'badge-warning' : 'badge-gray']">
              {{ row.source === 'skill' ? t('admin.financeTransactions.sourceLabels.skill') : t('admin.financeTransactions.sourceLabels.manual') }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                v-if="row.receipt_key"
                @click="viewReceipt(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                :title="t('admin.financeTransactions.viewReceipt')"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                @click="openEditDialog(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                :title="t('common.edit')"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('empty.noData')"
              :description="t('admin.financeTransactions.failedToLoad')"
              :action-text="t('admin.financeTransactions.record')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create/Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="isEditing ? t('admin.financeTransactions.editTransaction') : t('admin.financeTransactions.record')"
      @close="closeEdit"
    >
      <form id="finance-transaction-form" @submit.prevent="handleSave" class="space-y-4">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.financeTransactions.form.type') }}</label>
            <Select v-model="form.type" :options="typeOptions" @change="onTypeChange" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.financeTransactions.form.category') }}</label>
            <Select v-model="form.category" :options="categoryOptionsForForm" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.financeTransactions.form.amount') }}</label>
            <input v-model="form.amount_yuan" type="number" min="0.01" step="0.01" class="input" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.financeTransactions.form.occurredAt') }}</label>
            <input v-model="form.occurred_at_str" type="datetime-local" class="input" required />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.financeTransactions.form.note') }}</label>
          <textarea v-model="form.note" rows="3" class="input"></textarea>
        </div>

        <div>
          <label class="input-label">{{ t('admin.financeTransactions.form.receipt') }}</label>
          <div class="flex items-center gap-3">
            <input ref="fileInputRef" type="file" accept="image/*" class="hidden" @change="handleFileSelected" />
            <button type="button" class="btn btn-secondary" :disabled="uploadingReceipt" @click="fileInputRef?.click()">
              <Icon name="upload" size="sm" class="mr-1" />
              {{ uploadingReceipt ? t('admin.financeTransactions.form.uploading') : t('admin.financeTransactions.form.chooseReceipt') }}
            </button>
            <img v-if="receiptPreviewUrl" :src="receiptPreviewUrl" class="h-12 w-12 rounded object-cover ring-1 ring-gray-200 dark:ring-gray-700" />
            <span v-else-if="form.receipt_key" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.financeTransactions.form.receiptAttached') }}
            </span>
            <button v-if="form.receipt_key" type="button" class="text-xs text-red-500 hover:underline" @click="clearReceipt">
              {{ t('common.delete') }}
            </button>
          </div>
          <p class="input-hint">{{ t('admin.financeTransactions.form.receiptHint') }}</p>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="closeEdit" class="btn btn-secondary">{{ t('common.cancel') }}</button>
          <button type="submit" form="finance-transaction-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.financeTransactions.deleteTransaction')"
      :message="t('admin.financeTransactions.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatCurrency, formatDateTime, formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import type {
  FinanceTransaction,
  FinanceTransactionCategory,
  FinanceTransactionSummary,
  FinanceTransactionType
} from '@/types'
import type { Column } from '@/components/common/types'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatCard from '@/components/common/StatCard.vue'
import Icon from '@/components/icons/Icon.vue'

ChartJS.register(ArcElement, Tooltip, Legend)

const { t } = useI18n()
const appStore = useAppStore()

// StatCard renders `icon` as a Vue component via <component :is>. Icon.vue takes a `name`
// prop instead, so wrap the handful of icon names this page needs as tiny render functions.
const TrendingUpIconRaw = { render: () => h(Icon, { name: 'trendingUp' }) }
const TrendingDownIconRaw = { render: () => h(Icon, { name: 'trendingUp', class: 'rotate-180' }) }
const DollarIconRaw = { render: () => h(Icon, { name: 'dollar' }) }
const ChartIconRaw = { render: () => h(Icon, { name: 'chartBar' }) }

const INCOME_CATEGORIES: FinanceTransactionCategory[] = ['sale_revenue', 'other_income']
const EXPENSE_CATEGORIES: FinanceTransactionCategory[] = [
  'upstream_topup',
  'server_cost',
  'domain_cost',
  'early_cost',
  'other_expense'
]

const categoryLabel = (category: string) =>
  t(`admin.financeTransactions.categoryLabels.${category}`, category)

const typeLabel = (type: string) =>
  type === 'income' ? t('admin.financeTransactions.typeLabels.income') : t('admin.financeTransactions.typeLabels.expense')

// ===== List state =====
const transactions = ref<FinanceTransaction[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({ type: '', category: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 0 })

const typeFilterOptions = computed(() => [
  { value: '', label: t('admin.financeTransactions.allTypes') },
  { value: 'income', label: t('admin.financeTransactions.typeLabels.income') },
  { value: 'expense', label: t('admin.financeTransactions.typeLabels.expense') }
])

const categoryFilterOptions = computed(() => [
  { value: '', label: t('admin.financeTransactions.allCategories') },
  ...[...INCOME_CATEGORIES, ...EXPENSE_CATEGORIES].map((c) => ({ value: c, label: categoryLabel(c) }))
])

const columns = computed<Column[]>(() => [
  { key: 'occurredAt', label: t('admin.financeTransactions.columns.occurredAt') },
  { key: 'type', label: t('admin.financeTransactions.columns.type') },
  { key: 'category', label: t('admin.financeTransactions.columns.category') },
  { key: 'amount', label: t('admin.financeTransactions.columns.amount') },
  { key: 'note', label: t('admin.financeTransactions.columns.note') },
  { key: 'source', label: t('admin.financeTransactions.columns.source') },
  { key: 'actions', label: t('admin.financeTransactions.columns.actions') }
])

let currentController: AbortController | null = null

async function loadTransactions() {
  if (currentController) currentController.abort()
  currentController = new AbortController()

  try {
    loading.value = true
    const res = await adminAPI.financeTransactions.list(pagination.page, pagination.page_size, {
      type: filters.type || undefined,
      category: filters.category || undefined,
      search: searchQuery.value || undefined
    })
    transactions.value = res.items
    pagination.total = res.total
    pagination.pages = res.pages
    pagination.page = res.page
    pagination.page_size = res.page_size
  } catch (error: any) {
    if (currentController.signal.aborted || error?.name === 'AbortError') return
    console.error('Error loading finance transactions:', error)
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadTransactions()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadTransactions()
}

function handleFilterChange() {
  pagination.page = 1
  loadTransactions()
}

let searchDebounceTimer: number | null = null
function handleSearch() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = window.setTimeout(() => {
    pagination.page = 1
    loadTransactions()
  }, 300)
}

// ===== Summary (month range) =====
const summaryMonthOffset = ref(0) // 0 = current month, -1 = last month, ...

const summaryRange = computed(() => {
  const now = new Date()
  const base = new Date(now.getFullYear(), now.getMonth() + summaryMonthOffset.value, 1)
  const from = new Date(base.getFullYear(), base.getMonth(), 1)
  const to = new Date(base.getFullYear(), base.getMonth() + 1, 1)
  return { from: Math.floor(from.getTime() / 1000), to: Math.floor(to.getTime() / 1000), label: base }
})

const rangeLabel = computed(() =>
  summaryRange.value.label.toLocaleDateString(undefined, { year: 'numeric', month: 'long' })
)

function shiftMonth(delta: number) {
  summaryMonthOffset.value += delta
  loadSummary()
}

const summary = ref<FinanceTransactionSummary>({
  range_from: '',
  range_to: '',
  total_income_fen: 0,
  total_expense_fen: 0,
  net_profit_fen: 0,
  margin_percent: 0,
  by_category: []
})

async function loadSummary() {
  try {
    summary.value = await adminAPI.financeTransactions.summary(summaryRange.value.from, summaryRange.value.to)
  } catch (error: any) {
    console.error('Error loading finance summary:', error)
  }
}

const categoryChartData = computed(() => {
  const colors = ['#10b981', '#3b82f6', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6']
  return {
    labels: summary.value.by_category.map((c) => categoryLabel(c.category)),
    datasets: [
      {
        data: summary.value.by_category.map((c) => c.total_fen),
        backgroundColor: summary.value.by_category.map((_, i) => colors[i % colors.length]),
        borderWidth: 0
      }
    ]
  }
})

const categoryChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } }
}

// ===== Create/Edit dialog =====
const showEditDialog = ref(false)
const saving = ref(false)
const editingTransaction = ref<FinanceTransaction | null>(null)
const isEditing = computed(() => !!editingTransaction.value)

const form = reactive({
  type: 'expense' as FinanceTransactionType,
  category: 'server_cost' as FinanceTransactionCategory,
  amount_yuan: '',
  occurred_at_str: '',
  note: '',
  receipt_key: '' as string | undefined
})

const typeOptions = computed(() => [
  { value: 'income', label: t('admin.financeTransactions.typeLabels.income') },
  { value: 'expense', label: t('admin.financeTransactions.typeLabels.expense') }
])

const categoryOptionsForForm = computed(() => {
  const list = form.type === 'income' ? INCOME_CATEGORIES : EXPENSE_CATEGORIES
  return list.map((c) => ({ value: c, label: categoryLabel(c) }))
})

function onTypeChange() {
  const list = form.type === 'income' ? INCOME_CATEGORIES : EXPENSE_CATEGORIES
  if (!list.includes(form.category)) {
    form.category = list[0]
  }
}

function resetForm() {
  form.type = 'expense'
  form.category = 'server_cost'
  form.amount_yuan = ''
  form.occurred_at_str = formatDateTimeLocalInput(Math.floor(Date.now() / 1000))
  form.note = ''
  form.receipt_key = ''
  receiptPreviewUrl.value = ''
}

function fillFormFromTransaction(row: FinanceTransaction) {
  form.type = row.type
  form.category = row.category
  form.amount_yuan = (row.amount_fen / 100).toFixed(2)
  form.occurred_at_str = formatDateTimeLocalInput(Math.floor(new Date(row.occurred_at).getTime() / 1000))
  form.note = row.note || ''
  form.receipt_key = row.receipt_key || ''
  receiptPreviewUrl.value = ''
}

function openCreateDialog() {
  editingTransaction.value = null
  resetForm()
  showEditDialog.value = true
}

function openEditDialog(row: FinanceTransaction) {
  editingTransaction.value = row
  fillFormFromTransaction(row)
  showEditDialog.value = true
}

function closeEdit() {
  showEditDialog.value = false
  editingTransaction.value = null
}

async function handleSave() {
  const amountFen = Math.round(parseFloat(form.amount_yuan || '0') * 100)
  if (!amountFen || amountFen <= 0) {
    appStore.showError(t('admin.financeTransactions.form.invalidAmount'))
    return
  }
  const occurredAt = parseDateTimeLocalInput(form.occurred_at_str)

  saving.value = true
  try {
    if (!editingTransaction.value) {
      await adminAPI.financeTransactions.create({
        type: form.type,
        category: form.category,
        amount_fen: amountFen,
        occurred_at: occurredAt ?? undefined,
        note: form.note || undefined,
        receipt_key: form.receipt_key || undefined
      })
      appStore.showSuccess(t('common.success'))
    } else {
      await adminAPI.financeTransactions.update(editingTransaction.value.id, {
        type: form.type,
        category: form.category,
        amount_fen: amountFen,
        occurred_at: occurredAt ?? undefined,
        note: form.note,
        receipt_key: form.receipt_key
      })
      appStore.showSuccess(t('common.success'))
    }
    showEditDialog.value = false
    editingTransaction.value = null
    await Promise.all([loadTransactions(), loadSummary()])
  } catch (error: any) {
    console.error('Failed to save finance transaction:', error)
    appStore.showError(
      error.response?.data?.detail ||
        (editingTransaction.value ? t('admin.financeTransactions.failedToUpdate') : t('admin.financeTransactions.failedToCreate'))
    )
  } finally {
    saving.value = false
  }
}

// ===== Receipt upload (client-side compress before upload) =====
const fileInputRef = ref<HTMLInputElement | null>(null)
const uploadingReceipt = ref(false)
const receiptPreviewUrl = ref('')

async function compressImageToWebp(file: File, maxDim = 1600, quality = 0.75): Promise<Blob> {
  const bitmap = await createImageBitmap(file)
  let { width, height } = bitmap
  if (width > maxDim || height > maxDim) {
    const scale = maxDim / Math.max(width, height)
    width = Math.round(width * scale)
    height = Math.round(height * scale)
  }
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('canvas 2d context unavailable')
  ctx.drawImage(bitmap, 0, 0, width, height)
  return await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('compress failed'))), 'image/webp', quality)
  })
}

async function handleFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  uploadingReceipt.value = true
  try {
    const compressed = await compressImageToWebp(file)
    receiptPreviewUrl.value = URL.createObjectURL(compressed)
    const result = await adminAPI.financeTransactions.uploadReceipt(compressed, `${file.name.replace(/\.[^.]+$/, '')}.webp`)
    form.receipt_key = result.key
  } catch (error: any) {
    console.error('Failed to upload receipt:', error)
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.form.uploadFailed'))
    receiptPreviewUrl.value = ''
  } finally {
    uploadingReceipt.value = false
  }
}

function clearReceipt() {
  form.receipt_key = ''
  receiptPreviewUrl.value = ''
}

async function viewReceipt(row: FinanceTransaction) {
  try {
    const { url } = await adminAPI.financeTransactions.getReceiptUrl(row.id)
    window.open(url, '_blank', 'noopener')
  } catch (error: any) {
    console.error('Failed to load receipt url:', error)
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.failedToLoadReceipt'))
  }
}

// ===== Delete =====
const showDeleteDialog = ref(false)
const deletingTransaction = ref<FinanceTransaction | null>(null)

function handleDelete(row: FinanceTransaction) {
  deletingTransaction.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deletingTransaction.value) return
  try {
    await adminAPI.financeTransactions.delete(deletingTransaction.value.id)
    appStore.showSuccess(t('common.success'))
    showDeleteDialog.value = false
    deletingTransaction.value = null
    await Promise.all([loadTransactions(), loadSummary()])
  } catch (error: any) {
    console.error('Failed to delete finance transaction:', error)
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.failedToDelete'))
  }
}

onMounted(async () => {
  await Promise.all([loadTransactions(), loadSummary()])
})
</script>
