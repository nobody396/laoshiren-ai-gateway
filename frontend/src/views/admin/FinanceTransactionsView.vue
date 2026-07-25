<template>
  <AppLayout>
    <div class="mb-6 grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
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

    <div class="card mb-4 p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.financeTransactions.summary.rangeTitle') }}
          </h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{
              summaryMode === 'all'
                ? t('admin.financeTransactions.summary.allTimeHint')
                : t('admin.financeTransactions.summary.monthHint')
            }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <div v-if="summaryMode === 'month'" class="flex items-center gap-1">
            <button class="btn btn-secondary !px-2 !py-1" @click="shiftMonth(-1)">
              <Icon name="chevronLeft" size="sm" />
            </button>
            <span class="min-w-[7rem] text-center text-sm text-gray-600 dark:text-gray-300">{{ rangeLabel }}</span>
            <button class="btn btn-secondary !px-2 !py-1" @click="shiftMonth(1)">
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
          <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
            <button
              data-test="summary-scope-all"
              :class="[
                'rounded-md px-3 py-1 text-xs font-medium transition-colors',
                summaryMode === 'all'
                  ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-600 dark:text-primary-400'
                  : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
              ]"
              @click="setSummaryMode('all')"
            >
              {{ t('admin.financeTransactions.summary.allTime') }}
            </button>
            <button
              data-test="summary-scope-month"
              :class="[
                'rounded-md px-3 py-1 text-xs font-medium transition-colors',
                summaryMode === 'month'
                  ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-600 dark:text-primary-400'
                  : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
              ]"
              @click="setSummaryMode('month')"
            >
              {{ t('admin.financeTransactions.summary.byMonth') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="mb-4 grid grid-cols-1 gap-4 xl:grid-cols-2">
      <div class="card p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.financeTransactions.summary.expenseStructure') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.expenseStructureHint') }}
        </p>
        <div v-if="!hasExpenseGroupData" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.noData') }}
        </div>
        <div v-else class="mt-4 grid items-center gap-5 sm:grid-cols-[11rem_1fr]">
          <div class="mx-auto h-44 w-44">
            <Doughnut :data="expenseGroupChartData" :options="structureChartOptions" />
          </div>
          <div class="space-y-3">
            <div
              v-for="group in expenseGroups"
              :key="group.key"
              class="flex items-center justify-between gap-3"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: group.color }"></span>
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ group.label }}</span>
                </div>
                <p class="ml-[1.125rem] text-xs text-gray-500 dark:text-gray-400">
                  {{ group.tx_count }} {{ t('admin.financeTransactions.summary.transactionsUnit') }}
                </p>
              </div>
              <span class="shrink-0 text-sm font-semibold tabular-nums text-red-600 dark:text-red-400">
                {{ formatCurrency(group.total_fen / 100, 'CNY') }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div class="card p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.financeTransactions.summary.incomeChannels') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.incomeChannelsHint') }}
        </p>
        <div v-if="!hasIncomeChannelData" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.financeTransactions.summary.noData') }}
        </div>
        <div v-else class="mt-4 grid items-center gap-5 sm:grid-cols-[11rem_1fr]">
          <div class="mx-auto h-44 w-44">
            <Doughnut :data="incomeChannelChartData" :options="structureChartOptions" />
          </div>
          <div class="space-y-3">
            <div
              v-for="channel in incomeChannelTotals"
              :key="channel.payment_channel"
              class="flex items-center justify-between gap-3"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: channel.color }"></span>
                  <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
                    {{ paymentChannelLabel(channel.payment_channel) }}
                  </span>
                </div>
                <p class="ml-[1.125rem] text-xs text-gray-500 dark:text-gray-400">
                  {{ channel.tx_count }} {{ t('admin.financeTransactions.summary.transactionsUnit') }}
                </p>
              </div>
              <span class="shrink-0 text-sm font-semibold tabular-nums text-green-600 dark:text-green-400">
                {{ formatCurrency(channel.total_fen / 100, 'CNY') }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card mb-6 p-5">
      <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.financeTransactions.summary.byCategory') }}
      </h3>
      <div v-if="summary.by_category.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.financeTransactions.summary.noData') }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[42rem] text-sm">
          <thead>
            <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
              <th class="pb-2">{{ t('admin.financeTransactions.columns.type') }}</th>
              <th class="pb-2">{{ t('admin.financeTransactions.summary.businessGroup') }}</th>
              <th class="pb-2">{{ t('admin.financeTransactions.columns.category') }}</th>
              <th class="pb-2 text-right">{{ t('admin.financeTransactions.columns.txCount') }}</th>
              <th class="pb-2 text-right">{{ t('admin.financeTransactions.columns.amount') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in summary.by_category" :key="`${c.type}:${c.category}`" class="border-t border-gray-100 dark:border-gray-700">
              <td class="py-2">
                <span :class="['badge', c.type === 'income' ? 'badge-success' : 'badge-gray']">
                  {{ typeLabel(c.type) }}
                </span>
              </td>
              <td class="py-2 text-gray-600 dark:text-gray-300">
                {{ businessGroupLabel(c) }}
              </td>
              <td class="py-1.5">
                <span class="text-gray-700 dark:text-gray-200">{{ categoryLabel(c.category) }}</span>
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

    <div class="card mb-6 p-4">
      <h3 class="mb-1 text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.financeTransactions.summary.trendTitle') }}
      </h3>
      <p class="mb-4 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.financeTransactions.summary.trendHint') }}
      </p>
      <div v-if="!hasTrendData" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.financeTransactions.summary.noTrendData') }}
      </div>
      <div v-else class="h-72">
        <Bar :data="trendChartData" :options="trendChartOptions" />
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
            <span class="text-sm text-gray-600 dark:text-gray-300">{{ formatFinanceDateTime(row.occurred_at) }}</span>
          </template>

          <template #cell-type="{ row }">
            <span :class="['badge', row.type === 'income' ? 'badge-success' : 'badge-gray']">
              {{ typeLabel(row.type) }}
            </span>
          </template>

          <template #cell-category="{ row }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ categoryLabel(row.category) }}</span>
          </template>

          <template #cell-paymentChannel="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300">
              {{ row.payment_channel ? paymentChannelLabel(row.payment_channel) : '-' }}
            </span>
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
            <Select
              v-model="form.type"
              data-test="transaction-type-select"
              :options="typeOptions"
              @change="onTypeChange"
            />
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

        <div v-if="form.type === 'income'">
          <label class="input-label">
            {{ t('admin.financeTransactions.form.paymentChannel') }}
            <span class="text-red-500">*</span>
          </label>
          <Select
            v-model="form.payment_channel"
            data-test="payment-channel-select"
            :options="paymentChannelOptions"
          />
          <p class="input-hint">{{ t('admin.financeTransactions.form.paymentChannelHint') }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.financeTransactions.form.note') }}</label>
          <textarea v-model="form.note" rows="3" class="input"></textarea>
        </div>

        <div>
          <label class="input-label">
            {{ t('admin.financeTransactions.form.receipt') }}
            <span v-if="form.type === 'income'" class="text-red-500">*</span>
          </label>
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
          <p class="input-hint">
            {{
              form.type === 'income'
                ? t('admin.financeTransactions.form.incomeReceiptHint')
                : t('admin.financeTransactions.form.receiptHint')
            }}
          </p>
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

    <!-- Receipt Preview -->
    <BaseDialog
      :show="showReceiptViewer"
      :title="t('admin.financeTransactions.receiptPreviewTitle')"
      width="extra-wide"
      close-on-click-outside
      @close="closeReceiptPreview"
    >
      <div
        data-test="receipt-preview"
        class="relative flex min-h-72 items-center justify-center overflow-hidden rounded-xl bg-gray-100 p-3 dark:bg-dark-900"
      >
        <div
          v-if="receiptViewerLoading"
          class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 text-sm text-gray-500 dark:text-gray-400"
        >
          <Icon name="refresh" size="lg" class="animate-spin" />
          <span>{{ t('admin.financeTransactions.receiptPreviewLoading') }}</span>
        </div>
        <div
          v-if="receiptViewerFailed"
          class="py-16 text-center text-sm text-red-600 dark:text-red-400"
        >
          {{ t('admin.financeTransactions.receiptPreviewFailed') }}
        </div>
        <img
          v-if="receiptViewerUrl && !receiptViewerFailed"
          :src="receiptViewerUrl"
          :alt="t('admin.financeTransactions.receiptPreviewTitle')"
          :class="[
            'max-h-[70vh] max-w-full rounded-lg object-contain shadow-sm transition-opacity',
            receiptViewerLoading ? 'opacity-0' : 'opacity-100'
          ]"
          @load="receiptViewerLoading = false"
          @error="handleReceiptPreviewError"
        />
      </div>

      <template #footer>
        <div class="flex w-full items-center justify-end gap-3">
          <a
            v-if="receiptViewerUrl && !receiptViewerFailed"
            :href="receiptViewerUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary"
          >
            <Icon name="download" size="sm" class="mr-1" />
            {{ t('admin.financeTransactions.downloadReceipt') }}
          </a>
          <button type="button" class="btn btn-primary" @click="closeReceiptPreview">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, ArcElement, BarElement, CategoryScale, LinearScale, Tooltip, Legend } from 'chart.js'
import { Bar, Doughnut } from 'vue-chartjs'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { formatCurrency } from '@/utils/format'
import type {
  FinanceCategoryTotal,
  FinanceTransaction,
  FinanceTransactionCategory,
  FinancePaymentChannel,
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

ChartJS.register(ArcElement, BarElement, CategoryScale, LinearScale, Tooltip, Legend)

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
  'hosting_cost',
  'cdn_cost',
  'domain_cost',
  'domain_email_cost',
  'early_cost',
  'other_expense'
]
const FORM_EXPENSE_CATEGORIES = EXPENSE_CATEGORIES.filter((category) => category !== 'server_cost')
const PAYMENT_CHANNELS: FinancePaymentChannel[] = [
  'wechat',
  'alipay',
  'liandong_shop'
]

const categoryLabel = (category: string) =>
  t(`admin.financeTransactions.categoryLabels.${category}`, category)

const typeLabel = (type: string) =>
  type === 'income' ? t('admin.financeTransactions.typeLabels.income') : t('admin.financeTransactions.typeLabels.expense')

const paymentChannelLabel = (channel: string) =>
  t(`admin.financeTransactions.paymentChannelLabels.${channel}`, channel)

const FINANCE_TIME_ZONE = 'Asia/Shanghai'

function financeDateParts(date: Date) {
  return new Intl.DateTimeFormat('en-US', {
    timeZone: FINANCE_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  }).formatToParts(date)
}

function financeDatePart(parts: Intl.DateTimeFormatPart[], type: Intl.DateTimeFormatPartTypes) {
  return parts.find((part) => part.type === type)?.value ?? ''
}

function formatFinanceDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, {
    timeZone: FINANCE_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23'
  }).format(date)
}

