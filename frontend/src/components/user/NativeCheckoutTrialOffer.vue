<template>
  <section v-if="offer" class="trial-offer" aria-labelledby="native-checkout-trial-title">
    <div class="trial-offer__copy">
      <div class="trial-offer__badges">
        <span class="trial-offer__badge">{{ t('nativeCheckout.trialBadge') }}</span>
        <span v-if="offer.once_per_user" class="trial-offer__limit">{{ t('nativeCheckout.onceOnly') }}</span>
      </div>
      <h2 id="native-checkout-trial-title">{{ offer.name }}</h2>
      <p>{{ offer.description || t('nativeCheckout.trialDescription') }}</p>
      <p class="trial-offer__email">{{ t('nativeCheckout.registeredEmail') }}</p>
    </div>

    <div class="trial-offer__deal">
      <div class="trial-offer__amounts">
        <span>
          <small>{{ t('nativeCheckout.pay') }}</small>
          <strong>¥{{ formatCNY(offer.pay_amount_cny_fen) }}</strong>
        </span>
        <span class="trial-offer__arrow" aria-hidden="true">→</span>
        <span class="trial-offer__benefit">
          <small>{{ t('nativeCheckout.receive') }}</small>
          <strong>¥{{ formatCNY(offer.benefit_amount_cny_fen) }}</strong>
        </span>
      </div>

      <button
        type="button"
        class="trial-offer__action"
        :disabled="submitting || offer.claimed || order?.status === 'completed'"
        @click="startCheckout"
      >
        <span v-if="submitting">{{ t('nativeCheckout.preparing') }}</span>
        <span v-else-if="offer.claimed || order?.status === 'completed'">{{ t('nativeCheckout.claimed') }}</span>
        <span v-else-if="order?.status === 'checking' || order?.status === 'manual_review'">{{ t('nativeCheckout.reviewing') }}</span>
        <span v-else-if="order?.status === 'pending'">{{ t('nativeCheckout.continuePayment') }}</span>
        <span v-else-if="order?.status === 'fulfilling'">{{ t('nativeCheckout.crediting') }}</span>
        <span v-else>{{ t('nativeCheckout.buyNow') }}</span>
      </button>
      <p v-if="order?.status === 'checking' || order?.status === 'manual_review'" class="trial-offer__checking">
        {{ t('nativeCheckout.checkingHint') }}
      </p>
      <p v-else-if="offer.claimed || order?.status === 'completed'" class="trial-offer__success">
        {{ t('nativeCheckout.completedHint', { amount: formatCNY(offer.benefit_amount_cny_fen) }) }}
      </p>
    </div>
  </section>

  <Teleport to="body">
    <div v-if="showModal" class="checkout-modal" @click.self="closeModal">
      <section class="checkout-modal__card" role="dialog" aria-modal="true" aria-labelledby="native-checkout-modal-title">
        <header class="checkout-modal__header">
          <div>
            <p>{{ isChecking ? t('nativeCheckout.orderProgress') : paymentMethodTitle }}</p>
            <h2 id="native-checkout-modal-title">{{ modalTitle }}</h2>
          </div>
          <button type="button" class="checkout-modal__close" :aria-label="t('common.close')" @click="closeModal">×</button>
        </header>

        <div class="checkout-modal__body">
          <div v-if="isChecking" class="checkout-modal__qr checkout-modal__checking" aria-hidden="true">
            <span>•••</span>
          </div>
          <div v-else class="checkout-modal__qr">
            <img v-if="qrImageURL" :src="qrImageURL" :alt="paymentQRAlt" />
            <div v-else class="checkout-modal__loading">{{ t('nativeCheckout.loadingQR') }}</div>
          </div>
          <p v-if="qrImageURL" class="checkout-modal__kind">
            {{ paymentScanInstruction }}
          </p>
          <p v-if="qrKind === 'payment_link'" class="checkout-modal__kind checkout-modal__kind--notice">
            {{ t('nativeCheckout.linkQRHint') }}
          </p>
          <div v-if="activeOrder?.status === 'pending' && activeOrder.created_at" class="checkout-modal__timing">
            <p>{{ t('nativeCheckout.orderCreatedAt', { time: orderCreatedAtText }) }}</p>
            <p :class="{ 'checkout-modal__timing-expired': paymentWindowExpired }">
              {{ paymentWindowText }}
            </p>
            <p>{{ t('nativeCheckout.providerExpiryHint') }}</p>
          </div>
          <p class="checkout-modal__status">
            <span class="checkout-modal__pulse" aria-hidden="true"></span>
            {{ statusText }}
          </p>
          <p class="checkout-modal__eta">{{ t('nativeCheckout.automaticEta') }}</p>
          <p class="checkout-modal__safety">{{ t('nativeCheckout.doNotRepeat') }}</p>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import {
  createNativeCheckoutOrder,
  getNativeCheckoutDirectQR,
  getNativeCheckoutOrder,
  listNativeCheckoutOffers,
  type NativeCheckoutOffer,
  type NativeCheckoutOrder,
} from '@/api/nativeCheckout'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const offer = ref<NativeCheckoutOffer | null>(null)
const order = ref<NativeCheckoutOrder | null>(null)
const activeOrder = computed(() => order.value)
const submitting = ref(false)
const showModal = ref(false)
const qrImageURL = ref('')
const qrKind = ref<'direct' | 'payment_link' | ''>('')
const clockNow = ref(Date.now())
let pollTimer: ReturnType<typeof setTimeout> | null = null
let clockTimer: ReturnType<typeof setInterval> | null = null
let pollInFlight = false
let objectURL = ''
let completionNotified = false
let checkingNotified = false
let disposed = false

