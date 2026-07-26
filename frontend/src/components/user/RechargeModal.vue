<template>
  <Teleport to="body">
    <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center p-4">
      <!-- Backdrop -->
      <div class="absolute inset-0 bg-black/50" @click="close" />

      <!-- Modal -->
      <div class="relative w-full max-w-md rounded-2xl bg-white dark:bg-dark-900 shadow-xl overflow-hidden">
        <!-- Header -->
        <div class="flex items-center justify-between px-6 py-4 border-b border-gray-100 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('topup.title') }}</h2>
          <button @click="close" class="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-dark-700 transition-colors">
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Step 1: Select amount & pay type -->
        <div v-if="step === 1" class="px-6 py-5 space-y-5">
          <div v-if="showChannelSelector" class="space-y-2">
            <p class="text-sm font-medium text-gray-700 dark:text-dark-300">{{ t('topup.chooseChannel') }}</p>
            <div class="grid grid-cols-2 gap-2">
              <button
                v-if="cardShopMode"
                type="button"
                @click="selectTopupChannel('card_shop')"
                :class="[
                  'rounded-lg border-2 px-3 py-3 text-left transition-all',
                  showingCardShop
                    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                    : 'border-gray-200 text-gray-700 hover:border-primary-300 dark:border-dark-600 dark:text-dark-200'
                ]"
              >
                <span class="flex items-center gap-2 text-sm font-semibold">
                  <Icon name="gift" size="sm" />
                  {{ t('topup.cardShopChannelTitle') }}
                </span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('topup.availableNow') }}</span>
              </button>
              <button
                type="button"
                @click="selectTopupChannel('qr')"
                :disabled="!qrTopupAvailable"
                :class="[
                  'rounded-lg border-2 px-3 py-3 text-left transition-all disabled:cursor-not-allowed disabled:opacity-70',
                  showingQrTopup
                    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                    : 'border-gray-200 text-gray-700 hover:border-primary-300 dark:border-dark-600 dark:text-dark-200'
                ]"
              >
                <span class="flex items-center gap-2 text-sm font-semibold">
                  <Icon name="creditCard" size="sm" />
                  {{ t('topup.qrChannelTitle') }}
                </span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                  {{ qrTopupAvailable ? t('topup.availableNow') : t('topup.comingSoon') }}
                </span>
              </button>
            </div>
          </div>

          <template v-if="showingCardShop">
            <div>
              <p class="text-sm font-medium text-gray-700 dark:text-dark-300 mb-3">{{ t('topup.cardShopSelectAmount') }}</p>
              <div class="grid grid-cols-1 gap-2">
                <button
                  v-for="product in activeCardShopProducts"
                  :key="product.id"
                  @click="openCardShopProduct(product)"
                  class="rounded-lg border-2 border-gray-200 px-4 py-3 text-left transition-all hover:border-primary-300 hover:bg-primary-50 dark:border-dark-600 dark:hover:border-primary-500/40"
                >
                  <span class="block text-sm font-semibold text-gray-900 dark:text-white">
                    {{ product.label || `¥${product.amount_cny}` }}
                  </span>
                  <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                    {{ t('topup.cardShopAmount', { amount: product.amount_cny }) }}
                  </span>
                </button>
              </div>
            </div>

            <p class="rounded-lg border border-primary-200 bg-primary-50 px-3 py-2.5 text-sm leading-6 text-primary-700 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-300">
              {{ t('topup.cardShopHint') }}
            </p>

            <button
              type="button"
              class="w-full btn btn-secondary py-3 text-base font-semibold"
              @click="goRedeem"
            >
              {{ t('topup.cardShopGoRedeem') }}
            </button>
          </template>

          <template v-else>
          <!-- Amount presets -->
          <div>
            <p class="text-sm font-medium text-gray-700 dark:text-dark-300 mb-3">{{ t('topup.selectAmount') }}</p>
            <div class="grid grid-cols-3 gap-2">
              <button
                v-for="preset in presets"
                :key="preset"
                @click="selectPreset(preset)"
                :class="[
                  'rounded-lg border-2 py-2.5 text-sm font-semibold transition-all',
                  selectedPreset === preset && !useCustom
                    ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 text-primary-700 dark:text-primary-300'
                    : 'border-gray-200 dark:border-dark-600 text-gray-700 dark:text-dark-200 hover:border-primary-300'
                ]"
              >
                ¥{{ preset }}
              </button>
            </div>
          </div>

          <!-- Custom amount -->
          <div>
            <p class="text-sm font-medium text-gray-700 dark:text-dark-300 mb-1.5">{{ t('topup.customAmount') }}</p>
            <div class="relative">
              <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 font-medium">¥</span>
              <input
                v-model="customAmountInput"
                type="number"
                min="20"
                step="1"
                :placeholder="t('topup.customAmountPlaceholder')"
                @focus="useCustom = true; selectedPreset = null"
                :class="[
                  'w-full rounded-lg border-2 pl-7 pr-4 py-2.5 text-sm transition-colors',
                  'bg-white dark:bg-dark-800 text-gray-900 dark:text-white',
                  useCustom
                    ? 'border-primary-500 focus:outline-none'
                    : 'border-gray-200 dark:border-dark-600 focus:border-primary-400 focus:outline-none'
                ]"
              />
            </div>
            <p v-if="amountError" class="mt-1 text-xs text-red-500">{{ amountError }}</p>
          </div>

          <!-- Pay type -->
          <div>
            <p class="text-sm font-medium text-gray-700 dark:text-dark-300 mb-2">{{ t('topup.selectPayType') }}</p>
            <div v-if="hasAvailablePayType" class="flex gap-3">
              <button
                v-if="canUseAlipay"
                @click="selectPayType('alipay')"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 rounded-lg border-2 py-2.5 text-sm font-medium transition-all',
                  payType === 'alipay'
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                    : 'border-gray-200 dark:border-dark-600 text-gray-600 dark:text-dark-300 hover:border-blue-300'
                ]"
              >
                <!-- Alipay icon -->
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M21.422 15.358c-3.83-1.153-6.055-1.84-6.055-1.84.598-1.163.959-2.478 1.028-3.869H20V8.5h-4.5V7h-1.75v1.5h-4.5v1.149h7.397c-.106 2.73-1.5 4.872-3.5 6.114C11.47 17.21 9.48 16.5 7.5 16.5c-2.76 0-5 2.24-5 5 0 .17.01.34.03.5H2.5v.5h19v-.5h-.077A10.47 10.47 0 0022 20c0-1.9-.214-3.375-.578-4.642zM7.5 20c-1.38 0-2.5-1.12-2.5-2.5S6.12 15 7.5 15s2.5 1.12 2.5 2.5S8.88 20 7.5 20z"/>
                </svg>
                {{ t('topup.alipay') }}
              </button>
              <button
                v-if="canUseWechat"
                @click="selectPayType('wechat')"
                :class="[
                  'flex-1 flex items-center justify-center gap-2 rounded-lg border-2 py-2.5 text-sm font-medium transition-all',
                  payType === 'wechat'
                    ? 'border-green-500 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300'
                    : 'border-gray-200 dark:border-dark-600 text-gray-600 dark:text-dark-300 hover:border-green-300'
                ]"
              >
                <!-- WeChat icon -->
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M8.667 12c0 .58.47 1.05 1.05 1.05S10.767 12.58 10.767 12s-.47-1.05-1.05-1.05S8.667 11.42 8.667 12zm5.666 0c0 .58.47 1.05 1.05 1.05s1.05-.47 1.05-1.05-.47-1.05-1.05-1.05-1.05.47-1.05 1.05zM12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z"/>
                </svg>
                {{ t('topup.wechat') }}
              </button>
            </div>
            <p
              v-else
              class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm text-amber-700 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-300"
            >
              {{ t('topup.noAvailablePayType') }}
            </p>
          </div>

          <!-- Note -->
          <p class="text-xs text-gray-400 dark:text-dark-400">{{ t('topup.creditsNote') }}</p>

          <!-- Confirm button -->
          <button
            @click="submitOrder"
            :disabled="submitting || !hasAvailablePayType || !!amountError || effectiveAmountYuan < 20"
            class="w-full btn btn-primary py-3 text-base font-semibold disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <span v-if="submitting" class="flex items-center justify-center gap-2">
              <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              {{ t('topup.submitting') }}
            </span>
            <span v-else>{{ t('topup.confirmButton') }} — ¥{{ effectiveAmountYuan }}</span>
          </button>
          </template>
        </div>

        <!-- Step 2: QR Code -->
        <div v-else-if="step === 2" class="px-6 py-5 space-y-4">
          <div class="text-center space-y-3">
            <p class="text-sm text-gray-600 dark:text-dark-300">
              {{ t('topup.scanHint', { payType: payType === 'alipay' ? t('topup.alipay') : t('topup.wechat') }) }}
            </p>

            <!-- QR Code image -->
            <div class="flex items-center justify-center">
              <div class="relative inline-block">
                <img
                  v-if="qrCodeURL && !qrExpired"
                  :src="qrCodeURL"
                  alt="Payment QR Code"
                  class="h-48 w-48 rounded-lg border border-gray-200 dark:border-dark-600"
                />
                <!-- Expired overlay -->
                <div v-if="qrExpired" class="flex h-48 w-48 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 dark:bg-dark-800 dark:border-dark-600">
                  <div class="text-center space-y-2">
                    <p class="text-sm text-gray-500">{{ t('topup.qrExpired') }}</p>
                    <button @click="resubmitOrder" class="btn btn-secondary btn-sm">{{ t('topup.refresh') }}</button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Countdown -->
            <div v-if="!qrExpired" class="flex items-center justify-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <span>{{ t('topup.qrExpiry') }}: {{ countdownText }}</span>
            </div>

            <!-- Amount info -->
            <div class="rounded-lg bg-gray-50 dark:bg-dark-800 px-4 py-3 text-sm">
              <span class="text-gray-500 dark:text-dark-400">充值金额：</span>
              <span class="font-semibold text-gray-900 dark:text-white">¥{{ displayAmountText }}</span>
              <span class="text-gray-400 mx-2">→</span>
              <span class="font-semibold text-primary-600 dark:text-primary-400">${{ displayUSDText }}</span>
            </div>

            <p class="text-xs text-gray-400 dark:text-dark-500">{{ t('topup.waitingPayment') }}</p>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { createTopupOrder, queryTopupOrderStatus, type TopupPayType } from '@/api/topup'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { CardShopProduct } from '@/types'