function formatFinanceDateTimeInput(timestampSeconds: number) {
  const parts = financeDateParts(new Date(timestampSeconds * 1000))
  return [
    financeDatePart(parts, 'year'),
    financeDatePart(parts, 'month'),
    financeDatePart(parts, 'day')
  ].join('-') + `T${financeDatePart(parts, 'hour')}:${financeDatePart(parts, 'minute')}`
}

function parseFinanceDateTimeInput(value: string) {
  if (!value) return null
  const timestamp = Date.parse(`${value}:00+08:00`)
  return Number.isNaN(timestamp) ? null : Math.floor(timestamp / 1000)
}

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
  { key: 'paymentChannel', label: t('admin.financeTransactions.columns.paymentChannel') },
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

// ===== Summary (all-time by default, with a Shanghai-month drilldown) =====
type SummaryMode = 'all' | 'month'

const emptySummary = (): FinanceTransactionSummary => ({
  range_from: '',
  range_to: '',
  total_income_fen: 0,
  total_expense_fen: 0,
  net_profit_fen: 0,
  margin_percent: 0,
  by_category: [],
  by_payment_channel: [],
  monthly_series: []
})

const summaryMode = ref<SummaryMode>('all')
const summaryMonthOffset = ref(0) // 0 = current month, -1 = last month, ...

