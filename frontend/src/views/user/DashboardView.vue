<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" @recharge="showRechargeModal = true" />
        <MonthlyCardStatus />
        <UserSavingsCard />

        <!-- 邀请看板（暂时隐藏） -->
        <div class="card p-6 space-y-5">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('user.referral.title') }}</h3>

          <!-- 统计卡片 + 邀请码横排 -->
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <!-- 已邀请用户 -->
            <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.referral.invitedCount') }}</p>
              <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ referralStats?.invited_user_count ?? '—' }}</p>
            </div>

            <!-- 累计佣金 -->
            <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.referral.totalCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">⚡{{ referralStats?.total_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>

            <!-- 本月佣金 -->
            <div class="rounded-lg bg-gray-50 dark:bg-dark-800 p-4 text-center">
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('user.referral.thisMonthCommission') }}</p>
              <p class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">⚡{{ referralStats?.this_month_commission.toFixed(4) ?? '0.0000' }}</p>
            </div>
          </div>

          <!-- 邀请码 + 复制链接 -->
          <div class="flex items-center gap-4 flex-wrap">
            <div v-if="inviteCodeLoading" class="h-10 w-40 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
            <template v-else>
              <div class="flex items-center gap-3">
                <span class="text-sm text-gray-600 dark:text-dark-300">{{ t('user.referral.myInviteCode') }}:</span>
                <code class="rounded-lg bg-gray-100 dark:bg-dark-800 px-4 py-2 text-lg font-mono font-bold tracking-widest text-primary-600 dark:text-primary-400">
                  {{ inviteCode || '—' }}
                </code>
                <button v-if="inviteCode" @click="copyInviteLink" class="btn btn-secondary btn-sm">
                  {{ copied ? t('common.copied') : t('common.copy') }}
                </button>
              </div>
            </template>
          </div>
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ inviteHint }}</p>
        </div>

        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
        </div>
      </template>
    </div>
  <RechargeModal v-model="showRechargeModal" @success="onRechargeSuccess" />
</AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { userAPI, type UserReferralDashboard } from '@/api/user'
import { getMyInviteCode } from '@/api/agent'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import MonthlyCardStatus from '@/components/user/dashboard/MonthlyCardStatus.vue'
import UserSavingsCard from '@/components/user/dashboard/UserSavingsCard.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import RechargeModal from '@/components/user/RechargeModal.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { UsageLog, TrendDataPoint, ModelStat } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])

const formatLD = (d: Date) => d.toISOString().split('T')[0]
const startDate = ref(formatLD(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatLD(new Date()))
const granularity = ref('day')

// 充值弹窗
const showRechargeModal = ref(false)

async function onRechargeSuccess() {
  await loadStats()
}

// 邀请相关
const inviteCode = ref('')
const inviteCodeLoading = ref(true)
const referralStats = ref<UserReferralDashboard | null>(null)
const { copied, copyToClipboard } = useClipboard()
const defaultInviteeBonusRate = 0.10
const defaultReferralBonusRate = 0.05

const registerUrl = computed(() =>
  inviteCode.value ? `${window.location.origin}/register?ref=${inviteCode.value}` : ''
)
const inviteeBonusRate = computed(() =>
  formatRate(referralStats.value?.first_recharge_invitee_rate ?? defaultInviteeBonusRate)
)
const referralBonusRate = computed(() =>
  formatRate(referralStats.value?.first_recharge_referral_rate ?? defaultReferralBonusRate)
)
const inviteHint = computed(() =>
  user.value?.role === 'agent'
    ? t('agent.inviteCodeHintWithRate', { rate: inviteeBonusRate.value })
    : t('user.referral.inviteHintWithRates', {
      inviteeRate: inviteeBonusRate.value,
      referralRate: referralBonusRate.value
    })
)

async function copyInviteLink() {
  const url = registerUrl.value || inviteCode.value
  if (!url) return
  await copyToClipboard(url, t('common.copiedToClipboard'))
}

function formatRate(value: number): string {
  return `${(value * 100).toFixed(2)}%`
}

const loadStats = async () => {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const res = await Promise.all([
      usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }),
      usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })
    ])
    trendData.value = res[0].trend || []
    modelStats.value = res[1].models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}

const loadRecent = async () => {
  loadingUsage.value = true
  try {
    const res = await usageAPI.getByDateRange(startDate.value, endDate.value)
    recentUsage.value = res.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

const loadReferral = async () => {
  inviteCodeLoading.value = true
  try {
    const [codeRes, statsRes] = await Promise.all([
      getMyInviteCode(),
      userAPI.getUserReferralDashboard()
    ])
    inviteCode.value = codeRes.invite_code
    referralStats.value = statsRes
  } catch (e) {
    console.error('Failed to load referral data:', e)
  } finally {
    inviteCodeLoading.value = false
  }
}

onMounted(() => {
  loadStats()
  loadCharts()
  loadRecent()
  loadReferral()
})
</script>
