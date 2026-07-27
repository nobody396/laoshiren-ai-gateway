<template>
  <AppLayout>
    <main class="mx-auto max-w-7xl space-y-6 pb-12">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-primary-700 dark:text-primary-300">Affiliate V2.1</p>
          <h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">联盟运营台</h1>
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">计划开关、收款审核、人工打款和私域社群的单一操作入口。</p>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="loadAll">刷新</button>
      </header>

      <div v-if="error" class="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
        {{ error }}
      </div>

      <section v-if="loading" class="grid gap-4 md:grid-cols-3">
        <div v-for="item in 3" :key="item" class="card h-44 animate-pulse bg-gray-100 dark:bg-dark-800" />
      </section>

      <template v-else>
        <section v-if="program" class="card overflow-hidden">
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <div>
              <h2 class="text-xl font-semibold text-gray-950 dark:text-white">计划模式与核心规则</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">必须先经过 Shadow 验证，才能进入 Live；金额按固定精度保存。</p>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-500 dark:text-dark-400">Revision {{ program.revision }}</span>
              <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="programModeClass">{{ program.mode.toUpperCase() }}</span>
            </div>
          </div>

          <form class="p-6" @submit.prevent="saveProgram">
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">运行模式</span>
                <select v-model="programForm.mode" class="input">
                  <option value="off">Off · 不记录不入账</option>
                  <option value="shadow">Shadow · 只观察不入账</option>
                  <option value="live" :disabled="program.mode === 'off'">Live · 正式入账</option>
                </select>
              </label>
              <NumberField v-model="programForm.ordinaryReferralRate" label="普通邀请奖励" suffix="%" :min="0" :max="10" :step="1" />
              <NumberField v-model="programForm.firstPaidThreshold" label="首笔奖励门槛" prefix="¥" :min="0" :step="1" />
              <NumberField v-model="programForm.firstPaidBonus" label="被邀请人固定奖励" prefix="⚡" :min="0" :step="1" />
              <NumberField v-model="programForm.directUserCount" label="路线 A 有效用户" suffix="人" :min="1" :step="1" />
              <NumberField v-model="programForm.perUserConsumption" label="单个有效用户消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.directTeamConsumption" label="路线 A 团队消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.combinedConsumption" label="路线 B 合并消费" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.maxCampaignLinks" label="最多活动链接" suffix="条" :min="0" :max="100" :step="1" />
              <NumberField v-model="programForm.conversionMultiplier" label="现金转额度倍率" suffix="×" :min="1" :step="0.1" />
              <NumberField v-model="programForm.withdrawalMinimum" label="最低提现金额" prefix="¥" :min="1" :step="1" />
              <NumberField v-model="programForm.withdrawalSLAHours" label="处理 SLA" suffix="小时" :min="1" :max="168" :step="1" />
              <NumberField v-model="programForm.marginFloor" label="压力毛利率底线" suffix="%" :min="35" :max="100" :step="1" />
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900 sm:col-span-2 lg:col-span-3">
                <p class="text-xs text-gray-500 dark:text-dark-400">Agent 固定奖励池</p>
                <p class="mt-1 text-xl font-bold text-gray-950 dark:text-white">{{ program.agent_pool_rate_bps / 100 }}%</p>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">该值由后端锁死，不可扩大，客户返利与 Agent 现金佣金之和始终等于 10%。</p>
              </div>
            </div>
            <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-5 dark:border-dark-800">
              <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
                {{ program.started_at ? `首次 Live：${formatBeijingTime(program.started_at)}` : '尚未进入过 Live；首次启用时间将由服务器以 UTC 保存，并按北京时间展示。' }}
              </p>
              <button class="btn btn-primary" :disabled="programSaving">{{ programSaving ? '保存中…' : '保存计划设置' }}</button>
            </div>
          </form>
        </section>

        <section class="grid gap-6 xl:grid-cols-2">
          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">支付宝资料审核</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">只有实名、账号和收款码通过审核后才允许提现。</p>
              </div>
              <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ pendingProfiles.length }} 待审</span>
            </div>
            <div class="max-h-[42rem] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-800">
              <div v-for="profile in pendingProfiles" :key="profile.agent_id" class="p-5">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p class="font-semibold text-gray-900 dark:text-white">Agent #{{ profile.agent_id }} · {{ profile.alipay_real_name }}</p>
                    <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">{{ profile.alipay_account }} · {{ profile.contact_phone }}</p>
                    <p v-if="profile.payment_note" class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ profile.payment_note }}</p>
                  </div>
                  <button class="btn btn-secondary btn-sm" @click="previewPaymentProfile(profile)">查看收款码</button>
                </div>
                <input v-model="reviewNotes[profile.agent_id]" maxlength="500" class="input mt-4" placeholder="审核备注；拒绝时必填">
                <div class="mt-3 flex justify-end gap-2">
                  <button class="btn btn-secondary btn-sm" :disabled="reviewingId === profile.agent_id" @click="rejectPaymentProfile(profile)">拒绝并退回</button>
                  <button class="btn btn-primary btn-sm" :disabled="reviewingId === profile.agent_id" @click="verifyPaymentProfile(profile)">验证通过</button>
                </div>
              </div>
              <div v-if="!pendingProfiles.length" class="p-12 text-center text-sm text-gray-500 dark:text-dark-400">当前没有待审核资料</div>
            </div>
          </article>

          <article class="card overflow-hidden">
            <div class="flex items-center justify-between gap-3 border-b border-gray-100 px-6 py-5 dark:border-dark-800">
              <div>
                <h2 class="text-xl font-semibold text-gray-950 dark:text-white">提现打款队列</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">提交即“处理中”；人工扫码后只需标记“已到账”。</p>
              </div>
              <span class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-700 dark:bg-amber-950 dark:text-amber-300">{{ withdrawals.length }} 处理中</span>
            </div>
            <div class="max-h-[42rem] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-800">
              <div v-for="withdrawal in withdrawals" :key="withdrawal.id" class="p-5">
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p class="text-2xl font-bold text-gray-950 dark:text-white">{{ formatMicros(withdrawal.amount_micros, '¥') }}</p>
                    <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">Agent #{{ withdrawal.agent_id }} · {{ withdrawal.payment_alipay_real_name }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ withdrawal.payment_alipay_account }} · 截止 {{ formatBeijingTime(withdrawal.due_at) }}</p>
                  </div>
                  <button class="btn btn-secondary btn-sm" @click="previewWithdrawalQR(withdrawal)">扫码打款</button>
                </div>
                <input v-model="paymentReferences[withdrawal.id]" maxlength="200" class="input mt-4" placeholder="支付宝流水号（可选）">
                <div class="mt-3 flex justify-end gap-2">
                  <button class="btn btn-secondary btn-sm" :disabled="processingWithdrawalId === withdrawal.id" @click="failWithdrawal(withdrawal)">打款失败</button>
                  <button class="btn btn-primary btn-sm" :disabled="processingWithdrawalId === withdrawal.id" @click="completeWithdrawal(withdrawal)">标记已到账</button>
                </div>
              </div>
              <div v-if="!withdrawals.length" class="p-12 text-center text-sm text-gray-500 dark:text-dark-400">当前没有待打款申请</div>
            </div>
          </article>
        </section>

        <section v-if="community" class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-5 dark:border-dark-800">
            <h2 class="text-xl font-semibold text-gray-950 dark:text-white">Agent 私域社群卡片</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">启用后，仅已开通 Agent 能看到文字与二维码；二维码接口禁止公开缓存。</p>
          </div>
          <form class="grid gap-6 p-6 lg:grid-cols-[1fr_20rem]" @submit.prevent="saveCommunity">
            <div class="space-y-4">
              <label class="flex items-center gap-3">
                <input v-model="communityForm.enabled" type="checkbox" class="h-4 w-4 accent-primary-600">
                <span class="text-sm font-medium text-gray-800 dark:text-dark-200">启用 Agent 社群引导</span>
              </label>
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">标题</span>
                <input v-model.trim="communityForm.title" required maxlength="120" class="input">
              </label>
              <label class="block">
                <span class="mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300">加群说明</span>
                <textarea v-model.trim="communityForm.message" maxlength="2000" class="input min-h-36" />
              </label>
              <button class="btn btn-primary" :disabled="communitySaving">保存社群设置</button>
            </div>
            <div>
              <label class="block rounded-2xl border border-dashed border-gray-300 p-4 text-center text-sm text-gray-600 hover:border-primary-400 dark:border-dark-600 dark:text-dark-300">
                <input type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" @change="uploadCommunityQR">
                {{ community.has_qr_code ? '更换社群二维码' : '上传社群二维码' }}
              </label>
              <img v-if="communityQRPreview" :src="communityQRPreview" alt="Agent 社群二维码预览" class="mx-auto mt-4 max-h-64 rounded-xl border border-gray-200 p-2 dark:border-dark-700">
              <p v-else class="mt-4 rounded-xl bg-gray-50 p-6 text-center text-sm text-gray-500 dark:bg-dark-900 dark:text-dark-400">尚未上传二维码</p>
            </div>
          </form>
        </section>
      </template>

      <div v-if="qrPreviewURL" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true" @click.self="closeQRPreview">
        <div class="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl dark:bg-dark-900">
          <div class="flex items-center justify-between gap-3">
            <h2 class="font-semibold text-gray-950 dark:text-white">{{ qrPreviewTitle }}</h2>
            <button class="btn btn-secondary btn-sm" @click="closeQRPreview">关闭</button>
          </div>
          <img :src="qrPreviewURL" :alt="qrPreviewTitle" class="mx-auto mt-4 max-h-[65vh] rounded-xl border border-gray-200 p-2 dark:border-dark-700">
        </div>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  completeAffiliateWithdrawal,
  failAffiliateWithdrawal,
  getAffiliateCommunity,
  getAffiliateCommunityQRCode,
  getAffiliateProgram,
  getAffiliateWithdrawalQRCode,
  getPaymentQRCode,
  listAffiliateWithdrawals,
  listPendingPaymentProfiles,
  reviewPaymentProfile,
  updateAffiliateCommunity,
  updateAffiliateProgram,
  uploadAffiliateCommunityQRCode,
  type AdminAffiliateWithdrawal,
  type AffiliateCommunitySettings,
  type AffiliateProgramSettings,
  type AgentPaymentProfile
} from '@/api/admin/agents'
import { useAppStore } from '@/stores/app'
import { buildAuthErrorMessage } from '@/utils/authError'