import { BALANCE_TOPUP_PRESETS, isSupportedBalanceTopupAmount } from '@/constants/balanceTopups'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'success': []
}>()

const presets = BALANCE_TOPUP_PRESETS
const QR_TTL_SECONDS = 300 // 5 分钟
type TopupChannel = 'card_shop' | 'qr'

// State
const step = ref<1 | 2>(1)
const selectedTopupChannel = ref<TopupChannel>('card_shop')
const payType = ref<TopupPayType>('alipay')
const selectedPreset = ref<number | null>(20)
const useCustom = ref(false)
const customAmountInput = ref('')
const submitting = ref(false)
const qrCodeURL = ref('')
const orderNo = ref('')
const qrExpired = ref(false)
const countdown = ref(QR_TTL_SECONDS)
const activeOrderAmountYuan = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let pollInFlight = false

// 根据公开设置决定用户侧可见支付渠道；关闭的渠道直接不展示。
const xunhuAlipayEnabled = computed(() => appStore.cachedPublicSettings?.xunhu_alipay_enabled ?? false)
const xunhuWechatEnabled = computed(() => appStore.cachedPublicSettings?.xunhu_wechat_enabled ?? false)
const canUseAlipay = computed(() => xunhuAlipayEnabled.value)
const canUseWechat = computed(() => xunhuWechatEnabled.value)
const hasAvailablePayType = computed(() => canUseAlipay.value || canUseWechat.value)
const qrTopupAvailable = computed(() => hasAvailablePayType.value)
const activeCardShopProducts = computed<CardShopProduct[]>(() =>
  [...(appStore.cachedPublicSettings?.card_shop_products ?? [])]
    .filter(
      (product) =>
        product.enabled &&
        product.url &&
        isSupportedBalanceTopupAmount(product.amount_cny)
    )
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.amount_cny - b.amount_cny)
)
const cardShopMode = computed(
  () => (appStore.cachedPublicSettings?.card_shop_enabled ?? false) && activeCardShopProducts.value.length > 0
)
const showChannelSelector = computed(() => cardShopMode.value || qrTopupAvailable.value)
const showingCardShop = computed(() => selectedTopupChannel.value === 'card_shop' && cardShopMode.value)
const showingQrTopup = computed(() => selectedTopupChannel.value === 'qr')

