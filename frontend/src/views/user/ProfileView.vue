<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <div class="grid grid-cols-1 gap-6 sm:grid-cols-3">
        <StatCard :title="t('profile.accountBalance')" :value="formatCurrency(user?.balance || 0)" :icon="WalletIcon" icon-variant="success" />
        <StatCard :title="t('profile.concurrencyLimit')" :value="user?.concurrency || 0" :icon="BoltIcon" icon-variant="warning" />
        <StatCard :title="t('profile.memberSince')" :value="formatDate(user?.created_at || '', { year: 'numeric', month: 'long' })" :icon="CalendarIcon" icon-variant="primary" />
      </div>
      <ProfileInfoCard :user="user" />
      <div class="card p-6">
        <h3 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('profile.myInviteCode') }}</h3>
        <div v-if="inviteCodeLoading" class="h-10 w-48 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
        <div v-else class="space-y-2">
          <div class="flex flex-wrap items-center gap-3">
            <code class="rounded-lg bg-gray-100 px-4 py-2 font-mono font-bold tracking-widest text-primary-600 dark:bg-dark-800 dark:text-primary-400">
              {{ myInviteCode || '—' }}
            </code>
            <button v-if="myInviteCode" @click="copyInviteLink" class="btn btn-secondary btn-sm">
              {{ codeCopied ? t('common.copied') : t('common.copy') }}
            </button>
          </div>
          <p v-if="registerUrl" class="break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ registerUrl }}</p>
        </div>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ inviteCodeHint }}</p>
      </div>
      <ProfileEditForm :initial-username="user?.username || ''" />
      <ProfileIdentityBindingsCard />
      <BalanceAlertCard />
      <ProfilePasswordForm />
      <ProfileTotpCard />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'
import { userAPI, type UserReferralDashboard } from '@/api/user'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatCard from '@/components/common/StatCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfileIdentityBindingsCard from '@/components/user/profile/ProfileIdentityBindingsCard.vue'
import BalanceAlertCard from '@/components/user/profile/BalanceAlertCard.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import { getMyInviteCode } from '@/api/agent'
import { useClipboard } from '@/composables/useClipboard'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const myInviteCode = ref('')
const inviteCodeLoading = ref(true)
const referralStats = ref<UserReferralDashboard | null>(null)
const { copied: codeCopied, copyToClipboard } = useClipboard()
const defaultInviteeBonusRate = 0.10
const defaultReferralBonusRate = 0.05
const registerUrl = computed(() =>
  myInviteCode.value ? `${window.location.origin}/register?ref=${myInviteCode.value}` : ''
)
const inviteeBonusRate = computed(() =>
  formatRate(referralStats.value?.first_recharge_invitee_rate ?? defaultInviteeBonusRate)
)
const referralBonusRate = computed(() =>
  formatRate(referralStats.value?.first_recharge_referral_rate ?? defaultReferralBonusRate)
)
const inviteCodeHint = computed(() =>
  user.value?.role === 'agent'
    ? t('agent.inviteCodeHintWithRate', { rate: inviteeBonusRate.value })
    : t('profile.inviteCodeHintWithRates', {
      inviteeRate: inviteeBonusRate.value,
      referralRate: referralBonusRate.value
    })
)

const WalletIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12' })]) }
const BoltIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'm3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z' })]) }
const CalendarIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'M6.75 3v2.25M17.25 3v2.25' })]) }

async function copyInviteLink() {
  if (!myInviteCode.value) return
  await copyToClipboard(registerUrl.value || myInviteCode.value, t('common.copiedToClipboard'))
}

onMounted(async () => {
  try {
    const inviteCode = await getMyInviteCode()
    myInviteCode.value = inviteCode.invite_code
  } catch {
    // ignore invite code failures
  }

  try {
    referralStats.value = await userAPI.getUserReferralDashboard()
  } catch {
    // ignore referral dashboard failures
  } finally {
    inviteCodeLoading.value = false
  }
})

const formatCurrency = (v: number) => `⚡${v.toFixed(2)}`
const formatRate = (v: number) => `${(v * 100).toFixed(2)}%`
</script>