const NumberField = defineComponent({
  props: {
    modelValue: { type: Number, required: true },
    label: { type: String, required: true },
    prefix: { type: String, default: '' },
    suffix: { type: String, default: '' },
    min: { type: Number, default: undefined },
    max: { type: Number, default: undefined },
    step: { type: Number, default: 1 }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('label', { class: 'block' }, [
      h('span', { class: 'mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300' }, props.label),
      h('div', { class: 'relative' }, [
        props.prefix ? h('span', { class: 'pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-500 dark:text-dark-400' }, props.prefix) : null,
        h('input', {
          value: props.modelValue,
          type: 'number',
          min: props.min,
          max: props.max,
          step: props.step,
          class: ['input', props.prefix ? 'pl-8' : '', props.suffix ? 'pr-14' : ''],
          onInput: (event: Event) => emit('update:modelValue', Number((event.target as HTMLInputElement).value))
        }),
        props.suffix ? h('span', { class: 'pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-500 dark:text-dark-400' }, props.suffix) : null
      ])
    ])
  }
})

const appStore = useAppStore()
const loading = ref(true)
const error = ref('')
const programSaving = ref(false)
const communitySaving = ref(false)
const reviewingId = ref<number | null>(null)
const processingWithdrawalId = ref<number | null>(null)
const program = ref<AffiliateProgramSettings | null>(null)
const community = ref<AffiliateCommunitySettings | null>(null)
const pendingProfiles = ref<AgentPaymentProfile[]>([])
const withdrawals = ref<AdminAffiliateWithdrawal[]>([])
const reviewNotes = reactive<Record<number, string>>({})
const paymentReferences = reactive<Record<number, string>>({})
const communityQRPreview = ref('')
const qrPreviewURL = ref('')
const qrPreviewTitle = ref('')
const programForm = reactive({
  mode: 'off' as AffiliateProgramSettings['mode'],
  ordinaryReferralRate: 5,
  firstPaidThreshold: 50,
  firstPaidBonus: 5,
  directUserCount: 10,
  perUserConsumption: 20,
  directTeamConsumption: 1000,
  combinedConsumption: 2000,
  maxCampaignLinks: 5,
  conversionMultiplier: 1.2,
  withdrawalMinimum: 100,
  withdrawalSLAHours: 24,
  marginFloor: 35
})
const communityForm = reactive({ enabled: false, title: '', message: '' })