function currentShanghaiYearMonth(offset = 0) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: 'numeric'
  }).formatToParts(new Date())
  const year = Number(parts.find((part) => part.type === 'year')?.value)
  const month = Number(parts.find((part) => part.type === 'month')?.value)
  const shifted = new Date(Date.UTC(year, month - 1 + offset, 1))
  return { year: shifted.getUTCFullYear(), month: shifted.getUTCMonth() + 1 }
}

const summaryRange = computed(() => {
  const { year, month } = currentShanghaiYearMonth(summaryMonthOffset.value)
  const shanghaiOffsetMs = 8 * 60 * 60 * 1000
  const fromMs = Date.UTC(year, month - 1, 1) - shanghaiOffsetMs
  const toMs = Date.UTC(year, month, 1) - shanghaiOffsetMs
  return {
    from: Math.floor(fromMs / 1000),
    to: Math.floor(toMs / 1000),
    label: new Date(Date.UTC(year, month - 1, 1))
  }
})

const rangeLabel = computed(() =>
  summaryRange.value.label.toLocaleDateString(undefined, { year: 'numeric', month: 'long', timeZone: 'UTC' })
)

function shiftMonth(delta: number) {
  summaryMonthOffset.value += delta
  loadSummary()
}

const summary = ref<FinanceTransactionSummary>(emptySummary())
const allTimeSummary = ref<FinanceTransactionSummary>(emptySummary())

