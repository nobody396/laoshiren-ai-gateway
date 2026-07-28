<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-4">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <select v-model="typeFilter" class="input w-auto text-sm">
            <option value="">{{ t('agent.allTypes') }}</option>
            <option value="consumption">{{ t('agent.typeConsumption') }}</option>
            <option value="first_recharge_invitee">{{ t('agent.typeFirstRechargeInvitee') }}</option>
            <option value="first_recharge_referral">{{ t('agent.typeFirstRechargeReferral') }}</option>
          </select>
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
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('agent.commissionRecords') }}</h3>
        </div>

        <div v-if="loading" class="divide-y divide-gray-50 dark:divide-dark-800">
          <div v-for="i in 5" :key="i" class="flex gap-4 px-6 py-4 animate-pulse">
            <div class="h-4 w-32 rounded bg-gray-100 dark:bg-dark-700"></div>
            <div class="h-4 w-24 rounded bg-gray-100 dark:bg-dark-700 ml-auto"></div>
          </div>
        </div>

        <div v-else-if="records.length > 0" class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-800">
            <thead class="bg-gray-50 dark:bg-dark-800/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.commissionType') }}</th>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.triggeredBy') }}</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.sourceAmount') }}</th>
                <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.commissionAmount') }}</th>
                <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">{{ t('agent.commissionTime') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-800">
              <tr v-for="record in records" :key="record.id" class="hover:bg-gray-50 dark:hover:bg-dark-800/30 transition-colors">
                <td class="px-6 py-4">
                  <span :class="typeClass(record.type)" class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium">
                    {{ t(`agent.type_${record.type}`) }}
                  </span>
                </td>
                <td class="px-6 py-4 text-sm text-gray-500 dark:text-dark-400">
                  <div class="font-medium text-gray-800 dark:text-dark-200">{{ getTriggerDisplayName(record) }}</div>
                  <div v-if="getTriggerSecondaryText(record)" class="text-xs text-gray-400">{{ getTriggerSecondaryText(record) }}</div>
                </td>
                <td class="px-6 py-4 text-right text-sm text-gray-700 dark:text-dark-300">⚡{{ record.source_amount.toFixed(4) }}</td>
                <td class="px-6 py-4 text-right text-sm font-semibold text-green-600 dark:text-green-400">+¥{{ record.amount.toFixed(4) }}</td>
                <td class="px-6 py-4 text-sm text-gray-500 dark:text-dark-400">{{ formatDate(record.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="flex flex-col items-center justify-center py-12 text-gray-400">
          <svg class="mb-3 h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
          </svg>
          <p class="text-sm">{{ t('agent.noCommissions') }}</p>
        </div>

        <div v-if="pagination && pagination.total_pages > 1" class="flex items-center justify-between border-t border-gray-100 dark:border-dark-800 px-6 py-4">
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ (pagination.page - 1) * pagination.page_size + 1 }}–{{ Math.min(pagination.page * pagination.page_size, pagination.total) }} / {{ pagination.total }}
          </p>
          <div class="flex gap-2">
            <button :disabled="pagination.page <= 1" @click="changePage(pagination.page - 1)" class="btn btn-secondary btn-sm">{{ t('common.prev') }}</button>
            <button :disabled="pagination.page >= pagination.total_pages" @click="changePage(pagination.page + 1)" class="btn btn-secondary btn-sm">{{ t('common.next') }}</button>
          </div>
        </div>
      </div>

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
import { getAgentCommissions, type CommissionRecord, type PaginationResult } from '@/api/agent'
import { buildAuthErrorMessage } from '@/utils/authError'

const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const records = ref<CommissionRecord[]>([])
const pagination = ref<PaginationResult | null>(null)
const currentPage = ref(1)
const typeFilter = ref('')
const refreshTimer = ref<number | null>(null)

function getDefaultDates() {
  const now = new Date()
  const end = now.toISOString().slice(0, 10)
  const start = new Date(now.setDate(now.getDate() - 30)).toISOString().slice(0, 10)
  return { start, end }
}

const { start: defaultStart, end: defaultEnd } = getDefaultDates()
const startDate = ref(defaultStart)
const endDate = ref(defaultEnd)

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function typeClass(type: string): string {
  switch (type) {
    case 'consumption':
    case 'consumption_commission':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'first_recharge_invitee':
    case 'first_recharge_invitee_bonus':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
    case 'first_recharge_friend_invitee':
    case 'first_recharge_friend_invitee_bonus':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
    case 'first_recharge_referral':
    case 'first_recharge_referral_bonus':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-300'
  }
}

function getTextValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function getTriggerDisplayName(record: CommissionRecord): string {
  const username = getTextValue(record.username)
  const email = getTextValue(record.user_email)
  return username || email || `#${record.user_id}`
}

function getTriggerSecondaryText(record: CommissionRecord): string {
  const username = getTextValue(record.username)
  const email = getTextValue(record.user_email)
  return username && email ? email : ''
}

async function fetchData(options: { silent?: boolean } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  error.value = ''
  try {
    const params: Record<string, string | number> = { page: currentPage.value, page_size: 20 }
    if (typeFilter.value) params.type = typeFilter.value
    if (startDate.value) params.start = startDate.value
    if (endDate.value) params.end = endDate.value
    const res = await getAgentCommissions(params)
    records.value = res.items ?? []
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
  typeFilter.value = ''
  startDate.value = start
  endDate.value = end
  currentPage.value = 1
  fetchData()
}

onMounted(() => {
  fetchData()
  refreshTimer.value = window.setInterval(() => {
    fetchData({ silent: true })
  }, 5000)
})

onBeforeUnmount(() => {
  if (refreshTimer.value !== null) {
    window.clearInterval(refreshTimer.value)
    refreshTimer.value = null
  }
})
</script>
