<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <!-- Date Range Filter -->
      <div class="admin-filter-slab card p-4">
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
        <div v-for="i in 6" :key="i" class="admin-stat-card card h-24 animate-pulse bg-gray-100 dark:bg-dark-800"></div>
      </div>
      <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="creditCard" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.totalCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ dashboard?.total_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="checkCircle" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.settledCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ dashboard?.settled_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="clock" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.unsettledCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">¥{{ dashboard?.unsettled_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="calendar" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.thisMonthCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">¥{{ dashboard?.this_month_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="filter" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.periodCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">¥{{ dashboard?.period_commission.toFixed(4) ?? '0.0000' }}</p>
              <p class="mt-1 text-xs text-gray-400">{{ startDate && endDate ? `${startDate} ~ ${endDate}` : t('agent.allTime') }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="users" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.invitedUsers') }}</p>
              <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ dashboard?.invited_user_count ?? 0 }}</p>
            </div>
          </div>
        </div>
        <div class="admin-stat-card card p-5">
          <div class="flex items-center gap-3">
            <div class="admin-stat-icon flex h-10 w-10 flex-shrink-0 items-center justify-center">
              <Icon name="trendingUp" size="md" :stroke-width="2" />
            </div>
            <div class="min-w-0">
              <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.currentRate') }}</p>
              <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ formatRate(dashboard?.consumption_rate) }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Level Overview -->
      <div v-if="!loading && canEditPaymentProfile" class="card p-6">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div class="min-w-0">
            <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('agent.agentLevel') }}</p>
            <div class="mt-2 flex flex-wrap items-center gap-3">
              <span class="rounded-md bg-primary-50 px-3 py-1.5 text-lg font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
                {{ currentLevelLabel }}
              </span>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ t('agent.permanentLevel') }}: {{ permanentLevelLabel }}</span>
            </div>
          </div>
          <div class="grid w-full grid-cols-1 gap-3 sm:grid-cols-2 lg:max-w-3xl xl:grid-cols-4">
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.currentRate') }}</p>
              <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatRate(dashboard?.consumption_rate) }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.assessmentPeriod') }}</p>
              <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ assessmentPeriodLabel }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.nextAssessment') }}</p>
              <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ nextAssessmentLabel }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.upgradeTime') }}</p>
              <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ upgradeTimeLabel }}</p>
            </div>
          </div>
        </div>

        <div class="mt-6 grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('agent.monthlyProgress') }}</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ monthlyProgress ? t('agent.targetLevelWithRate', { level: displayLevelName(monthlyProgress.level_name, monthlyProgress.level_key), rate: formatRate(monthlyProgress.rate) }) : t('agent.maxLevelReached') }}
                </p>
              </div>
              <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">{{ progressPercent(monthlyProgress) }}</span>
            </div>
            <div class="mt-4 h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
              <div class="h-full rounded-full bg-primary-500 transition-all" :style="progressStyle(monthlyProgress)"></div>
            </div>
            <div class="mt-3 grid grid-cols-3 gap-3 text-sm">
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.thisMonthPerformance') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(monthlyProgress?.current_consumption ?? dashboard?.this_month_consumption) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.targetPerformance') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(monthlyProgress?.threshold) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.performanceGap') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(monthlyProgress?.gap) }}</p>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('agent.cumulativeProgress') }}</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ cumulativeProgress ? t('agent.targetLevelWithRate', { level: displayLevelName(cumulativeProgress.level_name, cumulativeProgress.level_key), rate: formatRate(cumulativeProgress.rate) }) : t('agent.maxLevelReached') }}
                </p>
              </div>
              <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">{{ progressPercent(cumulativeProgress) }}</span>
            </div>
            <div class="mt-4 h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
              <div class="h-full rounded-full bg-primary-500 transition-all" :style="progressStyle(cumulativeProgress)"></div>
            </div>
            <div class="mt-3 grid grid-cols-3 gap-3 text-sm">
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.totalPerformance') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(cumulativeProgress?.current_consumption ?? dashboard?.total_consumption) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.targetPerformance') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(cumulativeProgress?.threshold) }}</p>
              </div>
              <div>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.performanceGap') }}</p>
                <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatMoney(cumulativeProgress?.gap) }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Settlement & Payment Profile -->
      <div v-if="!loading" class="card p-6">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('agent.paymentProfile') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('agent.paymentProfileHint') }}</p>
          </div>
          <div class="grid w-full grid-cols-1 gap-3 sm:grid-cols-3 lg:max-w-2xl">
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.unsettledCommission') }}</p>
              <p class="mt-1 text-xl font-semibold text-green-600 dark:text-green-400">{{ formatMoney(dashboard?.unsettled_commission) }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.settlementMinimum') }}</p>
              <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatMoney(dashboard?.settlement_minimum_amount) }}</p>
            </div>
            <div>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('agent.settlementStatus') }}</p>
              <p class="mt-1 text-sm font-semibold" :class="dashboard?.settlement_eligible ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'">
                {{ settlementStatusLabel }}
              </p>
            </div>
          </div>
        </div>

        <div class="mt-5">
          <div class="h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
            <div class="h-full rounded-full bg-green-500 transition-all" :style="{ width: `${settlementProgressPercent}%` }"></div>
          </div>
          <div class="mt-2 flex flex-wrap justify-between gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span>{{ t('agent.settlementProgress') }} {{ settlementProgressPercent }}%</span>
            <span>{{ settlementGapLabel }}</span>
          </div>
        </div>

        <div class="mt-6 grid grid-cols-1 gap-5 lg:grid-cols-[1fr_220px]">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('agent.alipayRealName') }}</label>
              <input v-model.trim="paymentForm.alipay_real_name" class="input" :placeholder="t('agent.alipayRealNamePlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('agent.alipayAccount') }}</label>
              <input v-model.trim="paymentForm.alipay_account" class="input" :placeholder="t('agent.alipayAccountPlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('agent.contactPhone') }}</label>
              <input v-model.trim="paymentForm.contact_phone" class="input" :placeholder="t('agent.optional')" />
            </div>
            <div>
              <label class="input-label">{{ t('agent.alipayQRCode') }}</label>
              <input type="file" accept="image/png,image/jpeg,image/webp" class="input text-sm" :disabled="qrUploading || !paymentPrivacyConsent" @change="handleQRCodeChange" />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('agent.paymentNote') }}</label>
              <textarea v-model.trim="paymentForm.payment_note" class="input min-h-20" :placeholder="t('agent.optional')" />
            </div>
            <div class="sm:col-span-2 rounded-lg border border-primary-200 bg-primary-50 p-3 text-xs leading-5 text-gray-700 dark:border-primary-900 dark:bg-primary-950 dark:text-dark-200">
              {{ t('agent.paymentPrivacySummary') }}
              <router-link to="/legal/affiliate-payment-privacy" class="font-semibold text-primary-700 hover:underline dark:text-primary-300">
                {{ t('agent.paymentPrivacyNotice') }}
              </router-link>
            </div>
            <label class="sm:col-span-2 flex items-start gap-2 text-xs leading-5 text-gray-700 dark:text-dark-200">
              <input v-model="paymentPrivacyConsent" type="checkbox" class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span>{{ t('agent.paymentPrivacyConsent') }}</span>
            </label>
            <div class="sm:col-span-2 flex flex-wrap items-center gap-3">
              <button class="btn btn-primary btn-sm" :disabled="paymentSaving || !paymentPrivacyConsent" @click="savePaymentProfile">
                {{ paymentSaving ? t('common.saving') : t('common.save') }}
              </button>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ paymentProfileUpdatedLabel }}</span>
            </div>
          </div>

          <div class="flex min-h-52 items-center justify-center rounded-lg border border-dashed border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900">
            <img v-if="qrPreviewUrl" :src="qrPreviewUrl" class="max-h-48 max-w-full rounded-md object-contain" :alt="t('agent.alipayQRCode')" />
            <div v-else class="text-center text-sm text-gray-500 dark:text-dark-400">{{ t('agent.noAlipayQRCode') }}</div>
          </div>
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
          {{ t('agent.inviteCodeHintWithRate', { rate: inviteeBonusRate }) }}
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
import { ref, onMounted, onUnmounted, computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  AGENT_PAYMENT_PRIVACY_NOTICE_VERSION,
  getAgentDashboard,
  getAgentInviteCode,
  getAgentPaymentProfile,
  getAgentPaymentQRCode,
  updateAgentPaymentProfile,
  uploadAgentPaymentQRCode,
  type AgentDashboard,
  type AgentLevelProgress,
  type AgentPaymentProfile,
  type AgentPaymentProfileUpdate
} from '@/api/agent'
import { buildAuthErrorMessage } from '@/utils/authError'
import { imageBlobToDataURL } from '@/utils/imagePreview'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const inviteCodeLoading = ref(true)
const error = ref('')
const dashboard = ref<AgentDashboard | null>(null)
const inviteCode = ref('')
const { copied, copyToClipboard } = useClipboard()
const startDate = ref('')
const endDate = ref('')
const defaultInviteeBonusRate = 0.10
const paymentSaving = ref(false)
const qrUploading = ref(false)
const paymentPrivacyConsent = ref(false)
const qrPreviewUrl = ref('')
const paymentForm = reactive<Pick<AgentPaymentProfile, 'alipay_real_name' | 'alipay_account' | 'contact_phone' | 'payment_note'>>({
  alipay_real_name: '',
  alipay_account: '',
  contact_phone: '',
  payment_note: ''
})
const paymentProfileUpdatedAt = ref('')

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