async function setSummaryMode(mode: SummaryMode) {
  if (summaryMode.value === mode) return
  summaryMode.value = mode
  await loadSummary()
}

async function loadSummary() {
  try {
    if (summaryMode.value === 'all') {
      const loaded = await adminAPI.financeTransactions.summary(undefined, undefined, 'all')
      allTimeSummary.value = loaded
      summary.value = loaded
      return
    }
    summary.value = await adminAPI.financeTransactions.summary(
      summaryRange.value.from,
      summaryRange.value.to,
      'month'
    )
  } catch (error: any) {
    console.error('Error loading finance summary:', error)
  }
}

async function refreshSummaryData() {
  if (summaryMode.value === 'all') {
    await loadSummary()
    return
  }
  await Promise.all([
    loadSummary(),
    adminAPI.financeTransactions.summary(undefined, undefined, 'all').then((loaded) => {
      allTimeSummary.value = loaded
    })
  ])
}

type ExpenseGroupKey = 'fixed_business' | 'procurement' | 'setup' | 'other'

const FIXED_BUSINESS_CATEGORIES = new Set<FinanceTransactionCategory>([
  'server_cost',
  'hosting_cost',
  'cdn_cost',
  'domain_cost',
  'domain_email_cost'
])

function expenseGroupKeyForCategory(category: FinanceTransactionCategory): ExpenseGroupKey {
  if (FIXED_BUSINESS_CATEGORIES.has(category)) return 'fixed_business'
  if (category === 'upstream_topup') return 'procurement'
  if (category === 'early_cost') return 'setup'
  return 'other'
}

const EXPENSE_GROUP_META: Array<{ key: ExpenseGroupKey; color: string }> = [
  { key: 'fixed_business', color: '#f59e0b' },
  { key: 'procurement', color: '#ef4444' },
  { key: 'setup', color: '#8b5cf6' },
  { key: 'other', color: '#64748b' }
]

const expenseGroups = computed(() =>
  EXPENSE_GROUP_META.map((meta) => {
    const items = summary.value.by_category.filter(
      (item) =>
        item.type === 'expense' &&
        expenseGroupKeyForCategory(item.category) === meta.key
    )
    return {
      ...meta,
      label: t(`admin.financeTransactions.summary.expenseGroups.${meta.key}`),
      total_fen: items.reduce((total, item) => total + item.total_fen, 0),
      tx_count: items.reduce((total, item) => total + item.tx_count, 0)
    }
  }).filter((group) => group.total_fen > 0)
)

