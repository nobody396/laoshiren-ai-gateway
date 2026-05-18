<template>
  <AppLayout>
    <div class="relative overflow-hidden px-4 py-8 sm:px-6 lg:px-8">
      <div
        class="pointer-events-none absolute inset-x-0 top-0 h-64 bg-[radial-gradient(circle_at_top,rgba(37,99,235,0.18),transparent_60%)] dark:bg-[radial-gradient(circle_at_top,rgba(56,189,248,0.16),transparent_62%)]"
      ></div>

      <div class="relative mx-auto max-w-5xl">
        <section class="overflow-hidden rounded-[32px] border border-gray-200/80 bg-white/95 shadow-[0_30px_90px_-48px_rgba(15,23,42,0.45)] dark:border-dark-700 dark:bg-dark-900/95">
          <div class="grid lg:grid-cols-[1.05fr_0.95fr]">
            <div class="border-b border-gray-100 px-6 py-8 dark:border-dark-700 sm:px-8 lg:border-b-0 lg:border-r">
              <div
                class="inline-flex items-center rounded-full border border-primary-200 bg-primary-50 px-3 py-1 text-xs font-semibold tracking-[0.18em] text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-300"
              >
                {{ t('subscriptionAccess.badge') }}
              </div>

              <h1 class="mt-5 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-4xl">
                {{ t('subscriptionAccess.heading') }}
              </h1>
              <p class="mt-3 max-w-xl text-sm leading-7 text-gray-600 dark:text-dark-300 sm:text-base">
                {{ t('subscriptionAccess.summary') }}
              </p>

              <div v-if="cardShopMode" class="mt-8 space-y-8">
                <section>
                  <p class="mb-4 text-sm font-medium text-gray-700 dark:text-dark-300">
                    {{ t('topup.cardShopSelectAmount') }}
                  </p>
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <button
                      v-for="product in activeCardShopProducts"
                      :key="product.id"
                      @click="openCardShopProduct(product)"
                      class="rounded-2xl border border-gray-200 bg-white px-5 py-4 text-left transition-all hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-600 dark:bg-dark-800/80 dark:hover:border-primary-500/40 dark:hover:bg-dark-800"
                    >
                      <span class="block text-lg font-semibold text-gray-900 dark:text-white">
                        {{ product.label || `¥${product.amount_cny}` }}
                      </span>
                      <span class="mt-1 block text-sm text-gray-500 dark:text-dark-400">
                        {{ t('topup.cardShopAmount', { amount: product.amount_cny }) }}
                      </span>
                    </button>
                  </div>
                </section>

                <section class="rounded-2xl border border-primary-200 bg-primary-50 px-4 py-3 text-sm leading-6 text-primary-700 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-300">
                  {{ t('topup.cardShopHint') }}
                </section>
              </div>

              <div v-else-if="step === 1" class="mt-8 space-y-8">
                <section>
                  <p class="mb-4 text-sm font-medium text-gray-700 dark:text-dark-300">
                    {{ t('topup.selectAmount') }}
                  </p>
                  <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
                    <button
                      v-for="preset in presets"
                      :key="preset"
                      @click="selectPreset(preset)"
                      :class="[
                        'rounded-2xl border px-4 py-4 text-left transition-all',
                        selectedPreset === preset && !useCustom
                          ? 'border-primary-500 bg-primary-50 shadow-[0_18px_30px_-24px_rgba(59,130,246,0.8)] dark:bg-primary-900/20'
                          : 'border-gray-200 bg-white hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-600 dark:bg-dark-800/80 dark:hover:border-primary-500/40 dark:hover:bg-dark-800'
                      ]"
                    >
                      <span class="block text-lg font-semibold text-gray-900 dark:text-white">¥{{ preset }}</span>
                    </button>
                  </div>
                </section>

                <section>
                  <p class="mb-3 text-sm font-medium text-gray-700 dark:text-dark-300">
                    {{ t('topup.customAmount') }}
                  </p>
                  <div class="relative">
                    <span class="absolute left-4 top-1/2 -translate-y-1/2 text-lg font-semibold text-gray-400">¥</span>
                    <input
                      v-model="customAmountInput"
                      type="number"
                      min="20"
                      step="1"
                      :placeholder="t('topup.customAmountPlaceholder')"
                      @focus="useCustom = true; selectedPreset = null"
                      :class="[
                        'w-full rounded-2xl border px-12 py-4 text-base transition-colors focus:outline-none',
                        'bg-white text-gray-900 dark:bg-dark-800 dark:text-white',
                        useCustom
                          ? 'border-primary-500'
                          : 'border-gray-200 focus:border-primary-400 dark:border-dark-600 dark:focus:border-primary-500'
                      ]"
                    />
                  </div>
                  <p v-if="amountError" class="mt-2 text-sm text-red-500">{{ amountError }}</p>
                </section>

                <section>
                  <p class="mb-4 text-sm font-medium text-gray-700 dark:text-dark-300">
                    {{ t('topup.selectPayType') }}
                  </p>
                  <div
                    v-if="hasAvailablePayType"
                    :class="[
                      'grid gap-3',
                      hasMultiplePayTypes ? 'sm:grid-cols-2' : 'sm:grid-cols-1'
                    ]"
                  >
                    <button
                      v-if="canUseAlipay"
                      @click="selectPayType('alipay')"
                      :class="[
                        'flex items-center justify-between rounded-2xl border px-5 py-4 text-left transition-all',
                        payType === 'alipay'
                          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                          : 'border-gray-200 bg-white hover:border-blue-300 dark:border-dark-600 dark:bg-dark-800/80'
                      ]"
                    >
                      <div>
                        <span class="block text-base font-semibold text-gray-900 dark:text-white">{{ t('topup.alipay') }}</span>
                      </div>
                      <span class="flex h-10 w-10 items-center justify-center rounded-2xl bg-blue-500/10 text-blue-600 dark:text-blue-300">
                        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M21.422 15.358c-3.83-1.153-6.055-1.84-6.055-1.84.598-1.163.959-2.478 1.028-3.869H20V8.5h-4.5V7h-1.75v1.5h-4.5v1.149h7.397c-.106 2.73-1.5 4.872-3.5 6.114C11.47 17.21 9.48 16.5 7.5 16.5c-2.76 0-5 2.24-5 5 0 .17.01.34.03.5H2.5v.5h19v-.5h-.077A10.47 10.47 0 0022 20c0-1.9-.214-3.375-.578-4.642zM7.5 20c-1.38 0-2.5-1.12-2.5-2.5S6.12 15 7.5 15s2.5 1.12 2.5 2.5S8.88 20 7.5 20z"/>
                        </svg>
                      </span>
                    </button>

                    <button
                      v-if="canUseWechat"
                      @click="selectPayType('wechat')"
                      :class="[
                        'flex items-center justify-between rounded-2xl border px-5 py-4 text-left transition-all',
                        payType === 'wechat'
                          ? 'border-green-500 bg-green-50 dark:bg-green-900/20'
                          : 'border-gray-200 bg-white hover:border-green-300 dark:border-dark-600 dark:bg-dark-800/80'
                      ]"
                    >
                      <div>
                        <span class="block text-base font-semibold text-gray-900 dark:text-white">{{ t('topup.wechat') }}</span>
                      </div>
                      <span class="flex h-10 w-10 items-center justify-center rounded-2xl bg-green-500/10 text-green-600 dark:text-green-300">
                        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M8.667 12c0 .58.47 1.05 1.05 1.05S10.767 12.58 10.767 12s-.47-1.05-1.05-1.05S8.667 11.42 8.667 12zm5.666 0c0 .58.47 1.05 1.05 1.05s1.05-.47 1.05-1.05-.47-1.05-1.05-1.05-1.05.47-1.05 1.05zM12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z"/>
                        </svg>
                      </span>
                    </button>
                  </div>
                  <p
                    v-else
                    class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-300"
                  >
                    {{ t('topup.noAvailablePayType') }}
                  </p>
                </section>
              </div>

              <div v-else class="mt-8 rounded-[28px] border border-gray-200 bg-gray-50/80 p-5 dark:border-dark-700 dark:bg-dark-800/70">
                <p class="text-sm leading-6 text-gray-600 dark:text-dark-300">
                  {{ t('topup.scanHint', { payType: payType === 'alipay' ? t('topup.alipay') : t('topup.wechat') }) }}
                </p>
                <div class="mt-5 flex justify-center">
                  <div
                    class="flex h-60 w-60 items-center justify-center rounded-[28px] border border-dashed border-gray-300 bg-white dark:border-dark-600 dark:bg-dark-900"
                  >
                    <img
                      v-if="qrCodeURL && !qrExpired"
                      :src="qrCodeURL"
                      alt="Payment QR Code"
                      class="h-52 w-52 rounded-2xl object-cover"
                    />
                    <div v-else class="space-y-3 text-center">
                      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('topup.qrExpired') }}</p>
                      <button @click="resubmitOrder" class="btn btn-secondary btn-sm">{{ t('topup.refresh') }}</button>
                    </div>
                  </div>
                </div>
                <div class="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-white px-4 py-3 dark:bg-dark-900">
                  <div>
                    <p class="text-xs uppercase tracking-[0.18em] text-gray-400">{{ t('topup.selectAmount') }}</p>
                    <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">¥{{ effectiveAmountYuan }}</p>
                  </div>
                  <div class="text-right">
                    <p class="text-xs uppercase tracking-[0.18em] text-gray-400">USD</p>
                    <p class="mt-1 text-lg font-semibold text-primary-600 dark:text-primary-400">${{ effectiveAmountYuan }}.00</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="bg-gray-50/80 px-6 py-8 dark:bg-dark-950/60 sm:px-8">
              <div class="rounded-[28px] border border-gray-200 bg-white p-6 shadow-[0_20px_60px_-44px_rgba(15,23,42,0.55)] dark:border-dark-700 dark:bg-dark-900">
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <p class="text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('topup.title') }}</p>
                    <p class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
                      <template v-if="cardShopMode">
                        {{ t('topup.cardShopModeTitle') }}
                      </template>
                      <template v-else>
                      ¥{{ effectiveAmountYuan || 20 }}
                      </template>
                    </p>
                  </div>
                  <div v-if="!cardShopMode" class="rounded-2xl bg-primary-500/10 px-3 py-2 text-sm font-semibold text-primary-700 dark:text-primary-300">
                    ${{ effectiveAmountYuan || 20 }}.00
                  </div>
                </div>

                <div class="mt-6 space-y-4">
                  <template v-if="cardShopMode">
                    <div class="flex items-center justify-between text-sm">
                      <span class="text-gray-500 dark:text-dark-400">{{ t('topup.cardShopProductCount') }}</span>
                      <span class="font-medium text-gray-900 dark:text-white">
                        {{ activeCardShopProducts.length }}
                      </span>
                    </div>
                    <div class="flex items-center justify-between text-sm">
                      <span class="text-gray-500 dark:text-dark-400">{{ t('topup.cardShopRedeemType') }}</span>
                      <span class="font-medium text-gray-900 dark:text-white">
                        {{ t('topup.cardShopRedeemLink') }}
                      </span>
                    </div>
                  </template>
                  <div v-else class="flex items-center justify-between text-sm">
                    <span class="text-gray-500 dark:text-dark-400">{{ t('topup.selectPayType') }}</span>
                    <span class="font-medium text-gray-900 dark:text-white">
                      {{ selectedPayTypeLabel }}
                    </span>
                  </div>
                  <div v-if="!cardShopMode" class="flex items-center justify-between text-sm">
                    <span class="text-gray-500 dark:text-dark-400">{{ t('subscriptionAccess.rateLabel') }}</span>
                    <span class="font-medium text-gray-900 dark:text-white">{{ t('topup.creditsNote') }}</span>
                  </div>
                  <div v-if="step === 2 && !qrExpired" class="flex items-center justify-between text-sm">
                    <span class="text-gray-500 dark:text-dark-400">{{ t('topup.qrExpiry') }}</span>
                    <span class="font-medium text-gray-900 dark:text-white">{{ countdownText }}</span>
                  </div>
                </div>

                <button
                  v-if="cardShopMode"
                  @click="goRedeem"
                  class="mt-8 w-full rounded-2xl bg-gray-950 px-5 py-4 text-base font-semibold text-white transition hover:bg-gray-800 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-100"
                >
                  {{ t('topup.cardShopGoRedeem') }}
                </button>

                <button
                  v-else-if="step === 1"
                  @click="submitOrder"
                  :disabled="submitting || !hasAvailablePayType || !!amountError || effectiveAmountYuan < 20"
                  class="mt-8 w-full rounded-2xl bg-gray-950 px-5 py-4 text-base font-semibold text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-100"
                >
                  <span v-if="submitting">{{ t('topup.submitting') }}</span>
                  <span v-else>{{ t('topup.confirmButton') }} — ¥{{ effectiveAmountYuan || 20 }}</span>
                </button>

                <div v-else class="mt-8 space-y-4">
                  <div class="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-300">
                    {{ t('topup.waitingPayment') }}
                  </div>
                  <button
                    @click="resetToForm"
                    class="w-full rounded-2xl border border-gray-200 px-5 py-4 text-sm font-semibold text-gray-700 transition hover:border-primary-300 hover:text-primary-700 dark:border-dark-600 dark:text-dark-200 dark:hover:border-primary-500/40 dark:hover:text-primary-300"
                  >
                    {{ t('subscriptionAccess.editAmount') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { createTopupOrder, queryTopupOrderStatus, type TopupPayType } from '@/api/topup'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CardShopProduct } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const presets = [20, 50, 100, 200, 1000, 2000]
const QR_TTL_SECONDS = 300

const step = ref<1 | 2>(1)
const payType = ref<TopupPayType>('alipay')
const selectedPreset = ref<number | null>(20)
const useCustom = ref(false)
const customAmountInput = ref('')
const submitting = ref(false)
const qrCodeURL = ref('')
const orderNo = ref('')
const qrExpired = ref(false)
const countdown = ref(QR_TTL_SECONDS)

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null

// 根据公开设置决定用户侧可见支付渠道；关闭的渠道直接不展示。
const xunhuAlipayEnabled = computed(() => appStore.cachedPublicSettings?.xunhu_alipay_enabled ?? true)
const xunhuWechatEnabled = computed(() => appStore.cachedPublicSettings?.xunhu_wechat_enabled ?? true)
const canUseAlipay = computed(() => xunhuAlipayEnabled.value)
const canUseWechat = computed(() => xunhuWechatEnabled.value)
const hasAvailablePayType = computed(() => canUseAlipay.value || canUseWechat.value)
const hasMultiplePayTypes = computed(() => canUseAlipay.value && canUseWechat.value)
const activeCardShopProducts = computed<CardShopProduct[]>(() =>
  [...(appStore.cachedPublicSettings?.card_shop_products ?? [])]
    .filter((product) => product.enabled && product.url && product.amount_cny > 0)
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.amount_cny - b.amount_cny)
)
const cardShopMode = computed(
  () => (appStore.cachedPublicSettings?.card_shop_enabled ?? false) && activeCardShopProducts.value.length > 0
)
const selectedPayTypeLabel = computed(() => {
  if (!hasAvailablePayType.value) return t('topup.noAvailablePayType')
  return payType.value === 'alipay' ? t('topup.alipay') : t('topup.wechat')
})