const PENDING_POLL_MS = 3000
const CHECKING_POLL_MS = 15000
const RECOMMENDED_PAYMENT_WINDOW_MS = 10 * 60 * 1000

const isChecking = computed(() => (
  activeOrder.value?.status === 'checking'
  || activeOrder.value?.status === 'fulfilling'
  || activeOrder.value?.status === 'manual_review'
))

const modalTitle = computed(() => {
  if (isChecking.value) return t('nativeCheckout.orderCheckingTitle')
  if (activeOrder.value?.status === 'completed') return t('nativeCheckout.paymentCompleted')
  return t('nativeCheckout.scanToPay', { amount: formatCNY(activeOrder.value?.pay_amount_cny_fen || 0) })
})

const orderCreatedAtText = computed(() => {
  const value = activeOrder.value?.created_at
  if (!value) return ''
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(parsed)
})

const paymentWindowRemainingSeconds = computed(() => {
  const value = activeOrder.value?.created_at
  if (!value) return null
  const createdAt = Date.parse(value)
  if (!Number.isFinite(createdAt)) return null
  return Math.max(0, Math.ceil((createdAt + RECOMMENDED_PAYMENT_WINDOW_MS - clockNow.value) / 1000))
})

const paymentWindowExpired = computed(() => paymentWindowRemainingSeconds.value === 0)

const paymentWindowText = computed(() => {
  const remaining = paymentWindowRemainingSeconds.value
  if (remaining === null || remaining === 0) return t('nativeCheckout.recommendedWindowExpired')
  const minutes = Math.floor(remaining / 60).toString().padStart(2, '0')
  const seconds = (remaining % 60).toString().padStart(2, '0')
  return t('nativeCheckout.recommendedWindow', { time: `${minutes}:${seconds}` })
})

const statusText = computed(() => {
  switch (activeOrder.value?.status) {
    case 'creating': return t('nativeCheckout.creatingOrder')
    case 'checking': return t('nativeCheckout.checkingHint')
    case 'fulfilling': return t('nativeCheckout.creditingHint')
    case 'completed': return t('nativeCheckout.paymentCompleted')
    case 'manual_review': return t('nativeCheckout.checkingHint')
    default: return t('nativeCheckout.waitingPayment')
  }
})

const paymentMethodTitle = computed(() => {
  switch (activeOrder.value?.payment_method) {
    case 'wechat': return t('nativeCheckout.wechatPay')
    case 'alipay': return t('nativeCheckout.alipayPay')
    default: return t('nativeCheckout.qrPayment')
  }
})

const paymentQRAlt = computed(() => {
  switch (activeOrder.value?.payment_method) {
    case 'wechat': return t('nativeCheckout.wechatQRAlt')
    case 'alipay': return t('nativeCheckout.alipayQRAlt')
    default: return t('nativeCheckout.paymentQRAlt')
  }
})