const INCOME_CHANNEL_COLORS: Record<FinancePaymentChannel, string> = {
  wechat: '#22c55e',
  alipay: '#1677ff',
  liandong_shop: '#f97316',
  bank_transfer: '#8b5cf6',
  other: '#94a3b8'
}

const incomeChannelTotals = computed(() =>
  (summary.value.by_payment_channel ?? [])
    .filter((channel) => channel.total_fen > 0)
    .map((channel) => ({
      ...channel,
      color: INCOME_CHANNEL_COLORS[channel.payment_channel] ?? INCOME_CHANNEL_COLORS.other
    }))
)

const hasExpenseGroupData = computed(() => expenseGroups.value.length > 0)
const hasIncomeChannelData = computed(() => incomeChannelTotals.value.length > 0)

const expenseGroupChartData = computed(() => ({
  labels: expenseGroups.value.map((group) => group.label),
  datasets: [
    {
      data: expenseGroups.value.map((group) => group.total_fen),
      backgroundColor: expenseGroups.value.map((group) => group.color),
      borderWidth: 0
    }
  ]
}))

const incomeChannelChartData = computed(() => ({
  labels: incomeChannelTotals.value.map((channel) => paymentChannelLabel(channel.payment_channel)),
  datasets: [
    {
      data: incomeChannelTotals.value.map((channel) => channel.total_fen),
      backgroundColor: incomeChannelTotals.value.map((channel) => channel.color),
      borderWidth: 0
    }
  ]
}))

const structureChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } }
}

function businessGroupLabel(item: FinanceCategoryTotal) {
  if (item.type === 'income') {
    return t('admin.financeTransactions.summary.businessGroups.income')
  }
  const key = expenseGroupKeyForCategory(item.category)
  return t(`admin.financeTransactions.summary.expenseGroups.${key}`)
}

const trendSeries = computed(() => {
  const totalsByMonth = new Map(allTimeSummary.value.monthly_series.map((item) => [item.month, item]))
  const current = currentShanghaiYearMonth()
  return Array.from({ length: 12 }, (_, index) => {
    const shifted = new Date(Date.UTC(current.year, current.month - 12 + index, 1))
    const month = `${shifted.getUTCFullYear()}-${String(shifted.getUTCMonth() + 1).padStart(2, '0')}`
    return totalsByMonth.get(month) ?? {
      month,
      total_income_fen: 0,
      total_expense_fen: 0,
      net_profit_fen: 0
    }
  })
})

const hasTrendData = computed(() =>
  trendSeries.value.some((item) => item.total_income_fen > 0 || item.total_expense_fen > 0)
)

const trendChartData = computed(() => ({
  labels: trendSeries.value.map((item) => item.month),
  datasets: [
    {
      label: t('admin.financeTransactions.summary.income'),
      data: trendSeries.value.map((item) => item.total_income_fen / 100),
      backgroundColor: '#10b981',
      borderRadius: 4
    },
    {
      label: t('admin.financeTransactions.summary.expense'),
      data: trendSeries.value.map((item) => item.total_expense_fen / 100),
      backgroundColor: '#ef4444',
      borderRadius: 4
    }
  ]
}))

const trendChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top' as const }
  },
  scales: {
    y: { beginAtZero: true }
  }
}

// ===== Create/Edit dialog =====
const showEditDialog = ref(false)
const saving = ref(false)
const editingTransaction = ref<FinanceTransaction | null>(null)
const isEditing = computed(() => !!editingTransaction.value)

const form = reactive({
  type: 'expense' as FinanceTransactionType,
  category: 'hosting_cost' as FinanceTransactionCategory,
  payment_channel: '' as FinancePaymentChannel | '',
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
  const list =
    form.type === 'income'
      ? INCOME_CATEGORIES
      : form.category === 'server_cost'
        ? [...FORM_EXPENSE_CATEGORIES, 'server_cost' as FinanceTransactionCategory]
        : FORM_EXPENSE_CATEGORIES
  return list.map((c) => ({ value: c, label: categoryLabel(c) }))
})

