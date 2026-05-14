<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">订单管理</h1>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">查看全部充值订单并按支付/开票状态筛选。</p>
          </div>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model="filters.search" class="input min-w-[220px]" placeholder="搜索用户/订单号" @input="handleSearch" />
            <Select v-model="filters.status" :options="statusOptions" class="w-40" @change="reload" />
            <Select v-model="filters.invoice_status" :options="invoiceStatusOptions" class="w-40" @change="reload" />
            <input v-model="filters.start_time" type="date" class="input w-44" @change="reload" />
            <input v-model="filters.end_time" type="date" class="input w-44" @change="reload" />
            <button class="btn btn-secondary" @click="resetFilters">重置</button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="orders" :loading="loading" row-key="id">
            <template #cell-user_email="{ row }">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ row.user_email || '-' }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ row.user_name || `ID ${row.user_id}` }}</p>
              </div>
            </template>
            <template #cell-amount_cny_fen="{ value }">
              <span class="font-medium text-gray-900 dark:text-white">¥{{ formatFen(value) }}</span>
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
import type { TopupOrderInvoiceStatus, TopupOrderListItem, TopupOrderListResponse, TopupOrderPaymentStatus } from '@/types'
import { formatDateTime } from '@/utils/format'

const appStore = useAppStore()
const loading = ref(false)
const orders = ref<TopupOrderListItem[]>([])
const listData = reactive<TopupOrderListResponse>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
})

const filters = reactive({
  search: '',
  status: '',
  invoice_status: '',
  start_time: '',
  end_time: '',
})

const columns = computed<Column[]>(() => [
  { key: 'order_no', label: '订单号' },
  { key: 'user_email', label: '用户' },
  { key: 'amount_cny_fen', label: '金额' },
  { key: 'pay_type', label: '支付方式' },
  { key: 'status', label: '订单状态' },
  { key: 'invoice_status', label: '开票状态' },
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

const formatFen = (value: number) => (value / 100).toFixed(2)

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

const loadOrders = async () => {
  loading.value = true
  try {
    const data = await adminAPI.invoice.listTopupOrders({
      page: listData.page,
      page_size: listData.page_size,
      search: filters.search || undefined,
      status: (filters.status as TopupOrderPaymentStatus | '') || '',
      invoice_status: (filters.invoice_status as TopupOrderInvoiceStatus | '') || '',
      start_time: filters.start_time || undefined,
      end_time: filters.end_time || undefined,
    })
    orders.value = data.items
    Object.assign(listData, data)
  } catch (error: any) {
    appStore.showError(error.message || '加载订单失败')
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
  listData.page = 1
  await loadOrders()
}

const resetFilters = async () => {
  filters.search = ''
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

onMounted(async () => {
  await loadOrders()
})
</script>