// 当前有效的金额（元）
const effectiveAmountYuan = computed<number>(() => {
  if (useCustom.value) {
    const v = parseInt(customAmountInput.value)
    return isNaN(v) ? 0 : v
  }
  return selectedPreset.value ?? 0
})

const displayAmountYuan = computed(() => {
  if (step.value === 2 && activeOrderAmountYuan.value > 0) {
    return activeOrderAmountYuan.value
  }
  return effectiveAmountYuan.value || 20
})

const displayAmountText = computed(() => formatMoney(displayAmountYuan.value, false))
const displayUSDText = computed(() => formatMoney(displayAmountYuan.value, true))

const amountError = computed<string>(() => {
  if (effectiveAmountYuan.value > 0 && effectiveAmountYuan.value < 20) {
    return t('topup.minAmountError')
  }
  if (useCustom.value && customAmountInput.value !== '' && isNaN(parseInt(customAmountInput.value))) {
    return t('topup.invalidAmount')
  }
  return ''
})

const countdownText = computed(() => {
  const m = Math.floor(countdown.value / 60)
  const s = countdown.value % 60
  return `${m}:${s.toString().padStart(2, '0')}`
})

function formatMoney(value: number, fixed: boolean) {
  if (!Number.isFinite(value) || value <= 0) return fixed ? '20.00' : '20'
  return fixed || !Number.isInteger(value) ? value.toFixed(2) : String(value)
}