const effectiveAmountYuan = computed<number>(() => {
  if (useCustom.value) {
    const value = parseInt(customAmountInput.value, 10)
    return Number.isNaN(value) ? 0 : value
  }
  return selectedPreset.value ?? 0
})

const amountError = computed<string>(() => {
  if (effectiveAmountYuan.value > 0 && effectiveAmountYuan.value < 20) {
    return t('topup.minAmountError')
  }
  if (effectiveAmountYuan.value > 3000) {
    return t('topup.maxAmountError')
  }
  if (useCustom.value && customAmountInput.value !== '' && Number.isNaN(parseInt(customAmountInput.value, 10))) {
    return t('topup.invalidAmount')
  }
  return ''
})

const countdownText = computed(() => {
  const minutes = Math.floor(countdown.value / 60)
  const seconds = countdown.value % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
})

function selectPreset(value: number) {
  selectedPreset.value = value
  useCustom.value = false
  customAmountInput.value = ''
}

function selectPayType(type: TopupPayType) {
  if (type === 'alipay' && !canUseAlipay.value) return
  if (type === 'wechat' && !canUseWechat.value) return
  payType.value = type
}

function openCardShopProduct(product: CardShopProduct) {
  if (!product.url) return
  window.location.assign(product.url)
}

function goRedeem() {
  router.push('/redeem')
}

