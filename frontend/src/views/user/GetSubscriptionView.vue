<template>
  <AppLayout>
    <div class="topup-page">
      <div class="topup-shell">
        <section class="topup-panel" data-tour="topup-panel">
          <div class="topup-grid">
            <div class="topup-main">
              <div class="topup-eyebrow">
                {{ t('subscriptionAccess.badge') }}
              </div>

              <h1 class="topup-heading">
                {{ t('subscriptionAccess.heading') }}
              </h1>
              <p class="topup-copy">
                {{ t('subscriptionAccess.summary') }}
              </p>

              <div v-if="step === 1" class="topup-section topup-section--stack">
                <section>
                  <p class="topup-label">
                    {{ t('topup.balanceCardsTitle') }}
                  </p>
                  <div class="topup-products">
                    <button
                      v-for="product in balanceProducts"
                      :key="product.id"
                      type="button"
                      @click="selectBalanceProduct(product)"
                      class="topup-product"
                      :class="{ 'topup-product--active': selectedProductKind === 'balance' && selectedBalanceProduct?.id === product.id }"
                    >
                      <span class="topup-product-title">
                        {{ product.label }}
                      </span>
                      <span class="topup-product-desc">
                        {{ t('topup.cardShopAmount', { amount: product.amountCny }) }}
                      </span>
                    </button>
                  </div>
                </section>

                <section>
                  <p class="topup-label">
                    {{ t('topup.developerPlansTitle') }}
                  </p>
                  <div class="topup-plan-grid">
                    <button
                      v-for="plan in monthlyCreditCardPlans"
                      :key="plan.id"
                      type="button"
                      @click="selectMonthlyPlan(plan)"
                      class="topup-monthly-product"
                      :class="[
                        `topup-monthly-product--${plan.accent}`,
                        { 'topup-monthly-product--active': selectedProductKind === 'monthly' && selectedMonthlyPlan?.id === plan.id }
                      ]"
                    >
                      <span class="topup-monthly-product__head">
                        <span class="topup-product-title">{{ plan.name }}</span>
                        <span class="topup-status topup-status--available">{{ t('topup.monthlyPlanStatus') }}</span>
                      </span>
                      <span class="topup-monthly-product__price">{{ plan.price }} <small>/ 月</small></span>
                    </button>
                  </div>
                </section>

                <section v-if="selectedProductKind === 'balance'">
                  <p class="topup-label">
                    {{ t('topup.selectPayType') }}
                  </p>
                  <div class="topup-channel-grid">
                    <button
                      v-if="cardShopMode"
                      type="button"
                      @click="selectTopupChannel('card_shop')"
                      :disabled="!canUseCardShopForSelected"
                      class="topup-choice"
                      :class="{ 'topup-choice--active': showingCardShop }"
                    >
                      <span class="topup-choice-icon topup-choice-icon--warm">
                        <Icon name="gift" size="md" />
                      </span>
                      <span class="topup-choice-content">
                        <span class="topup-choice-title">{{ t('topup.cardShopChannelTitle') }}</span>
                        <span class="topup-choice-desc">{{ t('topup.cardShopChannelDesc') }}</span>
                        <span
                          class="topup-status"
                          :class="canUseCardShopForSelected ? 'topup-status--available' : 'topup-status--disabled'"
                        >
                          {{ canUseCardShopForSelected ? t('topup.availableNow') : t('topup.comingSoon') }}
                        </span>
                      </span>
                    </button>

                    <button
                      type="button"
                      @click="selectTopupChannel('qr')"
                      :disabled="!qrPaymentAvailableForSelected"
                      class="topup-choice"
                      :class="{ 'topup-choice--active': showingQrTopup }"
                    >
                      <span class="topup-choice-icon topup-choice-icon--laurel">
                        <Icon name="creditCard" size="md" />
                      </span>
                      <span class="topup-choice-content">
                        <span class="topup-choice-title">{{ t('topup.qrChannelTitle') }}</span>
                        <span class="topup-choice-desc">{{ t('topup.qrChannelDesc') }}</span>
                        <span
                          class="topup-status"
                          :class="qrPaymentAvailableForSelected ? 'topup-status--available' : 'topup-status--disabled'"
                        >
                          {{ qrPaymentAvailableForSelected ? t('topup.availableNow') : t('topup.comingSoon') }}
                        </span>
                      </span>
                    </button>
                  </div>
                  <p v-if="paymentNotice" class="topup-warning">{{ paymentNotice }}</p>
                </section>

              </div>

              <div v-else-if="step === 2 && showingQrTopup" class="topup-section topup-qr-panel">
                <p class="topup-copy topup-copy--compact">
                  {{ t('topup.scanHint', { payType: payType === 'alipay' ? t('topup.alipay') : t('topup.wechat') }) }}
                </p>
                <div class="topup-qr-wrap">
                  <div class="topup-qr-box">
                    <img
                      v-if="qrCodeURL && !qrExpired"
                      :src="qrCodeURL"
                      alt="Payment QR Code"
                      class="topup-qr-image"
                    />
                    <div v-else class="topup-qr-empty">
                      <p>{{ t('topup.qrExpired') }}</p>
                      <button @click="resubmitOrder" class="btn btn-secondary btn-sm">{{ t('topup.refresh') }}</button>
                    </div>
                  </div>
                </div>
                <div class="topup-amount-row">
                  <div>
                    <p class="topup-meta-label">{{ t('topup.selectAmount') }}</p>
                    <p class="topup-meta-value">¥{{ displayAmountText }}</p>
                  </div>
                  <div>
                    <p class="topup-meta-label">USD</p>
                    <p class="topup-meta-value topup-meta-value--accent">${{ displayUSDText }}</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="topup-summary">
              <div class="topup-summary-card">
                <div class="topup-summary-head">
                  <div>
                    <p class="topup-summary-kicker">{{ t('topup.title') }}</p>
                    <p class="topup-summary-title">
                      <template v-if="selectedProductKind === 'monthly'">
                        {{ selectedMonthlyPlan?.name }}
                      </template>
                      <template v-else>
                        {{ selectedBalanceProduct?.label }}
                      </template>
                    </p>
                  </div>
                  <div v-if="showingQrTopup" class="topup-price-chip">
                    ${{ displayUSDText }}
                  </div>
                  <div v-else-if="selectedProductKind === 'monthly'" class="topup-price-chip topup-price-chip--muted">
                    {{ t('topup.monthlyPlanStatus') }}
                  </div>
                </div>

                <div class="topup-summary-rows">
                  <template v-if="selectedProductKind === 'monthly'">
                    <div class="topup-summary-row">
                      <span>{{ t('topup.productType') }}</span>
                      <strong>{{ t('topup.developerMonthlyCard') }}</strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.monthlyPlanPrice') }}</span>
                      <strong>{{ selectedMonthlyPlan?.price }} / 月</strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.monthlyPlanDirectPrice') }}</span>
                      <strong>{{ selectedMonthlyPlan?.directPrice }} / 月</strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.monthlyPlanWeeklyLimit') }}</span>
                      <strong class="topup-summary-limit">
                        <span>{{ selectedMonthlyPlan?.displayWeeklyCreditsText }} AI credits / 周</span>
                        <small>
                          GPT Pro {{ selectedMonthlyPlan?.gptWeeklyUsage }} · Claude Max {{ selectedMonthlyPlan?.claudeWeeklyUsage }}
                        </small>
                      </strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.monthlyPlanMonthlyLimit') }}</span>
                      <strong class="topup-summary-limit">
                        <span>{{ selectedMonthlyPlan?.displayMonthlyCreditsText }} AI credits / 月</span>
                        <small>
                          GPT Pro {{ selectedMonthlyPlan?.gptMonthlyUsage }} · Claude Max {{ selectedMonthlyPlan?.claudeMonthlyUsage }}
                        </small>
                      </strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.monthlyPlanQuotaMode') }}</span>
                      <strong>{{ t('topup.monthlyPlanSharedPool') }}</strong>
                    </div>
                  </template>
                  <template v-else>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.productType') }}</span>
                      <strong>{{ t('topup.balanceCard') }}</strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.selectAmount') }}</span>
                      <strong>¥{{ displayAmountText }}</strong>
                    </div>
                    <div class="topup-summary-row">
                      <span>{{ t('topup.selectPayType') }}</span>
                      <strong>{{ selectedTopupMethodLabel }}</strong>
                    </div>
                    <div v-if="showingQrTopup" class="topup-summary-row">
                      <span>{{ t('subscriptionAccess.rateLabel') }}</span>
                      <strong>{{ t('topup.creditsNote') }}</strong>
                    </div>
                  </template>
                  <div v-if="step === 2 && !qrExpired" class="topup-summary-row">
                    <span>{{ t('topup.qrExpiry') }}</span>
                    <strong>{{ countdownText }}</strong>
                  </div>
                </div>

                <div v-if="selectedProductKind === 'monthly'" class="topup-monthly-actions">
                  <button
                    @click="openSelectedMonthlyCardShop"
                    :disabled="!canOpenSelectedMonthlyCardShop"
                    class="topup-primary-action topup-monthly-action"
                  >
                    {{ t('topup.monthlyCardShopAction') }}
                  </button>
                  <button
                    @click="openMonthlyDirectPurchase"
                    class="topup-secondary-action topup-monthly-action"
                  >
                    {{ t('topup.monthlyDirectAction') }}
                  </button>
                </div>

                <template v-else-if="showingCardShop">
                  <button
                    @click="openSelectedCardShopProduct"
                    :disabled="!selectedCardShopProduct"
                    class="topup-primary-action"
                  >
                    {{ t('topup.cardShopAction') }}
                  </button>
                  <button
                    @click="goRedeem"
                    class="topup-secondary-action topup-secondary-action--stacked"
                  >
                    {{ t('topup.cardShopGoRedeem') }}
                  </button>
                </template>

                <section v-else-if="step === 1" class="topup-inline-pay">
                  <p class="topup-label">
                    {{ t('topup.selectPayType') }}
                  </p>
                  <div
                    v-if="hasAvailablePayType"
                    class="topup-pay-grid"
                    :class="{ 'topup-pay-grid--single': !hasMultiplePayTypes }"
                  >
                    <button
                      v-if="canUseAlipay"
                      @click="selectPayType('alipay')"
                      class="topup-pay-option"
                      :class="{ 'topup-pay-option--active': payType === 'alipay' }"
                    >
                      <div>
                        <span class="topup-choice-title">{{ t('topup.alipay') }}</span>
                      </div>
                      <span class="topup-choice-icon topup-choice-icon--laurel">
                        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M21.422 15.358c-3.83-1.153-6.055-1.84-6.055-1.84.598-1.163.959-2.478 1.028-3.869H20V8.5h-4.5V7h-1.75v1.5h-4.5v1.149h7.397c-.106 2.73-1.5 4.872-3.5 6.114C11.47 17.21 9.48 16.5 7.5 16.5c-2.76 0-5 2.24-5 5 0 .17.01.34.03.5H2.5v.5h19v-.5h-.077A10.47 10.47 0 0022 20c0-1.9-.214-3.375-.578-4.642zM7.5 20c-1.38 0-2.5-1.12-2.5-2.5S6.12 15 7.5 15s2.5 1.12 2.5 2.5S8.88 20 7.5 20z"/>
                        </svg>
                      </span>
                    </button>

                    <button
                      v-if="canUseWechat"
                      @click="selectPayType('wechat')"
                      class="topup-pay-option"
                      :class="{ 'topup-pay-option--active': payType === 'wechat' }"
                    >
                      <div>
                        <span class="topup-choice-title">{{ t('topup.wechat') }}</span>
                      </div>
                      <span class="topup-choice-icon topup-choice-icon--laurel">
                        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M8.667 12c0 .58.47 1.05 1.05 1.05S10.767 12.58 10.767 12s-.47-1.05-1.05-1.05S8.667 11.42 8.667 12zm5.666 0c0 .58.47 1.05 1.05 1.05s1.05-.47 1.05-1.05-.47-1.05-1.05-1.05-1.05.47-1.05 1.05zM12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8z"/>
                        </svg>
                      </span>
                    </button>
                  </div>
                  <p v-else class="topup-warning">
                    {{ t('topup.noAvailablePayType') }}
                  </p>
                  <button
                    @click="submitOrder"
                    :disabled="submitting || !hasAvailablePayType || !!amountError || effectiveAmountYuan < 20"
                    class="topup-primary-action"
                  >
                    <span v-if="submitting">{{ t('topup.submitting') }}</span>
                    <span v-else>{{ t('topup.confirmButton') }} — ¥{{ effectiveAmountYuan || 20 }}</span>
                  </button>
                </section>

                <div v-else class="topup-summary-actions">
                  <div class="topup-success">
                    {{ t('topup.waitingPayment') }}
                  </div>
                  <button
                    @click="resetToForm"
                    class="topup-secondary-action"
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
    <div v-if="showMonthlyDirectPurchase" class="topup-modal-backdrop" @click.self="closeMonthlyDirectPurchase">
      <section class="topup-direct-modal" role="dialog" aria-modal="true">
        <div class="topup-direct-modal__head">
          <div>
            <p class="topup-summary-kicker">{{ t('topup.monthlyDirectAction') }}</p>
            <h2>{{ selectedMonthlyPlan?.name }}</h2>
          </div>
          <button type="button" class="topup-modal-close" @click="closeMonthlyDirectPurchase">
            ×
          </button>
        </div>
        <div v-if="monthlyDirectPurchaseQRCode" class="topup-direct-qr">
          <img :src="monthlyDirectPurchaseQRCode" alt="Monthly card support group QR code" />
        </div>
        <div v-else class="topup-direct-qr topup-direct-qr--empty">
          {{ t('topup.monthlyDirectNoQr') }}
        </div>
        <p class="topup-direct-copy">
          {{ t('topup.monthlyDirectInstruction', { plan: selectedMonthlyPlan?.name }) }}
        </p>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { createTopupOrder, queryTopupOrderStatus, type TopupPayType } from '@/api/topup'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CardShopProduct } from '@/types'
