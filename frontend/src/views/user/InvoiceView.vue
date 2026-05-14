<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card p-6">
        <div class="flex flex-wrap items-center gap-3">
          <button class="btn" :class="activeTab === 'requests' ? 'btn-primary' : 'btn-secondary'" @click="activeTab = 'requests'">
            开票记录
          </button>
          <button class="btn" :class="activeTab === 'profiles' ? 'btn-primary' : 'btn-secondary'" @click="activeTab = 'profiles'">
            发票抬头
          </button>
        </div>
      </div>

      <div v-if="activeTab === 'requests'" class="space-y-4">
        <div class="card p-6">
          <div class="flex flex-wrap items-center gap-3">
            <Select v-model="requestFilters.status" :options="requestStatusOptions" class="w-40" @change="reloadRequests" />
            <input v-model="requestFilters.start_time" type="date" class="input w-44" @change="reloadRequests" />
            <input v-model="requestFilters.end_time" type="date" class="input w-44" @change="reloadRequests" />
            <button class="btn btn-secondary" @click="resetRequestFilters">重置</button>
          </div>
        </div>

        <div class="space-y-4">
          <div v-for="request in requests.items" :key="request.id" class="card p-6">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div class="flex items-center gap-3">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ request.serial_no }}</h2>
                  <span class="badge" :class="requestStatusBadgeClass(request.status)">{{ requestStatusLabel(request.status) }}</span>
                </div>
                <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                  {{ request.profile_snapshot.title }} · ¥{{ formatFen(request.total_amount_fen) }} · {{ formatDateTime(request.created_at) }}
                </p>
                <p v-if="request.reject_reason" class="mt-2 text-sm text-red-600 dark:text-red-400">
                  驳回原因：{{ request.reject_reason }}
                </p>
              </div>
            </div>

            <div class="mt-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
              <p class="text-sm font-medium text-gray-700 dark:text-dark-200">关联订单</p>
              <div class="mt-2 flex flex-wrap gap-2">
                <span v-for="order in request.orders" :key="order.id" class="rounded-full bg-white px-3 py-1 text-sm text-gray-700 shadow-sm dark:bg-dark-700 dark:text-dark-100">
                  {{ order.topup_order.order_no }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <Pagination
          v-if="requests.total > 0"
          :page="requests.page"
          :total="requests.total"
          :page-size="requests.page_size"
          @update:page="handleRequestPageChange"
          @update:pageSize="handleRequestPageSizeChange"
        />
      </div>

      <div v-else class="space-y-4">
        <div class="card p-6">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">发票抬头</h1>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">编辑抬头不会影响已提交申请中的历史快照。</p>
            </div>
            <button class="btn btn-primary" @click="openProfileDialog()">新增抬头</button>
          </div>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <div v-for="profile in profiles" :key="profile.id" class="card p-6">
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="flex items-center gap-3">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ profile.title }}</h2>
                  <span v-if="profile.is_default" class="badge badge-success">默认</span>
                </div>
                <div class="mt-3 space-y-1 text-sm text-gray-600 dark:text-dark-300">
                  <p>税号：{{ profile.tax_number }}</p>
                  <p>邮箱：{{ profile.email }}</p>
                  <p v-if="profile.address">地址：{{ profile.address }}</p>
                  <p v-if="profile.phone">电话：{{ profile.phone }}</p>
                  <p v-if="profile.bank_name">开户行：{{ profile.bank_name }}</p>
                  <p v-if="profile.bank_account">银行账号：{{ profile.bank_account }}</p>
                </div>
              </div>
              <div class="flex flex-col gap-2">
                <button class="btn btn-secondary" @click="openProfileDialog(profile)">编辑</button>
                <button v-if="!profile.is_default" class="btn btn-secondary" @click="setDefault(profile.id)">设为默认</button>
                <button class="btn btn-secondary text-red-600" @click="openDeleteProfileDialog(profile)">删除</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="showDeleteProfileDialog"
      title="删除发票抬头"
      :message="deleteProfileTarget ? `确定要删除发票抬头“${deleteProfileTarget.title}”吗？此操作无法撤销。` : '确定要删除这个发票抬头吗？此操作无法撤销。'"
      confirm-text="删除"
      cancel-text="取消"
      :danger="true"
      @confirm="confirmRemoveProfile"
      @cancel="closeDeleteProfileDialog"
    />

    <Teleport to="body">
      <div v-if="showProfileDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="showProfileDialog = false"></div>
        <div class="relative z-10 w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <div class="flex items-start justify-between gap-4">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingProfileId ? '编辑抬头' : '新增抬头' }}</h2>
            <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="showProfileDialog = false">✕</button>
          </div>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="md:col-span-2">
              <label class="input-label">抬头名称</label>
              <input v-model="profileForm.title" class="input" />
            </div>
            <div>
              <label class="input-label">纳税人识别号</label>
              <input v-model="profileForm.tax_number" class="input" />
            </div>
            <div>
              <label class="input-label">邮箱</label>
              <input v-model="profileForm.email" class="input" />
            </div>
            <div>
              <label class="input-label">地址</label>
              <input v-model="profileForm.address" class="input" />
            </div>
            <div>
              <label class="input-label">电话</label>
              <input v-model="profileForm.phone" class="input" />
            </div>
            <div>
              <label class="input-label">开户银行</label>
              <input v-model="profileForm.bank_name" class="input" />
            </div>
            <div>
              <label class="input-label">银行账号</label>
              <input v-model="profileForm.bank_account" class="input" />
            </div>
          </div>

          <div class="mt-6 flex justify-end gap-3">
            <button class="btn btn-secondary" @click="showProfileDialog = false">取消</button>
            <button class="btn btn-primary" :disabled="savingProfile" @click="submitProfile">
              {{ savingProfile ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { invoiceAPI } from '@/api/invoice'
import { useAppStore } from '@/stores'
import type { InvoiceProfile, InvoiceRequestListResponse, InvoiceRequestStatus } from '@/types'
import { formatDateTime } from '@/utils/format'

const appStore = useAppStore()

const activeTab = ref<'requests' | 'profiles'>('requests')
const profiles = ref<InvoiceProfile[]>([])
const requests = reactive<InvoiceRequestListResponse>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1,
})