const paymentScanInstruction = computed(() => {
  switch (activeOrder.value?.payment_method) {
    case 'wechat': return t('nativeCheckout.wechatScanInstruction')
    case 'alipay': return t('nativeCheckout.alipayScanInstruction')
    default: return t('nativeCheckout.scanInstruction')
  }
})

function formatCNY(fen: number): string {
  const yuan = fen / 100
  return Number.isInteger(yuan) ? String(yuan) : yuan.toFixed(2)
}

async function loadOffer() {
  try {
    const offers = await listNativeCheckoutOffers()
    offer.value = offers.find((item) => item.code === 'trial-balance-1-to-5') ?? null
    order.value = offer.value?.order ?? null
    if (shouldPollStatus()) {
      startPolling()
    }
  } catch (error) {
    // The promotion is optional. A temporary provider/API failure must not
    // break the rest of the recharge page.
    console.error('Failed to load native checkout offer:', error)
  }
}

async function startCheckout() {
  if (!offer.value || submitting.value) return
  if (offer.value.claimed || order.value?.status === 'completed') return
  if (order.value?.status === 'checking' || order.value?.status === 'manual_review') {
    clearQRImage()
    showModal.value = true
    startPolling()
    return
  }
  submitting.value = true
  try {
    if (!order.value || order.value.status === 'failed') {
      order.value = await createNativeCheckoutOrder(offer.value.code)
    }
    if (['creating', 'pending', 'checking', 'fulfilling'].includes(order.value.status)) {
      showModal.value = true
    }
    if (order.value.status === 'pending') {
      await loadPaymentQR(order.value)
    }
    if (['creating', 'pending', 'checking', 'fulfilling'].includes(order.value.status)) {
      startPolling()
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('nativeCheckout.createFailed')))
  } finally {
    submitting.value = false
  }
}

async function loadPaymentQR(current: NativeCheckoutOrder) {
  clearQRImage()
  try {
    const blob = await getNativeCheckoutDirectQR(current.order_no)
    objectURL = URL.createObjectURL(blob)
    qrImageURL.value = objectURL
    qrKind.value = 'direct'
    return
  } catch {
    // A provider challenge page has no static payable QR. Fall back to a QR
    // containing the ordinary payment link and label it honestly in the UI.
  }
  if (!current.payment_url) {
    appStore.showError(t('nativeCheckout.qrUnavailable'))
    return
  }
  qrImageURL.value = await QRCode.toDataURL(current.payment_url, {
    errorCorrectionLevel: 'M',
    margin: 2,
    width: 320,
  })
  qrKind.value = 'payment_link'
}

function startPolling(delay = 0) {
  if (disposed || pollTimer || pollInFlight) return
  // This polls only our server's durable order row. The browser never polls
  // LDXP directly; a leased server worker owns payment and delivery checks.
  pollTimer = setTimeout(() => {
    pollTimer = null
    void pollStatus()
  }, delay)
}

function shouldPollStatus(): boolean {
  const status = order.value?.status
  if (!status || !['creating', 'pending', 'checking', 'fulfilling', 'manual_review'].includes(status)) return false
  if ((status === 'creating' || status === 'pending') && !showModal.value) return false
  return true
}

function nextPollDelay(): number {
  if (order.value?.status === 'manual_review') return CHECKING_POLL_MS
  if (order.value?.status === 'pending' && paymentWindowExpired.value) return CHECKING_POLL_MS
  return PENDING_POLL_MS
}