import { monthlyCreditCardPlans, type MonthlyCreditCardPlan } from '@/constants/monthlyCreditCards'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const presets = [20, 50, 100, 200, 1000, 2000]
const QR_TTL_SECONDS = 300
type TopupChannel = 'card_shop' | 'qr'
type SelectedProductKind = 'balance' | 'monthly'
type BalanceProduct = {
  id: string
  label: string
  amountCny: number
  cardShopProduct?: CardShopProduct
}

const step = ref<1 | 2>(1)
const selectedTopupChannel = ref<TopupChannel>('card_shop')
const selectedProductKind = ref<SelectedProductKind>('balance')
const selectedBalanceProductId = ref('')
const selectedMonthlyPlanId = ref<MonthlyCreditCardPlan['id']>('lite')
const payType = ref<TopupPayType>('alipay')
const submitting = ref(false)
const qrCodeURL = ref('')
const orderNo = ref('')
const qrExpired = ref(false)
const countdown = ref(QR_TTL_SECONDS)
const activeOrderAmountYuan = ref(0)
const showMonthlyDirectPurchase = ref(false)

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
const hasMultiplePayTypes = computed(() => canUseAlipay.value && canUseWechat.value)
const activeCardShopProducts = computed<CardShopProduct[]>(() =>
  [...(appStore.cachedPublicSettings?.card_shop_products ?? [])]
    .filter((product) => product.enabled && product.url && product.amount_cny > 0)
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.amount_cny - b.amount_cny)
)
const cardShopMode = computed(
  () => (appStore.cachedPublicSettings?.card_shop_enabled ?? false) && activeCardShopProducts.value.length > 0
)
const balanceProducts = computed<BalanceProduct[]>(() => {
  if (activeCardShopProducts.value.length > 0) {
    return activeCardShopProducts.value.map((product) => ({
      id: product.id,
      label: product.label || `¥${product.amount_cny} 余额卡`,
      amountCny: product.amount_cny,
      cardShopProduct: product
    }))
  }
  return presets.map((amount) => ({
    id: `qr-${amount}`,
    label: `¥${amount} 余额卡`,
    amountCny: amount
  }))
})
const selectedBalanceProduct = computed<BalanceProduct | undefined>(
  () => balanceProducts.value.find((product) => product.id === selectedBalanceProductId.value) ?? balanceProducts.value[0]
)
const selectedCardShopProduct = computed(() => selectedBalanceProduct.value?.cardShopProduct)
const selectedMonthlyPlan = computed(
  () => monthlyCreditCardPlans.find((plan) => plan.id === selectedMonthlyPlanId.value) ?? monthlyCreditCardPlans[0]
)
const selectedMonthlyCardShopUrl = computed(() => selectedMonthlyPlan.value?.cardShopUrl || '')
const canOpenSelectedMonthlyCardShop = computed(() => selectedMonthlyCardShopUrl.value.trim() !== '')
const monthlyDirectPurchaseQRCode = computed(
  () =>
    (appStore.cachedPublicSettings?.after_sales_qrcode || appStore.cachedPublicSettings?.tech_support_qrcode || '').trim()
)
const canUseCardShopForSelected = computed(() => cardShopMode.value && !!selectedCardShopProduct.value)
const qrPaymentAvailableForSelected = computed(
  () => selectedProductKind.value === 'balance' && qrTopupAvailable.value && effectiveAmountYuan.value >= 20
)
const showingCardShop = computed(
  () => selectedProductKind.value === 'balance' && selectedTopupChannel.value === 'card_shop' && canUseCardShopForSelected.value
)
const showingQrTopup = computed(() => selectedProductKind.value === 'balance' && selectedTopupChannel.value === 'qr')
const selectedTopupMethodLabel = computed(() => {
  if (showingCardShop.value) return t('topup.cardShopChannelTitle')
  if (showingQrTopup.value) return t('topup.qrChannelTitle')
  return t('topup.comingSoon')
})