const inviteeBonusRate = computed(() =>
  formatRate(dashboard.value?.first_recharge_invitee_rate ?? defaultInviteeBonusRate)
)

const currentLevelLabel = computed(() => displayLevelName(dashboard.value?.current_level_name, dashboard.value?.current_level))
const permanentLevelLabel = computed(() => displayLevelName(dashboard.value?.permanent_level_name, dashboard.value?.permanent_level))
const monthlyProgress = computed(() => dashboard.value?.next_monthly_progress ?? null)
const cumulativeProgress = computed(() => dashboard.value?.next_cumulative_progress ?? null)
const canEditPaymentProfile = computed(() => authStore.user?.role === 'agent')
const nextAssessmentLabel = computed(() => formatDateTime(dashboard.value?.next_assessment_at))
const assessmentPeriodLabel = computed(() => {
  const start = formatDateTime(dashboard.value?.assessment_period_start)
  const end = formatDateTime(dashboard.value?.assessment_period_end)
  if (start === '—' || end === '—') return '—'
  return `${start} - ${end}`
})
const upgradeTimeLabel = computed(() => {
  if (nextAssessmentLabel.value === '—') return '—'
  const hasReachedTarget = [monthlyProgress.value, cumulativeProgress.value].some((item) => item && item.gap <= 0.000001)
  if (hasReachedTarget) {
    return t('agent.upgradeAtAssessment', { time: nextAssessmentLabel.value })
  }
  return t('agent.upgradeAfterTargetAtAssessment', { time: nextAssessmentLabel.value })
})
const settlementProgressPercent = computed(() => {
  const minimum = dashboard.value?.settlement_minimum_amount || 50
  if (minimum <= 0) return 100
  const unsettled = dashboard.value?.unsettled_commission || 0
  return Math.min(100, Math.max(0, Math.round((unsettled / minimum) * 100)))
})
const settlementStatusLabel = computed(() => {
  if (!dashboard.value?.payment_profile_complete) return t('agent.paymentProfileIncomplete')
  if (dashboard.value?.settlement_eligible) return t('agent.settlementEligible')
  return t('agent.settlementBelowMinimum')
})
const settlementGapLabel = computed(() => {
  const gap = dashboard.value?.settlement_gap || 0
  if (gap <= 0.000001) return t('agent.settlementReady')
  return t('agent.settlementGap', { amount: formatMoney(gap) })
})
const paymentProfileUpdatedLabel = computed(() => {
  if (!paymentProfileUpdatedAt.value) return t('agent.paymentProfileNotSaved')
  return t('agent.paymentProfileUpdatedAt', { time: formatDateTime(paymentProfileUpdatedAt.value) })
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

async function fetchPaymentProfile() {
  try {
    const profile = await getAgentPaymentProfile()
    applyPaymentProfile(profile)
    if (profile.has_alipay_qr) {
      await refreshQRCodePreview()
    }
  } catch (e: unknown) {
    error.value = buildAuthErrorMessage(e, { fallback: t('common.error') })
  }
}

async function savePaymentProfile() {
  if (!paymentPrivacyConsent.value) {
    appStore.showError(t('agent.paymentPrivacyConsentRequired'))
    return
  }
  paymentSaving.value = true
  try {
    const profile = await updateAgentPaymentProfile(paymentProfilePayload())
    applyPaymentProfile(profile)
    appStore.showSuccess(t('agent.paymentProfileSaved'))
    await fetchDashboard()
  } catch (e: unknown) {
    appStore.showError(buildAuthErrorMessage(e, { fallback: t('common.error') }))
  } finally {
    paymentSaving.value = false
  }
}

async function handleQRCodeChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!paymentPrivacyConsent.value) {
    appStore.showError(t('agent.paymentPrivacyConsentRequired'))
    input.value = ''
    return
  }
  qrUploading.value = true
  try {
    const savedProfile = await updateAgentPaymentProfile(paymentProfilePayload())
    applyPaymentProfile(savedProfile)

    const profile = await uploadAgentPaymentQRCode(file)
    applyPaymentProfile(profile)
    setQRCodePreview(await imageBlobToDataURL(file))
    await refreshQRCodePreview({ clearOnError: false })
    appStore.showSuccess(t('agent.alipayQRCodeSaved'))
    await fetchDashboard()
  } catch (e: unknown) {
    appStore.showError(buildAuthErrorMessage(e, { fallback: t('common.error') }))
  } finally {
    qrUploading.value = false
    input.value = ''
  }
}