function selectPreset(v: number) {
  selectedPreset.value = v
  useCustom.value = false
  customAmountInput.value = ''
}

function selectPayType(type: TopupPayType) {
  if (type === 'alipay' && !canUseAlipay.value) return
  if (type === 'wechat' && !canUseWechat.value) return
  payType.value = type
}

function selectTopupChannel(channel: TopupChannel) {
  if (channel === 'card_shop' && !cardShopMode.value) return
  if (channel === 'qr' && !qrTopupAvailable.value) return
  selectedTopupChannel.value = channel
  if (channel === 'qr') {
    syncPayTypeWithSettings()
  }
}

function openCardShopProduct(product: CardShopProduct) {
  if (!product.url) return
  window.location.assign(product.url)
}

async function goRedeem() {
  emit('update:modelValue', false)
  await router.push('/redeem')
}

// 配置只保留一个渠道时，自动选中仍可用的支付方式。
function syncPayTypeWithSettings() {
  if (xunhuAlipayEnabled.value && !xunhuWechatEnabled.value) {
    payType.value = 'alipay'
  } else if (!xunhuAlipayEnabled.value && xunhuWechatEnabled.value) {
    payType.value = 'wechat'
  }
}

function syncTopupChannelWithSettings() {
  if (selectedTopupChannel.value === 'card_shop' && !cardShopMode.value) {
    selectedTopupChannel.value = 'qr'
  } else if (selectedTopupChannel.value === 'qr' && !qrTopupAvailable.value && cardShopMode.value) {
    selectedTopupChannel.value = 'card_shop'
  } else if (cardShopMode.value) {
    selectedTopupChannel.value = 'card_shop'
  } else {
    selectedTopupChannel.value = 'qr'
  }
  syncPayTypeWithSettings()
}

function close() {
  if (step.value === 1 || qrExpired.value) {
    reset()
    emit('update:modelValue', false)
  }
}

function reset() {
  step.value = 1
  qrCodeURL.value = ''
  orderNo.value = ''
  qrExpired.value = false
  countdown.value = QR_TTL_SECONDS
  activeOrderAmountYuan.value = 0
  submitting.value = false
  stopTimers()
}

function updateActiveOrderMeta(meta: { amount_cny_fen?: number; pay_type?: TopupPayType; qr_code_url?: string | null }) {
  if (typeof meta.amount_cny_fen === 'number' && Number.isFinite(meta.amount_cny_fen) && meta.amount_cny_fen > 0) {
    activeOrderAmountYuan.value = meta.amount_cny_fen / 100
  }
  if (meta.pay_type === 'alipay' || meta.pay_type === 'wechat') {
    payType.value = meta.pay_type
  }
  if (typeof meta.qr_code_url === 'string' && meta.qr_code_url.trim() !== '') {
    qrCodeURL.value = meta.qr_code_url
  }
}

async function submitOrder() {
  if (!hasAvailablePayType.value || effectiveAmountYuan.value < 20 || amountError.value) return
  syncPayTypeWithSettings()
  submitting.value = true
  try {
    const orderAmountYuan = effectiveAmountYuan.value
    const res = await createTopupOrder(Math.round(orderAmountYuan * 100), payType.value)
    orderNo.value = res.order_no
    qrCodeURL.value = res.qr_code_url
    activeOrderAmountYuan.value = orderAmountYuan
    updateActiveOrderMeta(res)
    step.value = 2
    startCountdown()
    startPolling()
  } catch (e: any) {
    appStore.showError(extractApiErrorMessage(e, t('topup.payFailed')))
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
    countdown.value--
    if (countdown.value <= 0) {
      if (countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
      }
      void pollOrderStatus().finally(() => {
        if (step.value === 2 && orderNo.value) {
          qrExpired.value = true
          stopTimers()
        }
      })
    }
  }, 1000)
}

function startPolling() {
  void pollOrderStatus()
  pollTimer = setInterval(() => {
    void pollOrderStatus()
  }, 3000)
}

async function pollOrderStatus() {
  if (!orderNo.value || qrExpired.value || pollInFlight) return
  pollInFlight = true
  try {
    const res = await queryTopupOrderStatus(orderNo.value)
    updateActiveOrderMeta(res)
    if (res.status === 'completed') {
      stopTimers()
      emit('success')
      emit('update:modelValue', false)
      reset()
    } else if (res.status === 'expired') {
      qrExpired.value = true
      stopTimers()
    }
  } catch {
    // ignore polling errors
  } finally {
    pollInFlight = false
  }
}

function stopTimers() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
}

// Reset when modal closes
watch(() => props.modelValue, async (val) => {
  if (!val) {
    reset()
    return
  }
  await appStore.fetchPublicSettings()
  syncTopupChannelWithSettings()
})

onUnmounted(() => stopTimers())
</script>