const effectiveAmountYuan = computed<number>(() => {
  if (selectedProductKind.value !== 'balance') return 0
  return selectedBalanceProduct.value?.amountCny ?? 0
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
  if (selectedProductKind.value !== 'balance' || selectedTopupChannel.value !== 'qr') return ''
  if (effectiveAmountYuan.value > 0 && effectiveAmountYuan.value < 20) {
    return t('topup.minAmountError')
  }
  if (effectiveAmountYuan.value > 3000) {
    return t('topup.maxAmountError')
  }
  return ''
})
const paymentNotice = computed(() => {
  if (selectedProductKind.value !== 'balance') return ''
  if (selectedTopupChannel.value === 'qr' && amountError.value) return amountError.value
  if (selectedTopupChannel.value === 'qr' && !qrTopupAvailable.value) return t('topup.noAvailablePayType')
  if (selectedTopupChannel.value === 'card_shop' && !canUseCardShopForSelected.value) return t('topup.cardShopUnavailable')
  return ''
})

const countdownText = computed(() => {
  const minutes = Math.floor(countdown.value / 60)
  const seconds = countdown.value % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
})

function formatMoney(value: number, fixed: boolean) {
  if (!Number.isFinite(value) || value <= 0) return fixed ? '20.00' : '20'
  return fixed || !Number.isInteger(value) ? value.toFixed(2) : String(value)
}

