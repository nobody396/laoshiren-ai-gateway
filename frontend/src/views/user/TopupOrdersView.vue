<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid gap-4" :class="invoiceManagementEnabled ? 'lg:grid-cols-[minmax(0,1fr)_320px]' : ''">
        <div class="card p-6">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">我的充值订单</h1>
              <p v-if="invoiceManagementEnabled" class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                仅已完成支付且未申请开票的订单可参与合并开票，合计满 ¥{{ formatFen(invoiceMinimumAmountFen) }} 后可申请。
              </p>
            </div>
            <button v-if="invoiceManagementEnabled" class="btn btn-primary" :disabled="!canSubmitInvoiceRequest" @click="openApplyDialog">
              申请开票
            </button>
          </div>
        </div>

        <div v-if="invoiceManagementEnabled" class="card p-6">
          <p class="text-sm font-medium text-gray-500 dark:text-dark-400">可申请开票金额</p>
          <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">
            ¥{{ formatFen(listData.selectable_amount_fen || 0) }}
          </p>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
            当前已选 {{ selectedOrderIds.length }} 笔，合计 ¥{{ formatFen(selectedAmountFen) }}
          </p>
          <p
            class="mt-1 text-xs"
            :class="canSubmitInvoiceRequest ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'"
          >
            最低开票金额 ¥{{ formatFen(invoiceMinimumAmountFen) }}
            <span v-if="selectedAmountShortfallFen > 0">，还差 ¥{{ formatFen(selectedAmountShortfallFen) }}</span>
          </p>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-wrap items-center gap-3">
            <Select v-model="filters.status" :options="statusOptions" class="w-40" @change="reload" />
            <Select v-if="invoiceManagementEnabled" v-model="filters.invoice_status" :options="invoiceStatusOptions" class="w-40" @change="reload" />
            <input v-model="filters.start_time" type="date" class="input w-44" @change="reload" />
            <input v-model="filters.end_time" type="date" class="input w-44" @change="reload" />
            <button class="btn btn-secondary" @click="resetFilters">重置</button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="orders" :loading="loading" row-key="id">
            <template #cell-select="{ row }">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600"
                :checked="selectedOrderIds.includes(row.id)"
                :disabled="!isSelectable(row)"
                @change="toggleSelection(row.id)"
              />
            </template>
            <template #cell-amount_cny_fen="{ value, row }">
              <div>
                <span class="font-medium text-gray-900 dark:text-white">¥{{ formatFen(value) }}</span>
                <p v-if="row.bonus_amount_cny_fen > 0" class="mt-1 text-xs text-emerald-600 dark:text-emerald-400">
                  到账 ⚡{{ formatFen(row.credited_amount_cny_fen) }}（赠送 {{ formatFen(row.bonus_amount_cny_fen) }}）
                </p>
              </div>
            </template>
            <template #cell-status="{ value }">
              <span class="badge" :class="statusBadgeClass(value)">{{ statusLabel(value) }}</span>
            </template>
            <template #cell-invoice_status="{ value }">
              <span class="badge" :class="invoiceStatusBadgeClass(value)">{{ invoiceStatusLabel(value) }}</span>
            </template>
            <template #cell-created_at="{ value }">
              {{ formatDateTime(value) }}
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="listData.total > 0"
            :page="listData.page"
            :total="listData.total"
            :page-size="listData.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </div>

    <Teleport to="body">
      <div v-if="invoiceManagementEnabled && showApplyDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="showApplyDialog = false"></div>
        <div class="relative z-10 w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">提交开票申请</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                本次共选择 {{ selectedOrderIds.length }} 笔订单，金额合计 ¥{{ formatFen(selectedAmountFen) }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                最低开票金额 ¥{{ formatFen(invoiceMinimumAmountFen) }}
              </p>
            </div>
            <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="showApplyDialog = false">✕</button>
          </div>

          <div class="mt-5 space-y-4">
            <div>
              <label class="input-label">发票抬头</label>
              <Select v-model="applyForm.profile_id" :options="profileOptions" />
              <p v-if="profiles.length === 0" class="mt-2 text-sm text-amber-600 dark:text-amber-400">
                还没有发票抬头，请先到“发票管理”页创建。
              </p>
            </div>
            <div>
              <label class="input-label">备注</label>
              <textarea v-model="applyForm.remark" rows="3" class="input min-h-[96px]"></textarea>
            </div>
          </div>

          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="showApplyDialog = false">取消</button>
            <button class="btn btn-primary" :disabled="submitting || !canConfirmInvoiceRequest" @click="submitInvoiceRequest">
              {{ submitting ? '提交中...' : '确认提交' }}
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
import { invoiceAPI } from '@/api/invoice'
import { useAppStore } from '@/stores'
import type { InvoiceProfile, TopupOrderInvoiceStatus, TopupOrderListItem, TopupOrderListResponse, TopupOrderPaymentStatus } from '@/types'
import { formatDateTime } from '@/utils/format'

const appStore = useAppStore()
const invoiceManagementEnabled = computed(() => appStore.cachedPublicSettings?.invoice_management_enabled === true)

const loading = ref(false)
const submitting = ref(false)
const showApplyDialog = ref(false)
const orders = ref<TopupOrderListItem[]>([])
const profiles = ref<InvoiceProfile[]>([])
const selectedOrderIds = ref<number[]>([])
const invoiceMinimumAmountFen = 30000
const listData = reactive<TopupOrderListResponse>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
  selectable_amount_fen: 0,
})