const programModeClass = computed(() => (
  program.value?.mode === 'live'
    ? 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300'
    : program.value?.mode === 'shadow'
      ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300'
      : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
))

watch(program, value => {
  if (!value) return
  programForm.mode = value.mode
  programForm.ordinaryReferralRate = value.ordinary_referral_rate_bps / 100
  programForm.firstPaidThreshold = microsToUnits(value.first_paid_bonus_threshold_micros)
  programForm.firstPaidBonus = microsToUnits(value.first_paid_bonus_micros)
  programForm.directUserCount = value.qualification_direct_user_count
  programForm.perUserConsumption = microsToUnits(value.qualification_min_user_consumption_micros)
  programForm.directTeamConsumption = microsToUnits(value.qualification_direct_team_consumption_micros)
  programForm.combinedConsumption = microsToUnits(value.qualification_combined_consumption_micros)
  programForm.maxCampaignLinks = value.max_campaign_links
  programForm.conversionMultiplier = value.commission_conversion_multiplier_millis / 1000
  programForm.withdrawalMinimum = microsToUnits(value.withdrawal_min_micros)
  programForm.withdrawalSLAHours = value.withdrawal_sla_hours
  programForm.marginFloor = value.margin_floor_bps / 100
}, { immediate: true })

watch(community, value => {
  if (!value) return
  communityForm.enabled = value.enabled
  communityForm.title = value.title
  communityForm.message = value.message
}, { immediate: true })