function selectPayType(type: TopupPayType) {
  if (type === 'alipay' && !canUseAlipay.value) return
  if (type === 'wechat' && !canUseWechat.value) return
  payType.value = type
}

function selectBalanceProduct(product: BalanceProduct) {
  selectedProductKind.value = 'balance'
  selectedBalanceProductId.value = product.id
  if (selectedTopupChannel.value === 'card_shop' && !product.cardShopProduct && qrTopupAvailable.value && product.amountCny >= 20) {
    selectedTopupChannel.value = 'qr'
  } else if (selectedTopupChannel.value === 'qr' && (!qrTopupAvailable.value || product.amountCny < 20) && product.cardShopProduct) {
    selectedTopupChannel.value = 'card_shop'
  }
}

function selectMonthlyPlan(plan: MonthlyCreditCardPlan) {
  selectedProductKind.value = 'monthly'
  selectedMonthlyPlanId.value = plan.id
}

function selectTopupChannel(channel: TopupChannel) {
  if (channel === 'card_shop' && !canUseCardShopForSelected.value) return
  if (channel === 'qr' && !qrPaymentAvailableForSelected.value) return
  selectedTopupChannel.value = channel
  if (channel === 'qr') {
    syncPayTypeWithSettings()
  }
}

function openSelectedCardShopProduct() {
  if (!selectedCardShopProduct.value?.url) return
  window.location.assign(selectedCardShopProduct.value.url)
}