const paymentChannelOptions = computed(() => [
  { value: '', label: t('admin.financeTransactions.form.selectPaymentChannel') },
  ...PAYMENT_CHANNELS.map((channel) => ({ value: channel, label: paymentChannelLabel(channel) }))
])

function onTypeChange() {
  const list = form.type === 'income' ? INCOME_CATEGORIES : FORM_EXPENSE_CATEGORIES
  if (!list.includes(form.category)) {
    form.category = list[0]
  }
  if (form.type === 'expense') {
    form.payment_channel = ''
  }
}

function resetForm() {
  form.type = 'expense'
  form.category = 'hosting_cost'
  form.payment_channel = ''
  form.amount_yuan = ''
  form.occurred_at_str = formatFinanceDateTimeInput(Math.floor(Date.now() / 1000))
  form.note = ''
  form.receipt_key = ''
  receiptPreviewUrl.value = ''
}

function fillFormFromTransaction(row: FinanceTransaction) {
  form.type = row.type
  form.category = row.category
  form.payment_channel = row.payment_channel || ''
  form.amount_yuan = (row.amount_fen / 100).toFixed(2)
  form.occurred_at_str = formatFinanceDateTimeInput(Math.floor(new Date(row.occurred_at).getTime() / 1000))
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
  if (form.type === 'income' && !form.receipt_key) {
    appStore.showError(t('admin.financeTransactions.form.receiptRequired'))
    return
  }
  if (form.type === 'income' && !form.payment_channel) {
    appStore.showError(t('admin.financeTransactions.form.paymentChannelRequired'))
    return
  }
  const occurredAt = parseFinanceDateTimeInput(form.occurred_at_str)

  saving.value = true
  try {
    if (!editingTransaction.value) {
      await adminAPI.financeTransactions.create({
        type: form.type,
        category: form.category,
        amount_fen: amountFen,
        occurred_at: occurredAt ?? undefined,
        note: form.note || undefined,
        receipt_key: form.receipt_key || undefined,
        payment_channel: form.type === 'income' ? form.payment_channel || undefined : undefined
      })
      appStore.showSuccess(t('common.success'))
    } else {
      await adminAPI.financeTransactions.update(editingTransaction.value.id, {
        type: form.type,
        category: form.category,
        amount_fen: amountFen,
        occurred_at: occurredAt ?? undefined,
        note: form.note,
        receipt_key: form.receipt_key,
        payment_channel: form.type === 'income' ? form.payment_channel : ''
      })
      appStore.showSuccess(t('common.success'))
    }
    showEditDialog.value = false
    editingTransaction.value = null
    await Promise.all([loadTransactions(), refreshSummaryData()])
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
  showReceiptViewer.value = true
  receiptViewerLoading.value = true
  receiptViewerFailed.value = false
  receiptViewerUrl.value = ''
  try {
    const { url } = await adminAPI.financeTransactions.getReceiptUrl(row.id)
    receiptViewerUrl.value = url
  } catch (error: any) {
    console.error('Failed to load receipt url:', error)
    receiptViewerLoading.value = false
    receiptViewerFailed.value = true
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.failedToLoadReceipt'))
  }
}

const showReceiptViewer = ref(false)
const receiptViewerUrl = ref('')
const receiptViewerLoading = ref(false)
const receiptViewerFailed = ref(false)

function handleReceiptPreviewError() {
  receiptViewerLoading.value = false
  receiptViewerFailed.value = true
  appStore.showError(t('admin.financeTransactions.receiptPreviewFailed'))
}

function closeReceiptPreview() {
  showReceiptViewer.value = false
  receiptViewerUrl.value = ''
  receiptViewerLoading.value = false
  receiptViewerFailed.value = false
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
    await Promise.all([loadTransactions(), refreshSummaryData()])
  } catch (error: any) {
    console.error('Failed to delete finance transaction:', error)
    appStore.showError(error.response?.data?.detail || t('admin.financeTransactions.failedToDelete'))
  }
}

onMounted(async () => {
  await Promise.all([loadTransactions(), loadSummary()])
})
</script>