const requestFilters = reactive({
  status: '',
  start_time: '',
  end_time: '',
})

const requestStatusOptions = [
  { value: '', label: '全部申请状态' },
  { value: 'pending', label: '待开票' },
  { value: 'exported', label: '已导出' },
  { value: 'completed', label: '已开票' },
  { value: 'rejected', label: '已驳回' },
]

const showProfileDialog = ref(false)
const showDeleteProfileDialog = ref(false)
const savingProfile = ref(false)
const editingProfileId = ref<number | null>(null)
const deleteProfileTarget = ref<InvoiceProfile | null>(null)
const profileForm = reactive({
  title: '',
  tax_number: '',
  email: '',
  address: '',
  phone: '',
  bank_name: '',
  bank_account: '',
})

const formatFen = (value: number) => (value / 100).toFixed(2)

const requestStatusLabel = (status: InvoiceRequestStatus) =>
  ({
    pending: '待开票',
    exported: '已导出',
    completed: '已开票',
    rejected: '已驳回',
  })[status] || status

const requestStatusBadgeClass = (status: InvoiceRequestStatus) =>
  status === 'completed' ? 'badge-success' : status === 'rejected' ? 'badge-danger' : 'badge-warning'

const loadProfiles = async () => {
  profiles.value = await invoiceAPI.listInvoiceProfiles()
}

const loadRequests = async () => {
  const data = await invoiceAPI.listInvoiceRequests({
    page: requests.page,
    page_size: requests.page_size,
    status: requestFilters.status || undefined,
    start_time: requestFilters.start_time || undefined,
    end_time: requestFilters.end_time || undefined,
  })
  Object.assign(requests, data)
}

const reloadRequests = async () => {
  requests.page = 1
  try {
    await loadRequests()
  } catch (error: any) {
    appStore.showError(error.message || '加载开票记录失败')
  }
}

const resetRequestFilters = async () => {
  requestFilters.status = ''
  requestFilters.start_time = ''
  requestFilters.end_time = ''
  await reloadRequests()
}

const handleRequestPageChange = async (page: number) => {
  requests.page = page
  await loadRequests()
}

const handleRequestPageSizeChange = async (pageSize: number) => {
  requests.page_size = pageSize
  requests.page = 1
  await loadRequests()
}

const resetProfileForm = () => {
  profileForm.title = ''
  profileForm.tax_number = ''
  profileForm.email = ''
  profileForm.address = ''
  profileForm.phone = ''
  profileForm.bank_name = ''
  profileForm.bank_account = ''
}

const openProfileDialog = (profile?: InvoiceProfile) => {
  editingProfileId.value = profile?.id || null
  profileForm.title = profile?.title || ''
  profileForm.tax_number = profile?.tax_number || ''
  profileForm.email = profile?.email || ''
  profileForm.address = profile?.address || ''
  profileForm.phone = profile?.phone || ''
  profileForm.bank_name = profile?.bank_name || ''
  profileForm.bank_account = profile?.bank_account || ''
  showProfileDialog.value = true
}

const submitProfile = async () => {
  savingProfile.value = true
  try {
    const payload = {
      title: profileForm.title,
      tax_number: profileForm.tax_number,
      email: profileForm.email,
      address: profileForm.address || null,
      phone: profileForm.phone || null,
      bank_name: profileForm.bank_name || null,
      bank_account: profileForm.bank_account || null,
    }
    if (editingProfileId.value) {
      await invoiceAPI.updateInvoiceProfile(editingProfileId.value, payload)
      appStore.showSuccess('抬头已更新')
    } else {
      await invoiceAPI.createInvoiceProfile(payload)
      appStore.showSuccess('抬头已创建')
    }
    showProfileDialog.value = false
    resetProfileForm()
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error.message || '保存抬头失败')
  } finally {
    savingProfile.value = false
  }
}

const setDefault = async (id: number) => {
  try {
    await invoiceAPI.setDefaultInvoiceProfile(id)
    appStore.showSuccess('默认抬头已更新')
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error.message || '设置默认抬头失败')
  }
}

const openDeleteProfileDialog = (profile: InvoiceProfile) => {
  deleteProfileTarget.value = profile
  showDeleteProfileDialog.value = true
}

const closeDeleteProfileDialog = () => {
  showDeleteProfileDialog.value = false
  deleteProfileTarget.value = null
}

const confirmRemoveProfile = async () => {
  if (!deleteProfileTarget.value) return

  try {
    await invoiceAPI.deleteInvoiceProfile(deleteProfileTarget.value.id)
    appStore.showSuccess('抬头已删除')
    closeDeleteProfileDialog()
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error.message || '删除抬头失败')
  }
}

onMounted(async () => {
  try {
    await Promise.all([loadProfiles(), loadRequests()])
  } catch (error: any) {
    appStore.showError(error.message || '加载发票管理数据失败')
  }
})
</script>