function microsToUnits(value: number) {
  return value / 1_000_000
}

function unitsToMicros(value: number) {
  return Math.round(value * 1_000_000)
}

function formatMicros(value: number, symbol: string) {
  return `${symbol}${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(microsToUnits(value))}`
}

function formatBeijingTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(new Date(value))
}

function setObjectURL(target: typeof communityQRPreview, blob: Blob) {
  if (target.value.startsWith('blob:')) URL.revokeObjectURL(target.value)
  target.value = URL.createObjectURL(blob)
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [settings, communitySettings, profiles, payoutQueue] = await Promise.all([
      getAffiliateProgram(),
      getAffiliateCommunity(),
      listPendingPaymentProfiles(),
      listAffiliateWithdrawals()
    ])
    program.value = settings
    community.value = communitySettings
    pendingProfiles.value = profiles
    withdrawals.value = payoutQueue
    if (communitySettings.has_qr_code) {
      try { setObjectURL(communityQRPreview, await getAffiliateCommunityQRCode()) } catch { /* optional preview */ }
    }
  } catch (cause: unknown) {
    error.value = buildAuthErrorMessage(cause, { fallback: '联盟运营数据加载失败' })
  } finally {
    loading.value = false
  }
}

async function saveProgram() {
  if (!program.value) return
  programSaving.value = true
  try {
    program.value = await updateAffiliateProgram({
      ...program.value,
      mode: programForm.mode,
      ordinary_referral_rate_bps: Math.round(programForm.ordinaryReferralRate * 100),
      first_paid_bonus_threshold_micros: unitsToMicros(programForm.firstPaidThreshold),
      first_paid_bonus_micros: unitsToMicros(programForm.firstPaidBonus),
      qualification_direct_user_count: Math.round(programForm.directUserCount),
      qualification_min_user_consumption_micros: unitsToMicros(programForm.perUserConsumption),
      qualification_direct_team_consumption_micros: unitsToMicros(programForm.directTeamConsumption),
      qualification_combined_consumption_micros: unitsToMicros(programForm.combinedConsumption),
      max_campaign_links: Math.round(programForm.maxCampaignLinks),
      commission_conversion_multiplier_millis: Math.round(programForm.conversionMultiplier * 1000),
      withdrawal_min_micros: unitsToMicros(programForm.withdrawalMinimum),
      withdrawal_sla_hours: Math.round(programForm.withdrawalSLAHours),
      margin_floor_bps: Math.round(programForm.marginFloor * 100)
    })
    appStore.showSuccess('联盟计划设置已保存')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '计划设置保存失败' }))
  } finally {
    programSaving.value = false
  }
}