// 配置只保留一个渠道时，自动选中仍可用的支付方式。
function syncPayTypeWithSettings() {
  if (xunhuAlipayEnabled.value && !xunhuWechatEnabled.value) {
    payType.value = 'alipay'
  } else if (!xunhuAlipayEnabled.value && xunhuWechatEnabled.value) {
    payType.value = 'wechat'
  }
}

function stopTimers() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

function resetToForm() {
  stopTimers()
  step.value = 1
  qrCodeURL.value = ''
  orderNo.value = ''
  qrExpired.value = false
  countdown.value = QR_TTL_SECONDS
  submitting.value = false
}

async function submitOrder() {
  if (!hasAvailablePayType.value || effectiveAmountYuan.value < 20 || amountError.value) return
  syncPayTypeWithSettings()

  submitting.value = true
  try {
    const response = await createTopupOrder(effectiveAmountYuan.value * 100, payType.value)
    orderNo.value = response.order_no
    qrCodeURL.value = response.qr_code_url
    qrExpired.value = false
    countdown.value = QR_TTL_SECONDS
    step.value = 2
    startCountdown()
    startPolling()
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('topup.payFailed')))
  } finally {
    submitting.value = false
  }
}

async function resubmitOrder() {
  qrExpired.value = false
  countdown.value = QR_TTL_SECONDS
  await submitOrder()
}

function startCountdown() {
  stopTimers()
  countdownTimer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) {
      qrExpired.value = true
      stopTimers()
    }
  }, 1000)
}

function startPolling() {
  pollTimer = setInterval(async () => {
    if (!orderNo.value || qrExpired.value) return

    try {
      const status = await queryTopupOrderStatus(orderNo.value)
      if (status.status === 'completed') {
        stopTimers()
        appStore.showSuccess(t('topup.paySuccess'))
        resetToForm()
      } else if (status.status === 'expired') {
        qrExpired.value = true
        stopTimers()
      }
    } catch (error) {
      console.error('Failed to query topup order status:', error)
    }
  }, 3000)
}

onUnmounted(() => {
  stopTimers()
})

void appStore.fetchPublicSettings().then(() => {
  syncPayTypeWithSettings()
})
</script>