async function pollStatus() {
  if (!order.value || pollInFlight) return
  pollInFlight = true
  try {
    const previousStatus = order.value.status
    order.value = await getNativeCheckoutOrder(order.value.order_no)
    if (order.value.status === 'pending' && previousStatus === 'creating' && showModal.value) {
      await loadPaymentQR(order.value)
    }
    if (order.value.status === 'completed') {
      stopPolling()
      showModal.value = false
      clearQRImage()
      if (!completionNotified) {
        completionNotified = true
        try {
          await authStore.refreshUser()
        } catch (error) {
          console.error('Failed to refresh user after native checkout:', error)
        }
        appStore.showSuccess(t('nativeCheckout.completedToast', {
          amount: formatCNY(order.value.benefit_amount_cny_fen),
        }))
      }
    } else if (order.value.status === 'checking' || order.value.status === 'manual_review') {
      clearQRImage()
      if (!checkingNotified) {
        checkingNotified = true
        appStore.showInfo(t('nativeCheckout.checkingHint'))
      }
    } else if (order.value.status === 'failed') {
      stopPolling()
      showModal.value = false
      clearQRImage()
    }
  } catch (error) {
    console.error('Failed to query native checkout status:', error)
  } finally {
    pollInFlight = false
    if (shouldPollStatus()) {
      startPolling(nextPollDelay())
    }
  }
}

function stopPolling() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
}

function clearQRImage() {
  if (objectURL) {
    URL.revokeObjectURL(objectURL)
    objectURL = ''
  }
  qrImageURL.value = ''
  qrKind.value = ''
}

function closeModal() {
  showModal.value = false
  clearQRImage()
  if (order.value?.status === 'creating' || order.value?.status === 'pending') stopPolling()
}

onMounted(() => {
  clockTimer = setInterval(() => { clockNow.value = Date.now() }, 1000)
  void loadOffer()
})
onUnmounted(() => {
  disposed = true
  stopPolling()
  if (clockTimer) clearInterval(clockTimer)
  clearQRImage()
})
</script>

<style scoped>
.trial-offer {
  display: grid;
  gap: 1.25rem;
  margin-top: 1.75rem;
  border: 1px solid rgb(var(--color-terracotta) / 0.34);
  border-radius: 10px;
  padding: 1.25rem;
  background:
    radial-gradient(circle at 100% 0, rgb(var(--color-terracotta) / 0.13), transparent 44%),
    var(--admin-control, rgb(var(--color-vellum) / 0.95));
  box-shadow: inset 0 0 0 1px rgb(var(--color-terracotta) / 0.05);
}

.trial-offer__badges { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
.trial-offer__badge,
.trial-offer__limit {
  display: inline-flex;
  border-radius: 999px;
  padding: 0.25rem 0.6rem;
  font-size: 0.72rem;
  font-weight: 750;
}
.trial-offer__badge { color: white; background: rgb(var(--color-terracotta)); }
.trial-offer__limit { color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark))); background: rgb(var(--color-terracotta) / 0.1); }
.trial-offer h2 { margin-top: 0.75rem; color: var(--admin-ink-deep, rgb(var(--color-ink-deep))); font-size: 1.25rem; font-weight: 750; }
.trial-offer__copy > p { margin-top: 0.45rem; color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.87rem; line-height: 1.65; }
.trial-offer__email { opacity: 0.86; }
.trial-offer__deal { min-width: 0; }
.trial-offer__amounts { display: flex; align-items: center; justify-content: space-between; gap: 0.65rem; }
.trial-offer__amounts span:not(.trial-offer__arrow) { display: grid; gap: 0.15rem; }
.trial-offer__amounts small { color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.72rem; }
.trial-offer__amounts strong { color: var(--admin-ink-deep, rgb(var(--color-ink-deep))); font-size: 1.5rem; }
.trial-offer__benefit strong { color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark))); }
.trial-offer__arrow { color: var(--admin-muted, rgb(var(--color-muted))); }
.trial-offer__action {
  width: 100%; margin-top: 0.85rem; border-radius: 7px; padding: 0.8rem 1rem;
  background: rgb(var(--color-terracotta)); color: #fff; font-weight: 750;
  transition: transform 160ms ease, opacity 160ms ease;
}
.trial-offer__action:hover:not(:disabled) { transform: translateY(-1px); }
.trial-offer__action:disabled { cursor: not-allowed; opacity: 0.62; }
.trial-offer__checking,
.trial-offer__success { margin-top: 0.65rem; font-size: 0.78rem; line-height: 1.5; }
.trial-offer__checking { color: var(--admin-muted, rgb(var(--color-muted))); }
.trial-offer__success { color: var(--admin-laurel-dark, rgb(var(--color-laurel-dark))); }