const filters = reactive({
  status: '',
  invoice_status: '',
  start_time: '',
  end_time: '',
})

const applyForm = reactive({
  profile_id: 0,
  remark: '',
})

const columns = computed<Column[]>(() => [
  ...(invoiceManagementEnabled.value ? [{ key: 'select', label: '' }] : []),
  { key: 'order_no', label: '订单号' },
  { key: 'amount_cny_fen', label: '金额' },
  { key: 'pay_type', label: '支付方式' },
  { key: 'status', label: '订单状态' },
  ...(invoiceManagementEnabled.value ? [{ key: 'invoice_status', label: '开票状态' }] : []),
  { key: 'created_at', label: '创建时间' },
])

const statusOptions = [
  { value: '', label: '全部订单状态' },
  { value: 'pending', label: '待支付' },
  { value: 'completed', label: '已完成' },
  { value: 'expired', label: '已过期' },
]

const invoiceStatusOptions = [
  { value: '', label: '全部开票状态' },
  { value: 'none', label: '未申请' },
  { value: 'applied', label: '申请中' },
  { value: 'invoiced', label: '已开票' },
]

const profileOptions = computed(() =>
  profiles.value.map((profile) => ({
    value: profile.id,
    label: `${profile.title}${profile.is_default ? '（默认）' : ''}`,
  })),
)

const selectedAmountFen = computed(() =>
  orders.value
    .filter((order) => selectedOrderIds.value.includes(order.id))
    .reduce((sum, order) => sum + order.amount_cny_fen, 0),
)

const selectedAmountShortfallFen = computed(() => Math.max(invoiceMinimumAmountFen - selectedAmountFen.value, 0))
const canSubmitInvoiceRequest = computed(() => invoiceManagementEnabled.value && selectedOrderIds.value.length > 0 && selectedAmountFen.value >= invoiceMinimumAmountFen)
const canConfirmInvoiceRequest = computed(() => Boolean(applyForm.profile_id) && profiles.value.length > 0 && canSubmitInvoiceRequest.value)

const formatFen = (value: number) => (value / 100).toFixed(2)

const showInvoiceAmountMinimumError = () => {
  appStore.showError(`开票金额需满 ¥${formatFen(invoiceMinimumAmountFen)}，当前还差 ¥${formatFen(selectedAmountShortfallFen.value)}`)
}

const statusLabel = (status: TopupOrderPaymentStatus) =>
  ({
    pending: '待支付',
    completed: '已完成',
    expired: '已过期',
  })[status] || status

