<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-4">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-300">{{ t('agent.dateRange') }}</span>
          <input v-model="startDate" type="date" class="input w-auto text-sm" :max="endDate || undefined" />
          <span class="text-gray-400">—</span>
          <input v-model="endDate" type="date" class="input w-auto text-sm" :min="startDate || undefined" />
          <button @click="fetchData()" class="btn btn-primary btn-sm">{{ t('common.refresh') }}</button>
          <button @click="clearFilters" class="btn btn-secondary btn-sm">{{ t('common.reset') }}</button>
        </div>
      </div>

      <!-- Table -->
      <div class="card overflow-hidden">
        <div class="border-b border-gray-100 dark:border-dark-800 px-6 py-4">
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('agent.invitedUsers') }}</h3>
          <p class="text-sm text-gray-500 dark:text-dark-400 mt-0.5">{{ t('agent.invitedUsersDesc') }}</p>
        </div>

        <!-- Loading skeleton -->
        <div v-if="loading" class="divide-y divide-gray-50 dark:divide-dark-800">
          <div v-for="i in 5" :key="i" class="flex gap-4 px-6 py-4 animate-pulse">
            <div class="h-4 w-40 rounded bg-gray-100 dark:bg-dark-700"></div>
            <div class="h-4 w-24 rounded bg-gray-100 dark:bg-dark-700"></div>
            <div class="h-4 w-24 rounded bg-gray-100 dark:bg-dark-700 ml-auto"></div>
          </div>
        </div>

        <!-- Data table -->
        <div v-else-if="users.length > 0" class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.userLabel') }}</th>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.joinedAt') }}</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.totalRecharge') }}</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.totalConsumption') }}</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.totalCommission') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="user in users" :key="user.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-800/30 transition-colors">
                <td class="px-6 py-4">
                  <div class="text-sm font-medium text-gray-900 dark:text-white">{{ getDisplayName(user) }}</div>
                  <div v-if="getSecondaryText(user)" class="text-xs text-gray-400">{{ getSecondaryText(user) }}</div>
                </td>
                <td class="px-6 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDate(user.joined_at) }}</td>
                <td class="px-6 py-4 text-right text-sm font-medium text-gray-900 dark:text-white">¥{{ user.total_recharge.toFixed(4) }}</td>
                <td class="px-6 py-4 text-right text-sm font-medium text-gray-900 dark:text-white">¥{{ user.total_consumption.toFixed(2) }}</td>
                <td class="px-6 py-4 text-right text-sm font-medium text-green-600 dark:text-green-400">¥{{ user.total_commission.toFixed(2) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Empty state -->
        <div v-else class="flex flex-col items-center justify-center py-12 text-gray-400">
          <svg class="mb-3 h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
          </svg>
          <p class="text-sm">{{ t('agent.noInvitedUsers') }}</p>
        </div>

        <!-- Pagination -->
        <div v-if="pagination && pagination.total_pages > 1" class="flex items-center justify-between border-t border-gray-100 dark:border-dark-800 px-6 py-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.showing') }} {{ (pagination.page - 1) * pagination.page_size + 1 }}–{{ Math.min(pagination.page * pagination.page_size, pagination.total) }} / {{ pagination.total }}
          </p>
          <div class="flex gap-2">
            <button :disabled="pagination.page <= 1" @click="changePage(pagination.page - 1)" class="btn btn-secondary btn-sm">{{ t('common.prev') }}</button>
            <button :disabled="pagination.page >= pagination.total_pages" @click="changePage(pagination.page + 1)" class="btn btn-secondary btn-sm">{{ t('common.next') }}</button>
          </div>
        </div>
      </div>

      <!-- Error -->
      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20">
        <p class="text-sm text-red-700 dark:text-red-400">{{ error }}</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getAgentInvitedUsers, type InvitedUserStat, type PaginationResult } from '@/api/agent'
import { buildAuthErrorMessage } from '@/utils/authError'

const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const users = ref<InvitedUserStat[]>([])
const pagination = ref<PaginationResult | null>(null)
const currentPage = ref(1)
const refreshTimer = ref<number | null>(null)

function getDefaultDates() {
  const now = new Date()
  const end = now.toISOString().slice(0, 10)
  const start = new Date(now.setDate(now.getDate() - 30)).toISOString().slice(0, 10)
  return { start, end }
}

const startDate = ref('')
const endDate = ref('')

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

function getTextValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function getDisplayName(user: InvitedUserStat): string {
  const username = getTextValue(user.username)
  const email = getTextValue(user.email)
  return username || email || `#${user.user_id}`
}

function getSecondaryText(user: InvitedUserStat): string {
  const username = getTextValue(user.username)
  const email = getTextValue(user.email)
  return username && email ? email : ''
}

async function fetchData(options: { silent?: boolean } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  error.value = ''
  try {
    const params: Record<string, string | number> = { page: currentPage.value, page_size: 20 }
    if (startDate.value) params.start = startDate.value
    if (endDate.value) params.end = endDate.value
    const res = await getAgentInvitedUsers(params)
    users.value = res.items ?? []
    pagination.value = res.pagination ?? null
  } catch (e: unknown) {
    error.value = buildAuthErrorMessage(e, { fallback: t('common.error') })
  } finally {
    if (!options.silent) {
      loading.value = false
    }
  }
}

function changePage(page: number) {
  currentPage.value = page
  fetchData()
}

function clearFilters() {
  const { start, end } = getDefaultDates()
  startDate.value = start
  endDate.value = end
  currentPage.value = 1
  fetchData()
}

onMounted(() => {
  fetchData()
  refreshTimer.value = window.setInterval(() => {
    if (!document.hidden) fetchData({ silent: true })
  }, 5000)
})

onBeforeUnmount(() => {
  if (refreshTimer.value !== null) {
    window.clearInterval(refreshTimer.value)
    refreshTimer.value = null
  }
})
</script>