.checkout-modal {
  position: fixed; inset: 0; z-index: 80; display: grid; place-items: center;
  padding: 1rem; background: rgb(15 23 42 / 0.58); backdrop-filter: blur(4px);
}
.checkout-modal__card {
  width: min(27rem, 100%); overflow: hidden; border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 12px; background: var(--admin-surface, rgb(var(--color-marble) / 0.98)); color: var(--admin-ink, rgb(var(--color-ink)));
  box-shadow: 0 28px 80px rgb(0 0 0 / 0.28);
}
.checkout-modal__header { display: flex; justify-content: space-between; gap: 1rem; padding: 1.2rem 1.25rem; border-bottom: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14)); }
.checkout-modal__header p { color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark))); font-size: 0.72rem; font-weight: 750; letter-spacing: 0.08em; text-transform: uppercase; }
.checkout-modal__header h2 { margin-top: 0.25rem; color: var(--admin-ink-deep, rgb(var(--color-ink-deep))); font-size: 1.2rem; font-weight: 750; }
.checkout-modal__close { width: 2rem; height: 2rem; border-radius: 6px; color: var(--admin-muted, rgb(var(--color-muted))); font-size: 1.5rem; line-height: 1; }
.checkout-modal__body { display: grid; justify-items: center; padding: 1.4rem; text-align: center; }
.checkout-modal__qr { display: grid; place-items: center; width: 18rem; max-width: 100%; aspect-ratio: 1; border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14)); border-radius: 10px; padding: 0.75rem; background: white; }
.checkout-modal__qr img { width: 100%; height: 100%; object-fit: contain; image-rendering: crisp-edges; }
.checkout-modal__checking {
  width: 7rem; border: 0; border-radius: 999px; padding: 0;
  background: rgb(var(--color-laurel) / 0.09); color: rgb(var(--color-laurel-dark));
  font-size: 1.65rem; font-weight: 800; letter-spacing: 0.18em;
}
.checkout-modal__checking span { animation: checkout-checking 1.4s ease-in-out infinite; }
.checkout-modal__loading { color: #64748b; font-size: 0.85rem; }
.checkout-modal__kind { margin-top: 0.85rem; color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.78rem; line-height: 1.5; }
.checkout-modal__kind--notice { color: rgb(180 83 9); }
.checkout-modal__timing { width: 100%; margin-top: 0.9rem; border-radius: 8px; padding: 0.75rem 0.85rem; background: rgb(var(--color-ink) / 0.045); color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.74rem; line-height: 1.55; }
.checkout-modal__timing p + p { margin-top: 0.18rem; }
.checkout-modal__timing p:nth-child(2) { color: var(--admin-ink-deep, rgb(var(--color-ink-deep))); font-size: 0.82rem; font-weight: 720; }
.checkout-modal__timing .checkout-modal__timing-expired { color: rgb(180 83 9); }
.checkout-modal__status { display: flex; align-items: center; gap: 0.45rem; margin-top: 1rem; font-size: 0.88rem; font-weight: 650; }
.checkout-modal__pulse { width: 0.5rem; height: 0.5rem; border-radius: 50%; background: rgb(var(--color-laurel)); box-shadow: 0 0 0 0 rgb(var(--color-laurel) / 0.35); animation: checkout-pulse 1.6s infinite; }
.checkout-modal__eta { margin-top: 0.55rem; color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.76rem; line-height: 1.5; }
.checkout-modal__safety { margin-top: 0.55rem; color: var(--admin-muted, rgb(var(--color-muted))); font-size: 0.75rem; }
@keyframes checkout-pulse { 70% { box-shadow: 0 0 0 7px rgb(var(--color-laurel) / 0); } 100% { box-shadow: 0 0 0 0 rgb(var(--color-laurel) / 0); } }
@keyframes checkout-checking { 50% { opacity: 0.35; transform: translateY(-2px); } }

@media (min-width: 720px) {
  .trial-offer { grid-template-columns: minmax(0, 1fr) minmax(14rem, 0.72fr); align-items: center; }
}

@media (prefers-reduced-motion: reduce) {
  .checkout-modal__pulse { animation: none; }
  .checkout-modal__checking span { animation: none; }
  .trial-offer__action { transition: none; }
}
</style>