async function verifyPaymentProfile(profile: AgentPaymentProfile) {
  reviewingId.value = profile.agent_id
  try {
    await reviewPaymentProfile(profile.agent_id, { status: 'verified', note: reviewNotes[profile.agent_id] || '' })
    pendingProfiles.value = pendingProfiles.value.filter(item => item.agent_id !== profile.agent_id)
    appStore.showSuccess(`Agent #${profile.agent_id} 收款资料已验证`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '验证失败' }))
  } finally {
    reviewingId.value = null
  }
}

async function rejectPaymentProfile(profile: AgentPaymentProfile) {
  const note = (reviewNotes[profile.agent_id] || '').trim()
  if (!note) {
    appStore.showError('拒绝时请填写修改原因')
    return
  }
  reviewingId.value = profile.agent_id
  try {
    await reviewPaymentProfile(profile.agent_id, { status: 'rejected', note })
    pendingProfiles.value = pendingProfiles.value.filter(item => item.agent_id !== profile.agent_id)
    appStore.showSuccess(`Agent #${profile.agent_id} 资料已退回`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '退回失败' }))
  } finally {
    reviewingId.value = null
  }
}

async function previewPaymentProfile(profile: AgentPaymentProfile) {
  try {
    setObjectURL(qrPreviewURL, await getPaymentQRCode(profile.agent_id))
    qrPreviewTitle.value = `Agent #${profile.agent_id} · ${profile.alipay_real_name}`
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款码加载失败' }))
  }
}

async function previewWithdrawalQR(withdrawal: AdminAffiliateWithdrawal) {
  try {
    setObjectURL(qrPreviewURL, await getAffiliateWithdrawalQRCode(withdrawal.id))
    qrPreviewTitle.value = `提现 #${withdrawal.id} · ${formatMicros(withdrawal.amount_micros, '¥')}`
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '收款码加载失败' }))
  }
}

function closeQRPreview() {
  if (qrPreviewURL.value.startsWith('blob:')) URL.revokeObjectURL(qrPreviewURL.value)
  qrPreviewURL.value = ''
  qrPreviewTitle.value = ''
}

async function completeWithdrawal(withdrawal: AdminAffiliateWithdrawal) {
  processingWithdrawalId.value = withdrawal.id
  try {
    await completeAffiliateWithdrawal(withdrawal.id, paymentReferences[withdrawal.id] || '')
    withdrawals.value = withdrawals.value.filter(item => item.id !== withdrawal.id)
    appStore.showSuccess(`提现 #${withdrawal.id} 已标记到账`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '到账确认失败' }))
  } finally {
    processingWithdrawalId.value = null
  }
}

async function failWithdrawal(withdrawal: AdminAffiliateWithdrawal) {
  const reason = (paymentReferences[withdrawal.id] || '').trim()
  if (!reason) {
    appStore.showError('请在流水号输入框填写失败原因')
    return
  }
  processingWithdrawalId.value = withdrawal.id
  try {
    await failAffiliateWithdrawal(withdrawal.id, reason)
    withdrawals.value = withdrawals.value.filter(item => item.id !== withdrawal.id)
    appStore.showSuccess(`提现 #${withdrawal.id} 已退回余额`)
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '提现退回失败' }))
  } finally {
    processingWithdrawalId.value = null
  }
}

async function saveCommunity() {
  if (!community.value) return
  communitySaving.value = true
  try {
    community.value = await updateAffiliateCommunity({
      enabled: communityForm.enabled,
      title: communityForm.title,
      message: communityForm.message,
      revision: community.value.revision
    })
    appStore.showSuccess('社群卡片已保存')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '社群设置保存失败' }))
  } finally {
    communitySaving.value = false
  }
}

async function uploadCommunityQR(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  communitySaving.value = true
  try {
    community.value = await uploadAffiliateCommunityQRCode(file)
    setObjectURL(communityQRPreview, file)
    appStore.showSuccess('社群二维码已上传')
  } catch (cause: unknown) {
    appStore.showError(buildAuthErrorMessage(cause, { fallback: '二维码上传失败' }))
  } finally {
    communitySaving.value = false
    input.value = ''
  }
}

onMounted(loadAll)
onBeforeUnmount(() => {
  closeQRPreview()
  if (communityQRPreview.value.startsWith('blob:')) URL.revokeObjectURL(communityQRPreview.value)
})
</script>