function applyPaymentProfile(profile: AgentPaymentProfile) {
  paymentForm.alipay_real_name = profile.alipay_real_name || ''
  paymentForm.alipay_account = profile.alipay_account || ''
  paymentForm.contact_phone = profile.contact_phone || ''
  paymentForm.payment_note = profile.payment_note || ''
  paymentProfileUpdatedAt.value = profile.updated_at || ''
  paymentPrivacyConsent.value = profile.privacy_consent_current
}

function paymentProfilePayload(): AgentPaymentProfileUpdate {
  return {
    alipay_real_name: paymentForm.alipay_real_name,
    alipay_account: paymentForm.alipay_account,
    contact_phone: paymentForm.contact_phone,
    payment_note: paymentForm.payment_note,
    privacy_consent_accepted: true as const,
    privacy_consent_version: AGENT_PAYMENT_PRIVACY_NOTICE_VERSION
  }
}

async function refreshQRCodePreview(options: { clearOnError?: boolean } = {}) {
  try {
    const blob = await getAgentPaymentQRCode()
    setQRCodePreview(await imageBlobToDataURL(blob))
  } catch {
    if (options.clearOnError !== false) {
      setQRCodePreview('')
    }
  }
}

function setQRCodePreview(url: string) {
  if (qrPreviewUrl.value.startsWith('blob:')) {
    URL.revokeObjectURL(qrPreviewUrl.value)
  }
  qrPreviewUrl.value = url
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

function formatMoney(value?: number | null): string {
  if (value == null) return '—'
  return `¥${value.toFixed(2)}`
}

function progressPercent(progress?: AgentLevelProgress | null): string {
  if (!progress) return '—'
  return `${Math.round((progress.progress ?? 0) * 100)}%`
}

function progressStyle(progress?: AgentLevelProgress | null) {
  const width = progress ? Math.min(Math.max(progress.progress ?? 0, 0), 1) * 100 : 0
  return { width: `${width}%` }
}

function formatDateTime(value?: string): string {
  if (!value) return '—'
  const isoWallTime = value.match(/^(\d{4})-(\d{2})-(\d{2})[T\s](\d{2}):(\d{2})/)
  if (isoWallTime) {
    return `${isoWallTime[1]}/${isoWallTime[2]}/${isoWallTime[3]} ${isoWallTime[4]}:${isoWallTime[5]}`
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatLevelKey(value?: string): string {
  switch (value) {
    case 'light':
      return t('agent.levelLight')
    case 'standard':
      return t('agent.levelStandard')
    case 'core':
      return t('agent.levelCore')
    case 'super':
      return t('agent.levelSuper')
    case 'manual_base':
      return t('agent.levelManual')
    default:
      return '—'
  }
}

function displayLevelName(name?: string | null, fallbackKey?: string): string {
  const normalized = (name || '').trim()
  switch (normalized) {
    case '轻代理':
      return t('agent.levelLight')
    case '标准代理':
      return t('agent.levelStandard')
    case '核心代理':
      return t('agent.levelCore')
    case '超级代理':
      return t('agent.levelSuper')
    default:
      return normalized || formatLevelKey(fallbackKey)
  }
}

onMounted(() => {
  const { start, end } = getDefaultDates()
  startDate.value = start
  endDate.value = end
  fetchDashboard()
  fetchInviteCode()
  if (canEditPaymentProfile.value) {
    fetchPaymentProfile()
  }
})

onUnmounted(() => {
  setQRCodePreview('')
})
</script>
