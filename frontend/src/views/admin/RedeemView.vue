<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <!-- Left: Search + Filters -->
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.redeem.searchCodes')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select
            v-model="filters.type"
            :options="filterTypeOptions"
            class="w-36"
            @change="loadCodes"
          />
          <Select
            v-model="filters.status"
            :options="filterStatusOptions"
            class="w-36"
            @change="loadCodes"
          />

          <!-- Right: Action buttons -->
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadCodes"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="handleExportCodes" class="btn btn-secondary">
              {{ t('admin.redeem.exportCsv') }}
            </button>
            <button @click="showGenerateDialog = true" class="btn btn-primary">
              {{ t('admin.redeem.generateCodes') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="codes" :loading="loading">
          <template #cell-code="{ value }">
            <div class="flex items-center space-x-2">
              <code class="font-mono text-sm text-gray-900 dark:text-gray-100">{{ value }}</code>
              <button
                @click="copyToClipboard(value)"
                :class="[
                  'flex items-center transition-colors',
                  copiedCode === value
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                ]"
                :title="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
              >
                <Icon v-if="copiedCode !== value" name="copy" size="sm" :stroke-width="2" />
                <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </button>
            </div>
          </template>

          <template #cell-type="{ value }">
            <span
              :class="[
                'badge',
                value === 'balance'
                  ? 'badge-success'
                  : value === 'subscription'
                    ? 'badge-warning'
                    : 'badge-primary'
              ]"
            >
              {{ t('admin.redeem.types.' + value) }}
            </span>
          </template>

          <template #cell-value="{ value, row }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">
              <template v-if="row.type === 'balance'">${{ value.toFixed(2) }}</template>
              <template v-else-if="row.type === 'subscription'">
                {{ row.validity_days || 30 }} {{ t('admin.redeem.days') }}
                <span v-if="row.group" class="ml-1 text-xs text-gray-500 dark:text-gray-400"
                  >({{ row.group.name }})</span
                >
              </template>
              <template v-else>{{ value }}</template>
            </span>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
                'badge',
                value === 'unused'
                  ? 'badge-success'
                  : value === 'used'
                    ? 'badge-gray'
                    : 'badge-danger'
              ]"
            >
              {{ t('admin.redeem.status.' + value) }}
            </span>
          </template>

          <template #cell-purpose="{ value }">
            <span class="badge badge-primary">
              {{ t('admin.redeem.purposes.' + (value || 'sale_recharge')) }}
            </span>
          </template>

          <template #cell-sales_status="{ value }">
            <span
              :class="[
                'badge',
                value === 'sold'
                  ? 'badge-success'
                  : value === 'gifted'
                    ? 'badge-warning'
                    : value === 'void'
                      ? 'badge-danger'
                      : 'badge-gray'
              ]"
            >
              {{ t('admin.redeem.salesStatus.' + (value || 'inventory')) }}
            </span>
          </template>

          <template #cell-batch="{ row }">
            <div class="max-w-44 truncate text-sm text-gray-700 dark:text-dark-300">
              {{ row.batch?.name || row.external_order_no || '-' }}
            </div>
          </template>

          <template #cell-used_by="{ value, row }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ row.user?.email || (value ? t('admin.redeem.userPrefix', { id: value }) : '-') }}
            </span>
          </template>

          <template #cell-used_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{
              value ? formatDateTime(value) : '-'
            }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-2">
              <button
                v-if="row.status === 'unused'"
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
              <span v-else class="text-gray-400 dark:text-dark-500">-</span>
            </div>
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

        <!-- Batch Actions -->
        <div v-if="filters.status === 'unused'" class="flex justify-end">
          <button @click="showDeleteUnusedDialog = true" class="btn btn-danger">
            {{ t('admin.redeem.deleteAllUnused') }}
          </button>
        </div>
      </template>
    </TablePageLayout>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.redeem.deleteCode')"
      :message="t('admin.redeem.deleteCodeConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Delete Unused Codes Dialog -->
    <ConfirmDialog
      :show="showDeleteUnusedDialog"
      :title="t('admin.redeem.deleteAllUnused')"
      :message="t('admin.redeem.deleteAllUnusedConfirm')"
      :confirm-text="t('admin.redeem.deleteAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDeleteUnused"
      @cancel="showDeleteUnusedDialog = false"
    />

    <!-- Generate Codes Dialog -->
    <Teleport to="body">
      <div v-if="showGenerateDialog" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="fixed inset-0 bg-black/50" @click="showGenerateDialog = false"></div>
        <div
          class="relative z-10 max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800"
        >
          <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.redeem.generateCodesTitle') }}
          </h2>
          <form @submit.prevent="handleGenerateCodes" class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
              <Select v-model="generateForm.type" :options="typeOptions" />
            </div>
            <div v-if="generateForm.type === 'balance'" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
              <label class="input-label">{{ t('admin.redeem.flow.label') }}</label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
                <button
                  v-for="preset in generatePresetOptions"
                  :key="preset.value"
                  type="button"
                  @click="applyGeneratePreset(preset.value)"
                  :class="[
                    'rounded-lg border-2 px-3 py-2.5 text-left transition-colors',
                    activeGeneratePreset === preset.value
                      ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                      : 'border-gray-200 text-gray-700 hover:border-primary-300 dark:border-dark-600 dark:text-dark-200'
                  ]"
                >
                  <span class="block text-sm font-semibold">{{ preset.label }}</span>
                  <span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-400">
                    {{ preset.description }}
                  </span>
                </button>
              </div>
            </div>
            <!-- 余额/并发类型：显示数值输入 -->
            <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'">
              <label class="input-label">
                {{
                  generateForm.type === 'balance'
                    ? t('admin.redeem.amount')
                    : t('admin.redeem.columns.value')
                }}
              </label>
              <input
                v-model.number="generateForm.value"
                type="number"
                :step="generateForm.type === 'balance' ? '0.01' : '1'"
                :min="generateForm.type === 'balance' ? '0.01' : '1'"
                required
                class="input"
              />
            </div>
            <!-- 邀请码类型：显示提示信息 -->
            <div v-if="generateForm.type === 'invitation'" class="rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
              <p class="text-sm text-blue-700 dark:text-blue-300">
                {{ t('admin.redeem.invitationHint') }}
              </p>
            </div>
            <!-- 订阅类型：显示分组选择和有效天数 -->
            <template v-if="generateForm.type === 'subscription'">
              <div>
                <label class="input-label">{{ t('admin.redeem.selectGroup') }}</label>
                <Select
                  v-model="generateForm.group_id"
                  :options="subscriptionGroupOptions"
                  :placeholder="t('admin.redeem.selectGroupPlaceholder')"
                >
                  <template #selected="{ option }">
                    <GroupBadge
                      v-if="option"
                      :name="(option as unknown as GroupOption).label"
                      :platform="(option as unknown as GroupOption).platform"
                      :subscription-type="(option as unknown as GroupOption).subscriptionType"
                      :rate-multiplier="(option as unknown as GroupOption).rate"
                    />
                    <span v-else class="text-gray-400">{{
                      t('admin.redeem.selectGroupPlaceholder')
                    }}</span>
                  </template>
                  <template #option="{ option, selected }">
                    <GroupOptionItem
                      :name="(option as unknown as GroupOption).label"
                      :platform="(option as unknown as GroupOption).platform"
                      :subscription-type="(option as unknown as GroupOption).subscriptionType"
                      :rate-multiplier="(option as unknown as GroupOption).rate"
                      :description="(option as unknown as GroupOption).description"
                      :selected="selected"
                    />
                  </template>
                </Select>
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
                <input
                  v-model.number="generateForm.validity_days"
                  type="number"
                  min="1"
                  max="365"
                  required
                  class="input"
                />
              </div>
            </template>
            <div>
              <label class="input-label">{{ t('admin.redeem.count') }}</label>
              <input
                v-model.number="generateForm.count"
                type="number"
                min="1"
                max="100"
                required
                class="input"
              />
            </div>
            <div
              v-if="generateForm.type === 'balance'"
              class="grid grid-cols-1 gap-4 border-t border-gray-100 pt-4 dark:border-dark-700 sm:grid-cols-2"
            >
              <div>
                <label class="input-label">{{ t('admin.redeem.batchName') }}</label>
                <input
                  v-model.trim="generateForm.batch_name"
                  type="text"
                  :placeholder="t('admin.redeem.batchNamePlaceholder')"
                  class="input"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.purpose') }}</label>
                <Select v-model="generateForm.purpose" :options="purposeOptions" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.salesStatusLabel') }}</label>
                <Select v-model="generateForm.sales_status" :options="salesStatusOptions" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.salesChannel') }}</label>
                <input
                  v-model.trim="generateForm.sales_channel"
                  type="text"
                  :placeholder="t('admin.redeem.salesChannelPlaceholder')"
                  class="input"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.externalOrderNo') }}</label>
                <input
                  v-model.trim="generateForm.external_order_no"
                  type="text"
                  :placeholder="t('admin.redeem.externalOrderNoPlaceholder')"
                  class="input"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.externalOrderUrl') }}</label>
                <input
                  v-model.trim="generateForm.external_order_url"
                  type="url"
                  :placeholder="t('admin.redeem.externalOrderUrlPlaceholder')"
                  class="input"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.soldToNote') }}</label>
                <input
                  v-model.trim="generateForm.sold_to_note"
                  type="text"
                  :placeholder="t('admin.redeem.soldToNotePlaceholder')"
                  class="input"
                />
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.externalUrl') }}</label>
                <input
                  v-model.trim="generateForm.external_url"
                  type="url"
                  :placeholder="t('admin.redeem.externalUrlPlaceholder')"
                  class="input"
                />
              </div>
              <div class="sm:col-span-2">
                <label class="input-label">{{ t('admin.redeem.internalNotes') }}</label>
                <textarea
                  v-model.trim="generateForm.internal_notes"
                  rows="2"
                  :placeholder="t('admin.redeem.internalNotesPlaceholder')"
                  class="input"
                ></textarea>
              </div>
            </div>
            <div class="flex justify-end gap-3 pt-2">
              <button type="button" @click="showGenerateDialog = false" class="btn btn-secondary">
                {{ t('common.cancel') }}
              </button>
              <button type="submit" :disabled="generating" class="btn btn-primary">
                {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Generated Codes Result Dialog -->
    <Teleport to="body">
      <div v-if="showResultDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeResultDialog"></div>
        <div class="relative z-10 w-full max-w-lg rounded-xl bg-white shadow-xl dark:bg-dark-800">
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-600"
          >
            <div class="flex items-center gap-3">
              <div
                class="flex h-10 w-10 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30"
              >
                <svg
                  class="h-5 w-5 text-green-600 dark:text-green-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </div>
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.redeem.generatedSuccessfully') }}
                </h2>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.redeem.codesCreated', { count: generatedCodes.length }) }}
                </p>
              </div>
            </div>
            <button
              @click="closeResultDialog"
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
            >
              <Icon name="x" size="md" :stroke-width="2" />
            </button>
          </div>
          <!-- Content -->
          <div class="p-5">
            <div class="mb-3 flex flex-wrap gap-2">
              <button
                type="button"
                :class="['btn btn-sm', resultOutputMode === 'links' ? 'btn-primary' : 'btn-secondary']"
                @click="resultOutputMode = 'links'"
              >
                {{ t('admin.redeem.redeemLinks') }}
              </button>
              <button
                type="button"
                :class="['btn btn-sm', resultOutputMode === 'codes' ? 'btn-primary' : 'btn-secondary']"
                @click="resultOutputMode = 'codes'"
              >
                {{ t('admin.redeem.rawCodes') }}
              </button>
            </div>
            <div class="relative">
              <textarea
                readonly
                :value="generatedResultText"
                :style="{ height: textareaHeight }"
                class="w-full resize-none rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-sm text-gray-800 focus:outline-none dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200"
              ></textarea>
            </div>
          </div>
          <!-- Footer -->
          <div
            class="flex justify-end gap-2 rounded-b-xl border-t border-gray-200 bg-gray-50 px-5 py-4 dark:border-dark-600 dark:bg-dark-700/50"
          >
            <button
              @click="copyGeneratedCodes"
              :class="[
                'btn flex items-center gap-2 transition-all',
                copiedAll ? 'btn-success' : 'btn-secondary'
              ]"
            >
              <Icon v-if="!copiedAll" name="copy" size="sm" :stroke-width="2" />
              <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
            </button>
            <button @click="downloadGeneratedCodes" class="btn btn-primary flex items-center gap-2">
              <Icon name="download" size="sm" :stroke-width="2" />
              {{ t('admin.redeem.download') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type {
  RedeemCode,
  RedeemCodeType,
  RedeemCodePurpose,
  RedeemCodeSalesStatus,
  Group,
  GroupPlatform,
  SubscriptionType
} from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const subscriptionGroups = ref<Group[]>([])

// 订阅类型分组选项
const subscriptionGroupOptions = computed(() => {
  return subscriptionGroups.value
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
})

const generatedCodesText = computed(() => {
  return generatedCodes.value.map((code) => code.code).join('\n')
})

const redeemURLBase = computed(() => {
  if (typeof window === 'undefined') return ''
  return window.location.origin
})

const buildRedeemURL = (code: string) => {
  const base = redeemURLBase.value.replace(/\/$/, '')
  return `${base}/redeem?code=${encodeURIComponent(code)}`
}

const generatedRedeemLinksText = computed(() => {
  return generatedCodes.value.map((code) => buildRedeemURL(code.code)).join('\n')
})

const resultOutputMode = ref<'links' | 'codes'>('links')
const generatedResultText = computed(() =>
  resultOutputMode.value === 'links' ? generatedRedeemLinksText.value : generatedCodesText.value
)

const textareaHeight = computed(() => {
  const lineCount = generatedCodes.value.length
  const lineHeight = 24 // approximate line height in px
  const padding = 24 // top + bottom padding
  const minHeight = 60
  const maxHeight = 240
  const calculatedHeight = Math.min(
    Math.max(lineCount * lineHeight + padding, minHeight),
    maxHeight
  )
  return `${calculatedHeight}px`
})

const copiedAll = ref(false)

const closeResultDialog = () => {
  showResultDialog.value = false
  generatedCodes.value = []
  copiedAll.value = false
  resultOutputMode.value = 'links'
}

const copyGeneratedCodes = async () => {
  const success = await clipboardCopy(generatedResultText.value, t('admin.redeem.copied'))
  if (success) {
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedResultText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-${resultOutputMode.value}-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const columns = computed<Column[]>(() => [
  { key: 'code', label: t('admin.redeem.columns.code') },
  { key: 'type', label: t('admin.redeem.columns.type'), sortable: true },
  { key: 'value', label: t('admin.redeem.columns.value'), sortable: true },
  { key: 'status', label: t('admin.redeem.columns.status'), sortable: true },
  { key: 'purpose', label: t('admin.redeem.columns.purpose') },
  { key: 'sales_status', label: t('admin.redeem.columns.salesStatus') },
  { key: 'batch', label: t('admin.redeem.columns.batch') },
  { key: 'used_by', label: t('admin.redeem.columns.usedBy') },
  { key: 'used_at', label: t('admin.redeem.columns.usedAt'), sortable: true },
  { key: 'actions', label: t('admin.redeem.columns.actions') }
])

const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterTypeOptions = computed(() => [
  { value: '', label: t('admin.redeem.allTypes') },
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterStatusOptions = computed(() => [
  { value: '', label: t('admin.redeem.allStatus') },
  { value: 'unused', label: t('admin.redeem.unused') },
  { value: 'used', label: t('admin.redeem.used') },
  { value: 'expired', label: t('admin.redeem.status.expired') }
])

const purposeOptions = computed(() => [
  { value: 'sale_recharge', label: t('admin.redeem.purposes.sale_recharge') },
  { value: 'gift', label: t('admin.redeem.purposes.gift') },
  { value: 'compensation', label: t('admin.redeem.purposes.compensation') },
  { value: 'internal_test', label: t('admin.redeem.purposes.internal_test') },
  { value: 'migration', label: t('admin.redeem.purposes.migration') }
])

const salesStatusOptions = computed(() => [
  { value: 'inventory', label: t('admin.redeem.salesStatus.inventory') },
  { value: 'sold', label: t('admin.redeem.salesStatus.sold') },
  { value: 'gifted', label: t('admin.redeem.salesStatus.gifted') },
  { value: 'void', label: t('admin.redeem.salesStatus.void') }
])

type GeneratePreset = 'store_inventory' | 'gift'

const generatePresetOptions = computed<Array<{ value: GeneratePreset; label: string; description: string }>>(() => [
  {
    value: 'store_inventory',
    label: t('admin.redeem.flow.storeInventory'),
    description: t('admin.redeem.flow.storeInventoryDesc')
  },
  {
    value: 'gift',
    label: t('admin.redeem.flow.gift'),
    description: t('admin.redeem.flow.giftDesc')
  }
])

const codes = ref<RedeemCode[]>([])
const loading = ref(false)
const generating = ref(false)
const searchQuery = ref('')
const filters = reactive({
  type: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 0
})

let abortController: AbortController | null = null

const showDeleteDialog = ref(false)
const showDeleteUnusedDialog = ref(false)
const deletingCode = ref<RedeemCode | null>(null)
const copiedCode = ref<string | null>(null)

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  group_id: null as number | null,
  validity_days: 30,
  batch_name: '',
  purpose: 'sale_recharge' as RedeemCodePurpose,
  sales_status: 'inventory' as RedeemCodeSalesStatus,
  sales_channel: '',
  external_url: '',
  sold_to_note: '',
  external_order_no: '',
  external_order_url: '',
  internal_notes: ''
})

const activeGeneratePreset = computed<GeneratePreset | ''>(() => {
  if (
    generateForm.type === 'balance' &&
    generateForm.purpose === 'sale_recharge' &&
    generateForm.sales_status === 'inventory'
  ) {
    return 'store_inventory'
  }
  if (
    generateForm.type === 'balance' &&
    generateForm.purpose === 'gift' &&
    generateForm.sales_status === 'gifted'
  ) {
    return 'gift'
  }
  return ''
})

const applyGeneratePreset = (preset: GeneratePreset) => {
  generateForm.type = 'balance'
  if (preset === 'store_inventory') {
    generateForm.purpose = 'sale_recharge'
    generateForm.sales_status = 'inventory'
    if (!generateForm.sales_channel) {
      generateForm.sales_channel = 'liandong_shop'
    }
    return
  }
  generateForm.purpose = 'gift'
  generateForm.sales_status = 'gifted'
}

// 监听类型变化，邀请码类型时自动设置 value 为 0
watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

watch(
  () => generateForm.purpose,
  (purpose) => {
    if (purpose === 'gift' || purpose === 'compensation') {
      generateForm.sales_status = 'gifted'
    } else if (purpose === 'sale_recharge' && generateForm.sales_status === 'gifted') {
      generateForm.sales_status = 'inventory'
    } else if ((purpose === 'internal_test' || purpose === 'migration') && generateForm.sales_status === 'sold') {
      generateForm.sales_status = 'inventory'
    }
  }
)

const loadCodes = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.list(
      pagination.page,
      pagination.page_size,
      {
        type: filters.type as RedeemCodeType,
        status: filters.status as any,
        search: searchQuery.value || undefined
      },
      {
        signal: currentController.signal
      }
    )
    if (currentController.signal.aborted) {
      return
    }
    codes.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.redeem.failedToLoad'))
    console.error('Error loading redeem codes:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadCodes()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadCodes()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadCodes()
}

const handleGenerateCodes = async () => {
  // 订阅类型必须选择分组
  if (generateForm.type === 'subscription' && !generateForm.group_id) {
    appStore.showError(t('admin.redeem.groupRequired'))
    return
  }

  generating.value = true
  try {
    const payload = {
      count: generateForm.count,
      type: generateForm.type,
      value: generateForm.value
    } as const
    const result = await adminAPI.redeem.generate({
      ...payload,
      group_id: generateForm.type === 'subscription' ? generateForm.group_id : undefined,
      validity_days: generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      batch_name: generateForm.type === 'balance' ? generateForm.batch_name : undefined,
      purpose: generateForm.type === 'balance' ? generateForm.purpose : undefined,
      sales_status: generateForm.type === 'balance' ? generateForm.sales_status : undefined,
      sales_channel: generateForm.type === 'balance' ? generateForm.sales_channel : undefined,
      external_url: generateForm.type === 'balance' ? generateForm.external_url : undefined,
      sold_to_note: generateForm.type === 'balance' ? generateForm.sold_to_note : undefined,
      external_order_no:
        generateForm.type === 'balance' ? generateForm.external_order_no : undefined,
      external_order_url:
        generateForm.type === 'balance' ? generateForm.external_order_url : undefined,
      internal_notes: generateForm.type === 'balance' ? generateForm.internal_notes : undefined
    })
    showGenerateDialog.value = false
    generatedCodes.value = result
    showResultDialog.value = true
    // 重置表单
    generateForm.group_id = null
    generateForm.validity_days = 30
    generateForm.batch_name = ''
    generateForm.sales_channel = ''
    generateForm.external_url = ''
    generateForm.sold_to_note = ''
    generateForm.external_order_no = ''
    generateForm.external_order_url = ''
    generateForm.internal_notes = ''
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToGenerate'))
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
}

const copyToClipboard = async (text: string) => {
  const success = await clipboardCopy(text, t('admin.redeem.copied'))
  if (success) {
    copiedCode.value = text
    setTimeout(() => {
      copiedCode.value = null
    }, 2000)
  }
}

const handleExportCodes = async () => {
  try {
    const blob = await adminAPI.redeem.exportCodes({
      type: filters.type as RedeemCodeType,
      status: filters.status as any,
      include_redeem_url: true,
      redeem_url_base: redeemURLBase.value
    })

    // Create download link
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    appStore.showSuccess(t('admin.redeem.codesExported'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToExport'))
    console.error('Error exporting codes:', error)
  }
}

const handleDelete = (code: RedeemCode) => {
  deletingCode.value = code
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingCode.value) return

  try {
    await adminAPI.redeem.delete(deletingCode.value.id)
    appStore.showSuccess(t('admin.redeem.codeDeleted'))
    showDeleteDialog.value = false
    deletingCode.value = null
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDelete'))
    console.error('Error deleting code:', error)
  }
}

const confirmDeleteUnused = async () => {
  try {
    // Get all unused codes and delete them
    const unusedCodesResponse = await adminAPI.redeem.list(1, 1000, { status: 'unused' })
    const unusedCodeIds = unusedCodesResponse.items.map((code) => code.id)

    if (unusedCodeIds.length === 0) {
      appStore.showInfo(t('admin.redeem.noUnusedCodes'))
      showDeleteUnusedDialog.value = false
      return
    }

    const result = await adminAPI.redeem.batchDelete(unusedCodeIds)
    appStore.showSuccess(t('admin.redeem.codesDeleted', { count: result.deleted }))
    showDeleteUnusedDialog.value = false
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDeleteUnused'))
    console.error('Error deleting unused codes:', error)
  }
}

// 加载订阅类型分组
const loadSubscriptionGroups = async () => {
  try {
    const groups = await adminAPI.groups.getAll()
    subscriptionGroups.value = groups
  } catch (error) {
    console.error('Error loading subscription groups:', error)
  }
}

onMounted(() => {
  loadCodes()
  loadSubscriptionGroups()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