function openSelectedMonthlyCardShop() {
  const url = selectedMonthlyCardShopUrl.value.trim()
  if (!url) return
  window.location.assign(url)
}

function openMonthlyDirectPurchase() {
  showMonthlyDirectPurchase.value = true
}

function closeMonthlyDirectPurchase() {
  showMonthlyDirectPurchase.value = false
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

function syncTopupChannelWithSettings() {
  if (!selectedBalanceProductId.value && balanceProducts.value.length > 0) {
    selectedBalanceProductId.value = balanceProducts.value[0].id
  }
  if (selectedTopupChannel.value === 'card_shop' && !canUseCardShopForSelected.value) {
    selectedTopupChannel.value = 'qr'
  } else if (selectedTopupChannel.value === 'qr' && !qrPaymentAvailableForSelected.value && canUseCardShopForSelected.value) {
    selectedTopupChannel.value = 'card_shop'
  } else if (canUseCardShopForSelected.value) {
    selectedTopupChannel.value = 'card_shop'
  } else {
    selectedTopupChannel.value = 'qr'
  }
  syncPayTypeWithSettings()
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
  activeOrderAmountYuan.value = 0
  submitting.value = false
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
  if (!showingQrTopup.value || !hasAvailablePayType.value || effectiveAmountYuan.value < 20 || amountError.value) return
  syncPayTypeWithSettings()

  submitting.value = true
  try {
    const orderAmountYuan = effectiveAmountYuan.value
    const response = await createTopupOrder(Math.round(orderAmountYuan * 100), payType.value)
    orderNo.value = response.order_no
    qrCodeURL.value = response.qr_code_url
    activeOrderAmountYuan.value = orderAmountYuan
    updateActiveOrderMeta(response)
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
    const status = await queryTopupOrderStatus(orderNo.value)
    updateActiveOrderMeta(status)
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
  } finally {
    pollInFlight = false
  }
}

onUnmounted(() => {
  stopTimers()
})

void appStore.fetchPublicSettings().then(() => {
  syncTopupChannelWithSettings()
})
</script>

<style scoped>
.topup-page {
  position: relative;
  overflow: hidden;
  padding: 2rem 1rem;
}

.topup-page::before {
  content: "";
  position: absolute;
  inset: 0 auto auto 50%;
  width: min(46rem, 86vw);
  height: 18rem;
  pointer-events: none;
  opacity: 0.42;
  transform: translateX(-50%);
  background:
    radial-gradient(circle at 35% 0%, rgba(154, 59, 31, 0.14), transparent 58%),
    radial-gradient(circle at 70% 24%, rgba(63, 90, 58, 0.12), transparent 52%);
}

.dark .topup-page::before {
  opacity: 0.26;
}

.topup-shell {
  position: relative;
  max-width: 64rem;
  margin: 0 auto;
}

.topup-panel {
  overflow: hidden;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background:
    linear-gradient(135deg, var(--admin-surface, rgba(250, 246, 236, 0.96)), var(--admin-surface-soft, rgba(239, 230, 207, 0.78)));
  box-shadow: var(--admin-shadow, 0 18px 48px rgba(49, 38, 20, 0.11));
  color: var(--admin-ink, #1f1a12);
}

.topup-grid {
  display: grid;
}

.topup-main {
  padding: 2rem 1.5rem;
  border-bottom: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
}

.topup-summary {
  padding: 1.5rem;
  background:
    linear-gradient(180deg, rgba(154, 59, 31, 0.045), rgba(63, 90, 58, 0.035)),
    var(--admin-surface-soft, rgba(239, 230, 207, 0.78));
}

.topup-eyebrow {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  border-radius: 4px;
  padding: 0.35rem 0.65rem;
  color: var(--admin-terracotta-dark, #7a2d17);
  background: rgba(154, 59, 31, 0.08);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.topup-heading {
  margin-top: 1.25rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: clamp(2rem, 4vw, 2.6rem);
  font-weight: 650;
  line-height: 1.16;
  letter-spacing: 0;
}

.topup-copy {
  max-width: 38rem;
  margin-top: 0.85rem;
  color: var(--admin-muted, #8a7d63);
  font-size: 0.95rem;
  line-height: 1.85;
}

.topup-copy--compact {
  margin-top: 0;
}

.topup-section {
  margin-top: 2rem;
}

.topup-section--stack {
  display: grid;
  gap: 2rem;
}

.topup-label {
  margin-bottom: 0.9rem;
  color: var(--admin-ink, #1f1a12);
  font-size: 0.92rem;
  font-weight: 650;
}

.topup-channel-grid,
.topup-products,
.topup-presets,
.topup-pay-grid,
.topup-plan-grid {
  display: grid;
  gap: 0.75rem;
}

.topup-channel-grid,
.topup-products {
  grid-template-columns: 1fr;
}

.topup-presets {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.topup-plan-grid {
  grid-template-columns: 1fr;
}

.topup-pay-grid {
  grid-template-columns: 1fr;
}

.topup-choice,
.topup-product,
.topup-pay-option {
  width: 100%;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  color: var(--admin-ink, #1f1a12);
  text-align: left;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.topup-choice:hover,
.topup-product:hover,
.topup-pay-option:hover {
  border-color: var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  background: var(--admin-parchment, #f2e9d2);
  transform: translateY(-1px);
  box-shadow: var(--admin-shadow-sm, 0 8px 24px rgba(49, 38, 20, 0.08));
}

.topup-choice:focus-visible,
.topup-product:focus-visible,
.topup-pay-option:focus-visible,
.topup-input:focus-visible,
.topup-primary-action:focus-visible,
.topup-secondary-action:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--admin-ring, rgba(154, 59, 31, 0.24));
}

.topup-choice {
  display: flex;
  gap: 0.9rem;
  align-items: flex-start;
  min-height: 8.5rem;
  padding: 1rem;
}

.topup-choice:disabled {
  cursor: not-allowed;
  opacity: 0.58;
  transform: none;
  box-shadow: none;
}

.topup-choice--active,
.topup-product--active,
.topup-pay-option--active {
  border-color: var(--admin-terracotta, #9a3b1f);
  background:
    linear-gradient(180deg, rgba(154, 59, 31, 0.1), transparent 100%),
    var(--admin-control, rgba(255, 252, 245, 0.95));
  box-shadow: inset 0 0 0 1px rgba(154, 59, 31, 0.12);
}

.topup-choice-icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: rgba(154, 59, 31, 0.08);
}

.topup-choice-icon--warm {
  color: var(--admin-terracotta-dark, #7a2d17);
}

.topup-choice-icon--laurel {
  color: var(--admin-laurel, #3f5a3a);
  background: rgba(63, 90, 58, 0.1);
}

.topup-choice-content {
  min-width: 0;
}

.topup-choice-title,
.topup-product-title {
  display: block;
  color: var(--admin-ink-deep, #13100b);
  font-weight: 700;
  line-height: 1.4;
}

.topup-choice-title {
  font-size: 1rem;
}

.topup-choice-desc,
.topup-product-desc {
  display: block;
  margin-top: 0.35rem;
  color: var(--admin-muted, #8a7d63);
  font-size: 0.88rem;
  line-height: 1.55;
}

.topup-status {
  display: inline-flex;
  margin-top: 0.85rem;
  border: 1px solid transparent;
  border-radius: 4px;
  padding: 0.25rem 0.55rem;
  font-size: 0.76rem;
  font-weight: 700;
}

.topup-status--available {
  border-color: rgba(63, 90, 58, 0.24);
  background: rgba(63, 90, 58, 0.1);
  color: var(--admin-laurel-dark, #26361f);
}

.topup-status--disabled {
  border-color: var(--admin-border, rgba(31, 26, 18, 0.14));
  background: var(--admin-surface-soft, rgba(239, 230, 207, 0.78));
  color: var(--admin-muted, #8a7d63);
}

.topup-product {
  min-height: 5.25rem;
  padding: 1rem 1.15rem;
}

.topup-product--preset {
  min-height: 4.4rem;
}

.topup-product-title {
  font-size: 1.15rem;
}

.topup-monthly-product {
  position: relative;
  overflow: hidden;
  min-height: 9.5rem;
  width: 100%;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  color: var(--admin-ink, #1f1a12);
  padding: 1rem;
  text-align: left;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.topup-monthly-product::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 0.25rem;
  background: var(--admin-muted, #8a7d63);
}

.topup-monthly-product--lite::before {
  background: #3f5a3a;
}

.topup-monthly-product--pro::before {
  background: #4b6faf;
}

.topup-monthly-product--max::before {
  background: #9a3b1f;
}

.topup-monthly-product--ultra::before {
  background: #13100b;
}

.topup-monthly-product:hover {
  border-color: var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  background: var(--admin-parchment, #f2e9d2);
  transform: translateY(-1px);
  box-shadow: var(--admin-shadow-sm, 0 8px 24px rgba(49, 38, 20, 0.08));
}

.topup-monthly-product--active {
  border-color: var(--admin-terracotta, #9a3b1f);
  background:
    linear-gradient(180deg, rgba(154, 59, 31, 0.1), transparent 100%),
    var(--admin-control, rgba(255, 252, 245, 0.95));
  box-shadow: inset 0 0 0 1px rgba(154, 59, 31, 0.12);
}

.topup-monthly-product__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.topup-monthly-product__price {
  display: block;
  margin-top: 1.25rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: 1.65rem;
  font-weight: 850;
  line-height: 1;
}

.topup-monthly-product__price small {
  color: var(--admin-muted, #8a7d63);
  font-size: 0.82rem;
  font-weight: 650;
}

.topup-warning,
.topup-success {
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: rgba(154, 59, 31, 0.08);
  color: var(--admin-terracotta-dark, #7a2d17);
  padding: 0.9rem 1rem;
  font-size: 0.9rem;
  line-height: 1.75;
}

.topup-warning {
  background: rgba(154, 106, 31, 0.12);
  color: var(--admin-warning, #9a6a1f);
}

.topup-success {
  background: rgba(63, 90, 58, 0.12);
  color: var(--admin-laurel-dark, #26361f);
}

.topup-input-wrap {
  position: relative;
}

.topup-currency {
  position: absolute;
  top: 50%;
  left: 1rem;
  color: var(--admin-muted, #8a7d63);
  font-size: 1rem;
  font-weight: 700;
  transform: translateY(-50%);
}

.topup-input {
  width: 100%;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  color: var(--admin-ink-deep, #13100b);
  padding: 0.95rem 1rem 0.95rem 2.8rem;
  font-size: 1rem;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease;
}

.topup-input::placeholder {
  color: var(--admin-muted, #8a7d63);
  opacity: 0.72;
}

.topup-input--active,
.topup-input:focus {
  border-color: var(--admin-terracotta, #9a3b1f);
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
}

.topup-error {
  margin-top: 0.55rem;
  color: var(--admin-danger, #9f2f23);
  font-size: 0.86rem;
}

.topup-pay-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 4.6rem;
  padding: 1rem 1.15rem;
}

.topup-qr-panel {
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: var(--admin-surface-soft, rgba(239, 230, 207, 0.78));
  padding: 1.25rem;
}

.topup-qr-wrap {
  display: flex;
  justify-content: center;
  margin-top: 1.25rem;
}

.topup-qr-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 15rem;
  height: 15rem;
  border: 1px dashed var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  border-radius: 8px;
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
}

.topup-qr-image {
  width: 13rem;
  height: 13rem;
  border-radius: 6px;
  object-fit: cover;
}

.topup-qr-empty {
  display: grid;
  gap: 0.8rem;
  color: var(--admin-muted, #8a7d63);
  font-size: 0.9rem;
  text-align: center;
}

.topup-amount-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 1.25rem;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  padding: 0.9rem 1rem;
}

.topup-meta-label {
  color: var(--admin-muted, #8a7d63);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.topup-meta-value {
  margin-top: 0.3rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: 1.12rem;
  font-weight: 700;
}

.topup-meta-value--accent {
  color: var(--admin-terracotta-dark, #7a2d17);
}

.topup-summary-card {
  border: 1px solid var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgba(154, 59, 31, 0.08), transparent 70%),
    var(--admin-surface, rgba(250, 246, 236, 0.96));
  padding: 1.5rem;
  box-shadow: var(--admin-shadow-sm, 0 8px 24px rgba(49, 38, 20, 0.08));
}

.topup-summary-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.topup-summary-kicker {
  color: var(--admin-muted, #8a7d63);
  font-size: 0.86rem;
  font-weight: 650;
}

.topup-summary-title {
  margin-top: 0.45rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: 1.85rem;
  font-weight: 650;
  line-height: 1.2;
}

.topup-price-chip {
  flex: 0 0 auto;
  border: 1px solid rgba(154, 59, 31, 0.2);
  border-radius: 4px;
  background: rgba(154, 59, 31, 0.1);
  color: var(--admin-terracotta-dark, #7a2d17);
  padding: 0.45rem 0.65rem;
  font-size: 0.9rem;
  font-weight: 700;
}

.topup-price-chip--muted {
  border-color: var(--admin-border, rgba(31, 26, 18, 0.14));
  background: var(--admin-surface-soft, rgba(239, 230, 207, 0.78));
  color: var(--admin-muted, #8a7d63);
}

.topup-summary-rows {
  display: grid;
  gap: 0.9rem;
  margin-top: 1.5rem;
}

.topup-summary-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: var(--admin-muted, #8a7d63);
  font-size: 0.92rem;
}

.topup-summary-row strong {
  color: var(--admin-ink-deep, #13100b);
  font-weight: 700;
  text-align: right;
}

.topup-summary-row .topup-summary-limit {
  display: grid;
  gap: 0.22rem;
}

.topup-summary-limit small {
  color: var(--admin-muted, #8a7d63);
  font-size: 0.74rem;
  font-weight: 650;
  line-height: 1.35;
}

.topup-primary-action,
.topup-secondary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 3.45rem;
  border-radius: 6px;
  padding: 0.9rem 1.25rem;
  font-size: 1rem;
  font-weight: 750;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.topup-primary-action {
  margin-top: 2rem;
  border: 1px solid var(--admin-terracotta, #9a3b1f);
  background: var(--admin-terracotta, #9a3b1f);
  color: var(--admin-marble, #faf6ec) !important;
  box-shadow: 0 12px 24px rgba(154, 59, 31, 0.18);
}

.topup-primary-action:hover:not(:disabled) {
  border-color: var(--admin-terracotta-dark, #7a2d17);
  background: var(--admin-terracotta-dark, #7a2d17);
  transform: translateY(-1px);
}

.topup-primary-action:disabled {
  cursor: not-allowed;
  opacity: 0.56;
  box-shadow: none;
  transform: none;
}

.topup-monthly-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 2rem;
}

.topup-monthly-actions .topup-primary-action,
.topup-monthly-actions .topup-secondary-action {
  margin-top: 0;
}

.topup-monthly-action {
  min-height: 3.35rem;
  padding-inline: 0.85rem;
  text-align: center;
}

.topup-summary-actions {
  display: grid;
  gap: 1rem;
  margin-top: 2rem;
}

.topup-inline-pay {
  margin-top: 1.5rem;
}

.topup-inline-pay .topup-primary-action {
  margin-top: 1rem;
}

.topup-secondary-action {
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  color: var(--admin-ink, #1f1a12);
}

.topup-secondary-action:hover {
  border-color: var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  background: var(--admin-parchment, #f2e9d2);
  color: var(--admin-ink-deep, #13100b);
}

.topup-secondary-action--stacked {
  margin-top: 0.75rem;
}

.dark .topup-primary-action {
  color: var(--admin-marble, #332d23) !important;
}

.topup-modal-backdrop {
  position: fixed;
  z-index: 80;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgba(19, 16, 11, 0.42);
}

.topup-direct-modal {
  width: min(28rem, 100%);
  border: 1px solid var(--admin-border-strong, rgba(31, 26, 18, 0.32));
  border-radius: 8px;
  padding: 1.25rem;
  background: var(--admin-surface, #faf6ec);
  color: var(--admin-ink, #1f1a12);
  box-shadow: 0 24px 60px rgba(31, 26, 18, 0.24);
}

.topup-direct-modal__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.topup-direct-modal__head h2 {
  margin-top: 0.25rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: 1.55rem;
  font-weight: 750;
  line-height: 1.2;
  letter-spacing: 0;
}

.topup-modal-close {
  display: inline-grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 6px;
  color: var(--admin-muted, #8a7d63);
  background: var(--admin-control, rgba(255, 252, 245, 0.95));
  font-size: 1.35rem;
  line-height: 1;
}

.topup-direct-qr {
  display: grid;
  min-height: 14rem;
  place-items: center;
  margin-top: 1.25rem;
  border: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
  border-radius: 8px;
  background: rgba(255, 252, 245, 0.78);
}

.topup-direct-qr img {
  display: block;
  width: min(13rem, 72vw);
  height: min(13rem, 72vw);
  object-fit: contain;
}

.topup-direct-qr--empty {
  color: var(--admin-muted, #8a7d63);
  font-weight: 650;
}

.topup-direct-copy {
  margin-top: 1rem;
  color: var(--admin-ink-deep, #13100b);
  font-size: 1rem;
  font-weight: 750;
  line-height: 1.65;
  text-align: center;
}

@media (max-width: 520px) {
  .topup-monthly-actions {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 640px) {
  .topup-page {
    padding-inline: 1.5rem;
  }

  .topup-main,
  .topup-summary {
    padding: 2rem;
  }

  .topup-channel-grid,
  .topup-products,
  .topup-plan-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .topup-presets {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .topup-pay-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .topup-pay-grid--single {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 1024px) {
  .topup-page {
    padding: 2rem;
  }

  .topup-grid {
    grid-template-columns: minmax(0, 1.05fr) minmax(22rem, 0.95fr);
  }

  .topup-main {
    border-right: 1px solid var(--admin-border, rgba(31, 26, 18, 0.14));
    border-bottom: 0;
  }

  .topup-summary-card {
    position: sticky;
    top: 6rem;
  }
}
</style>