const invoiceStatusLabel = (status: TopupOrderInvoiceStatus) =>
  ({
    none: '未申请',
    applied: '申请中',
    invoiced: '已开票',
  })[status] || status

const statusBadgeClass = (status: TopupOrderPaymentStatus) =>
  status === 'completed' ? 'badge-success' : status === 'pending' ? 'badge-warning' : 'badge-gray'

const invoiceStatusBadgeClass = (status: TopupOrderInvoiceStatus) =>
  status === 'none' ? 'badge-success' : status === 'applied' ? 'badge-warning' : 'badge-gray'

const isSelectable = (order: TopupOrderListItem) =>
  invoiceManagementEnabled.value && order.status === 'completed' && order.invoice_status === 'none'

const toggleSelection = (orderId: number) => {
  if (selectedOrderIds.value.includes(orderId)) {
    selectedOrderIds.value = selectedOrderIds.value.filter((id) => id !== orderId)
    return
  }
  selectedOrderIds.value = [...selectedOrderIds.value, orderId]
}

const loadProfiles = async () => {
  if (!invoiceManagementEnabled.value) {
    profiles.value = []
    applyForm.profile_id = 0
    return
  }
  profiles.value = await invoiceAPI.listInvoiceProfiles()
  if (!applyForm.profile_id && profiles.value.length > 0) {
    applyForm.profile_id = profiles.value.find((profile) => profile.is_default)?.id || profiles.value[0].id
  }
}

const loadOrders = async () => {
  loading.value = true
  try {
    const data = await invoiceAPI.listMyTopupOrders({
      page: listData.page,
      page_size: listData.page_size,
      status: (filters.status as TopupOrderPaymentStatus | '') || '',
      invoice_status: (filters.invoice_status as TopupOrderInvoiceStatus | '') || '',
      start_time: filters.start_time || undefined,
      end_time: filters.end_time || undefined,
    })
    orders.value = data.items
    Object.assign(listData, data)
    selectedOrderIds.value = selectedOrderIds.value.filter((id) => data.items.some((item) => item.id === id && isSelectable(item)))
  } catch (error: any) {
    appStore.showError(error.message || '加载充值订单失败')
  } finally {
    loading.value = false
  }
}

const reload = async () => {
  listData.page = 1
  await loadOrders()
}

const resetFilters = async () => {
  filters.status = ''
  filters.invoice_status = ''
  filters.start_time = ''
  filters.end_time = ''
  await reload()
}

const handlePageChange = async (page: number) => {
  listData.page = page
  await loadOrders()
}

const handlePageSizeChange = async (pageSize: number) => {
  listData.page_size = pageSize
  listData.page = 1
  await loadOrders()
}

const openApplyDialog = async () => {
  if (selectedOrderIds.value.length === 0) {
    return
  }
  if (!canSubmitInvoiceRequest.value) {
    showInvoiceAmountMinimumError()
    return
  }
  try {
    if (profiles.value.length === 0) {
      await loadProfiles()
    }
    showApplyDialog.value = true
  } catch (error: any) {
    appStore.showError(error.message || '加载发票抬头失败')
  }
}

const submitInvoiceRequest = async () => {
  if (!canSubmitInvoiceRequest.value) {
    showInvoiceAmountMinimumError()
    return
  }
  if (!applyForm.profile_id) return
  submitting.value = true
  try {
    await invoiceAPI.createInvoiceRequest({
      profile_id: applyForm.profile_id,
      order_ids: selectedOrderIds.value,
      remark: applyForm.remark || undefined,
    })
    appStore.showSuccess('开票申请已提交')
    selectedOrderIds.value = []
    applyForm.remark = ''
    showApplyDialog.value = false
    await loadOrders()
  } catch (error: any) {
    appStore.showError(error.message || '提交开票申请失败')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  await appStore.fetchPublicSettings()
  await Promise.all([
    loadOrders(),
    invoiceManagementEnabled.value ? loadProfiles() : Promise.resolve(),
  ])
})
</script>
