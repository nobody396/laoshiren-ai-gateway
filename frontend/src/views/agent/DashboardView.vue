<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <!-- Date Range Filter -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-300">{{ t('agent.dateRange') }}</span>
          <input
            v-model="startDate"
            type="date"
            class="input w-auto text-sm"
            :max="endDate || undefined"
          />
          <span class="text-gray-400">—</span>
          <input
            v-model="endDate"
            type="date"
            class="input w-auto text-sm"
            :min="startDate || undefined"
          />
          <button @click="fetchDashboard" class="btn btn-primary btn-sm">
            {{ t('common.refresh') }}
          </button>
          <button @click="clearDateRange" class="btn btn-secondary btn-sm">
            {{ t('common.reset') }}
          </button>
        </div>
      </div>

      <!-- Stats Cards -->
      <div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div v-for="i in 6" :key="i" class="card h-24 animate-pulse bg-gray-100 dark:bg-dark-800"></div>
      </div>
      <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.totalCommission') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">${{ dashboard?.total_commission.toFixed(4) ?? '0.0000' }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.settledCommission') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">${{ dashboard?.settled_commission.toFixed(4) ?? '0.0000' }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.unsettledCommission') }}</p>
          <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">${{ dashboard?.unsettled_commission.toFixed(4) ?? '0.0000' }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.thisMonthCommission') }}</p>
          <p class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">${{ dashboard?.this_month_commission.toFixed(4) ?? '0.0000' }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.periodCommission') }}</p>
          <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">${{ dashboard?.period_commission.toFixed(4) ?? '0.0000' }}</p>
          <p class="mt-1 text-xs text-gray-400">{{ startDate && endDate ? `${startDate} ~ ${endDate}` : t('agent.allTime') }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.invitedUsers') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ dashboard?.invited_user_count ?? 0 }}</p>
        </div>
        <div class="card p-5">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.currentRate') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatRate(dashboard?.consumption_rate) }}</p>
        </div>
      </div>

      <!-- Invite Code Card -->
      <div class="card p-6">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-3">{{ t('agent.myInviteCode') }}</h3>
        <div v-if="inviteCodeLoading" class="h-10 w-64 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
        <div v-else class="flex items-center gap-3">
          <code class="rounded-lg bg-gray-100 dark:bg-dark-800 px-4 py-2 text-lg font-mono font-bold tracking-widest text-primary-600 dark:text-primary-400">
            {{ inviteCode || '—' }}
          </code>
          <button
            v-if="inviteCode"
            @click="copyInviteCode"
            class="btn btn-secondary btn-sm"
          >
            {{ copied ? t('common.copied') : t('common.copy') }}
          </button>
        </div>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          {{ t('agent.inviteCodeHint') }}
          <span v-if="registerUrl" class="ml-1 font-mono text-xs text-gray-600 dark:text-dark-300">{{ registerUrl }}</span>
        </p>
      </div>

      <!-- Quick Links -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <router-link to="/agent/users" class="card p-5 hover:border-primary-300 dark:hover:border-primary-700 transition-colors">
          <h4 class="font-semibold text-gray-900 dark:text-white">{{ t('agent.viewInvitedUsers') }}</h4>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('agent.viewInvitedUsersDesc') }}</p>
        </router-link>
        <router-link to="/agent/commissions" class="card p-5 hover:border-primary-300 dark:hover:border-primary-700 transition-colors">
          <h4 class="font-semibold text-gray-900 dark:text-white">{{ t('agent.viewCommissions') }}</h4>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('agent.viewCommissionsDesc') }}</p>
        </router-link>
      </div>

      <!-- Error -->
      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20">
        <p class="text-sm text-red-700 dark:text-red-400">{{ error }}</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getAgentDashboard, getAgentInviteCode, type AgentDashboard } from '@/api/agent'
import { buildAuthErrorMessage } from '@/utils/authError'
import { useClipboard } from '@/composables/useClipboard'

const { t } = useI18n()

const loading = ref(true)
const inviteCodeLoading = ref(true)
const error = ref('')
const dashboard = ref<AgentDashboard | null>(null)
const inviteCode = ref('')
const { copied, copyToClipboard } = useClipboard()
const startDate = ref('')
const endDate = ref('')

function getDefaultDates() {
  const now = new Date()
  const end = now.toISOString().slice(0, 10)
  const start = new Date(now.setDate(now.getDate() - 30)).toISOString().slice(0, 10)
  return { start, end }
}

const registerUrl = computed(() => {
  if (!inviteCode.value) return ''
  return `${window.location.origin}/register?ref=${inviteCode.value}`
})

async function fetchDashboard() {
  loading.value = true
  error.value = ''
  try {
    const params: { start?: string; end?: string } = {}
    if (startDate.value) params.start = startDate.value
    if (endDate.value) params.end = endDate.value
    dashboard.value = await getAgentDashboard(params)
  } catch (e: unknown) {
    error.value = buildAuthErrorMessage(e, { fallback: t('common.error') })
  } finally {
    loading.value = false
  }
}

async function fetchInviteCode() {
  inviteCodeLoading.value = true
  try {
    const res = await getAgentInviteCode()
    inviteCode.value = res.invite_code
  } catch (e) {
    console.error('Failed to fetch invite code', e)
  } finally {
    inviteCodeLoading.value = false
  }
}

async function copyInviteCode() {
  if (!inviteCode.value) return
  await copyToClipboard(registerUrl.value || inviteCode.value, t('common.copiedToClipboard'))
}

function clearDateRange() {
  const { start, end } = getDefaultDates()
  startDate.value = start
  endDate.value = end
  fetchDashboard()
}

function formatRate(value?: number): string {
  return `${(((value ?? 0) * 100)).toFixed(2)}%`
}

onMounted(() => {
  const { start, end } = getDefaultDates()
  startDate.value = start
  endDate.value = end
  fetchDashboard()
  fetchInviteCode()
})
</script>
