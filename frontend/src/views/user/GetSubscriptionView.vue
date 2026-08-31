<template>
  <AppLayout>
    <div class="topup-page">
      <div class="topup-shell">
        <header class="topup-hero">
          <h1 class="topup-heading">{{ t('topup.title') }}</h1>
          <p class="topup-copy">{{ t('subscriptionAccess.summary') }}</p>
        </header>

        <div class="topup-grid" data-tour="topup-panel">
          <main class="topup-main">
            <template v-if="step === 1">
              <div class="topup-catalog-tabs" role="tablist" aria-label="充值商品类型">
                <button
                  type="button"
                  role="tab"
                  :aria-selected="selectedProductKind === 'balance'"
                  :class="{ 'topup-catalog-tab--active': selectedProductKind === 'balance' }"
                  class="topup-catalog-tab"
                  @click="selectCatalog('balance')"
                >
                  余额充值
                </button>
                <button
                  type="button"
                  role="tab"
                  :aria-selected="selectedProductKind === 'monthly'"
                  :class="{ 'topup-catalog-tab--active': selectedProductKind === 'monthly' }"
                  class="topup-catalog-tab"
                  @click="selectCatalog('monthly')"
                >
                  {{ t('topup.developerPlansTitle') }}
                </button>
              </div>

              <section v-if="selectedProductKind === 'balance'" class="topup-section">
                <h2 class="topup-label">选择产品</h2>
                <div class="topup-product-list">
                  <button
                    v-for="product in balanceProducts"
                    :key="product.id"
                    type="button"
                    class="topup-product"
                    :class="{ 'topup-product--active': selectedBalanceProduct?.id === product.id }"
                    @click="selectBalanceProduct(product)"
                  >
                    <span class="topup-radio" aria-hidden="true"><span></span></span>
                    <span class="topup-product-name">{{ balanceProductDisplayLabel(product) }}</span>
                    <span v-if="product.promotional && !product.newcomerOnly" class="topup-promotion-badge">限时特惠</span>
                    <span class="topup-product-price">¥{{ product.amountCny }}</span>
                  </button>
                </div>
              </section>

              <section v-else class="topup-section">
                <h2 class="topup-label">选择产品</h2>
                <div class="topup-product-list">
                  <button
                    v-for="plan in monthlyCreditCardPlans"
                    :key="plan.id"
                    type="button"
                    class="topup-product topup-product--monthly"
                    :class="{ 'topup-product--active': selectedMonthlyPlan?.id === plan.id }"
                    @click="selectMonthlyPlan(plan)"
                  >
                    <span class="topup-radio" aria-hidden="true"><span></span></span>
                    <span class="topup-product-name">{{ plan.name }}</span>
                    <span class="topup-product-price">{{ monthlyPlanPrice(plan) }}</span>
                  </button>
                </div>
                <p class="topup-monthly-catalog-note">
                  {{ t('topup.monthlyPlanSharedPool') }}。
                  {{ t('topup.monthlyPlanModelsPrefix') }}
                  <RouterLink to="/models">{{ t('topup.monthlyPlanModelsLink') }}</RouterLink>。
                </p>
              </section>
            </template>

            <section v-else-if="showingQrTopup" class="topup-qr-panel">
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
                    <button type="button" class="topup-secondary-action" @click="resubmitOrder">{{ t('topup.refresh') }}</button>
                  </div>
                </div>
              </div>
              <p v-if="qrCodeClientRendered && !qrExpired" class="topup-copy topup-copy--compact">
                {{ t('topup.qrContentHint') }}
              </p>
            </section>
          </main>

          <aside class="topup-summary">
            <div class="topup-summary-card">
              <p class="topup-summary-kicker">订单摘要</p>

              <template v-if="selectedProductKind === 'balance'">
                <p class="topup-summary-amount">¥{{ displayAmountText }}</p>

                <div class="topup-summary-rows">
                  <div class="topup-summary-row">
                    <span>产品</span>
                    <strong>{{ selectedBalanceProduct ? balanceProductDisplayLabel(selectedBalanceProduct) : '—' }}</strong>
                  </div>
                  <div class="topup-summary-row">
                    <span>单价</span>
                    <strong>¥{{ selectedBalanceProduct?.amountCny ?? 0 }}</strong>
                  </div>
                  <div v-if="supportsSelectedQuantity && step === 1" class="topup-summary-row topup-summary-row--quantity">
                    <span>数量</span>
                    <div class="topup-quantity" aria-label="购买数量">
                      <button
                        type="button"
                        aria-label="减少数量"
                        :disabled="selectedQuantity <= 1"
                        @click="decreaseQuantity"
                      >−</button>
                      <output data-testid="topup-quantity">{{ selectedQuantity }}</output>
                      <button
                        type="button"
                        aria-label="增加数量"
                        :disabled="selectedQuantity >= maxSelectedQuantity"
                        @click="increaseQuantity"
                      >+</button>
                    </div>
                  </div>
                  <div class="topup-summary-divider"></div>
                  <div class="topup-summary-row">
                    <span>支付金额</span>
                    <strong>¥{{ displayAmountText }}</strong>
                  </div>
                  <div class="topup-summary-row">
                    <span>{{ t('topup.creditedAmount') }}</span>
                    <strong class="topup-credit-amount">¥{{ displayCreditedText }}</strong>
                  </div>
                  <div v-if="step === 2 && !qrExpired" class="topup-summary-row">
                    <span>{{ t('topup.qrExpiry') }}</span>
                    <strong>{{ countdownText }}</strong>
                  </div>
                </div>

                <section
                  v-if="!selectedNewcomerNative && step === 1"
                  class="topup-summary-payment"
                  aria-labelledby="topup-summary-payment-title"
                >
                  <h2 id="topup-summary-payment-title" class="topup-label">支付方式</h2>
                  <div class="topup-summary-methods">
                    <button
                      type="button"
                      data-testid="topup-method-alipay"
                      :disabled="!canUseAlipay || effectiveAmountYuan < 20"
                      :class="{ 'topup-summary-method--active': selectedTopupChannel === 'alipay' }"
                      class="topup-summary-method"
                      @click="selectTopupChannel('alipay')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="alipay" />
                      <strong>{{ t('nativeCheckout.alipayPay') }}</strong>
                    </button>
                    <button
                      type="button"
                      data-testid="topup-method-wechat"
                      :disabled="!canUseWechat || effectiveAmountYuan < 20"
                      :class="{ 'topup-summary-method--active': selectedTopupChannel === 'wechat' }"
                      class="topup-summary-method"
                      @click="selectTopupChannel('wechat')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="wechat" />
                      <strong>{{ t('nativeCheckout.wechatPay') }}</strong>
                    </button>
                    <button
                      v-if="affiliateWallet?.wallet_checkout_enabled && !selectedBalanceProduct?.promotional"
                      type="button"
                      data-testid="topup-method-commission-wallet"
                      :disabled="effectiveAmountYuan < 20"
                      :class="{ 'topup-summary-method--active': selectedTopupChannel === 'commission_wallet' }"
                      class="topup-summary-method"
                      @click="selectTopupChannel('commission_wallet')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="commission_wallet" />
                      <strong>佣金钱包</strong>
                    </button>
                  </div>
                  <p v-if="selectedTopupChannel === 'commission_wallet'" class="topup-warning">
                    可用佣金 {{ affiliateWalletAvailableText }}，本次支付 {{ balanceWalletPriceText }}
                  </p>
                  <p v-if="paymentNotice" class="topup-warning">{{ paymentNotice }}</p>
                </section>

                <div v-if="selectedNewcomerNative" class="topup-newcomer-actions">
                  <NativeCheckoutTrialOffer compact />
                  <p class="topup-warning">{{ t('topup.newcomerCardLimit') }}</p>
                </div>

                <button
                  v-else-if="step === 1"
                  type="button"
                  data-testid="topup-submit"
                  class="topup-primary-action"
                  :disabled="submitting || !canUseSelectedQrMethod || !!amountError || effectiveAmountYuan < 20"
                  @click="submitOrder"
                >
                  <span v-if="submitting">{{ t('topup.submitting') }}</span>
                  <span v-else>{{ selectedTopupChannel === 'commission_wallet' ? `使用佣金钱包支付 ${balanceWalletPriceText}` : `立即支付 ¥${displayAmountText}` }}</span>
                </button>

                <div v-else class="topup-summary-actions">
                  <div class="topup-success">{{ t('topup.waitingPayment') }}</div>
                  <button type="button" class="topup-secondary-action" @click="resetToForm">
                    {{ t('subscriptionAccess.editAmount') }}
                  </button>
                </div>
              </template>

              <template v-else>
                <p class="topup-summary-amount">{{ selectedMonthlyDisplayPrice }}</p>
                <div class="topup-summary-rows">
                  <div class="topup-summary-row"><span>产品</span><strong>{{ selectedMonthlyPlan?.name }}</strong></div>
                  <div class="topup-summary-row"><span>网站实时价格</span><strong>{{ selectedMonthlyDisplayPrice }}</strong></div>
                  <div class="topup-summary-row"><span>{{ t('topup.monthlyPlanMonthlyLimit') }}</span><strong>{{ selectedMonthlyPlan?.displayMonthlyCreditsText }} AI credits</strong></div>
                  <div class="topup-summary-row"><span>有效期</span><strong>31 天</strong></div>
                </div>

                <section class="topup-summary-payment" aria-labelledby="monthly-summary-payment-title">
                  <h2 id="monthly-summary-payment-title" class="topup-label">支付方式</h2>
                  <div class="topup-summary-methods">
                    <button
                      type="button"
                      data-testid="monthly-method-alipay"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': selectedMonthlyTopupChannel === 'alipay' }"
                      :disabled="!selectedMonthlyNativeOffer || !canUseAlipay"
                      @click="selectMonthlyTopupChannel('alipay')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="alipay" />
                      <strong>{{ t('nativeCheckout.alipayPay') }}</strong>
                    </button>
                    <button
                      type="button"
                      data-testid="monthly-method-wechat"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': selectedMonthlyTopupChannel === 'wechat' }"
                      :disabled="!selectedMonthlyNativeOffer || !canUseWechat"
                      @click="selectMonthlyTopupChannel('wechat')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="wechat" />
                      <strong>{{ t('nativeCheckout.wechatPay') }}</strong>
                    </button>
                    <button
                      v-if="affiliateWallet?.wallet_checkout_enabled"
                      type="button"
                      data-testid="monthly-method-commission-wallet"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': selectedMonthlyTopupChannel === 'commission_wallet' }"
                      :disabled="!selectedMonthlyNativeOffer"
                      @click="selectMonthlyTopupChannel('commission_wallet')"
                    >
                      <span class="topup-radio" aria-hidden="true"><span></span></span>
                      <PaymentMethodIcon kind="commission_wallet" />
                      <strong>佣金钱包</strong>
                    </button>
                  </div>
                </section>

                <div v-if="selectedMonthlyTopupChannel === 'commission_wallet'" class="topup-summary-rows">
                  <div class="topup-summary-row"><span>可用佣金</span><strong>{{ affiliateWalletAvailableText }}</strong></div>
                  <div class="topup-summary-row"><span>佣金钱包价</span><strong>{{ commissionWalletPriceText }}</strong></div>
                  <div class="topup-summary-row"><span>支付后剩余</span><strong>{{ commissionWalletRemainingText }}</strong></div>
                </div>

                <div class="topup-monthly-actions">
                  <button
                    v-if="selectedMonthlyTopupChannel === 'commission_wallet'"
                    type="button"
                    class="topup-primary-action"
                    :disabled="commissionWalletBusy || !canPayMonthlyWithCommission"
                    @click="payMonthlyWithCommission"
                  >
                    {{ commissionWalletBusy ? '正在开通…' : `使用佣金钱包支付 ${commissionWalletPriceText}` }}
                  </button>
                  <NativeCheckoutTrialOffer
                    v-else-if="selectedMonthlyNativeOffer && selectedMonthlyPayMethod"
                    :key="selectedMonthlyPlan.id"
                    :offer-code="selectedMonthlyPlan.id"
                    :preferred-pay-method="selectedMonthlyPayMethod"
                    :action-label="`立即支付 ${selectedMonthlyDisplayPrice}`"
                    action-only
                  />
                  <button
                    v-else
                    type="button"
                    class="topup-primary-action"
                    disabled
                  >
                    <span>{{ t('topup.noAvailablePayType') }}</span>
                  </button>
                </div>
              </template>
            </div>
          </aside>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PaymentMethodIcon from '@/components/user/PaymentMethodIcon.vue'
import { createTopupOrder, queryTopupOrderStatus, type TopupPayType } from '@/api/topup'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  BALANCE_TOPUP_PRESETS,
  PROMOTIONAL_BALANCE_TOPUPS,
  getCreditedBalanceTopupProductAmount
} from '@/constants/balanceTopups'
import { type MonthlyCreditCardPlan } from '@/constants/monthlyCreditCards'
import { useMonthlyCreditCardPlans } from '@/composables/useMonthlyCreditCardPlans'
import { useManualNewcomerOffer } from '@/composables/useManualNewcomerOffer'
import { useNativeCheckoutOffers } from '@/composables/useNativeCheckoutOffers'
import NativeCheckoutTrialOffer from '@/components/user/NativeCheckoutTrialOffer.vue'
import {
  createCommissionWalletCheckoutOrder,
  type NativeCheckoutPaymentMethod,
} from '@/api/nativeCheckout'
import {
  affiliateIdempotencyKey,
  getAffiliateWallet,
  purchaseBalanceWithAffiliateCommission,
  type AffiliateWallet,
} from '@/api/agent'
import { isDirectQrImageUrl, renderQrCodeDataUrl } from '@/utils/qrImage'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const presets = BALANCE_TOPUP_PRESETS
const QR_TTL_SECONDS = 300
type SelectedProductKind = 'balance' | 'monthly'
type MonthlyPaymentChannel = NativeCheckoutPaymentMethod | 'commission_wallet'
type BalancePaymentChannel = TopupPayType | 'commission_wallet'
type BalanceProduct = {
  id: string
  label: string
  amountCny: number
  creditedAmountCny?: number
  bonusAmountCny?: number
  promotional?: boolean
  newcomerOnly?: boolean
}

const step = ref<1 | 2>(1)
const selectedTopupChannel = ref<BalancePaymentChannel>('alipay')
const selectedProductKind = ref<SelectedProductKind>('balance')
const selectedBalanceProductId = ref('')
const selectedQuantity = ref(1)
const selectedMonthlyPlanId = ref<MonthlyCreditCardPlan['id']>('plus')
const selectedMonthlyTopupChannel = ref<MonthlyPaymentChannel>('alipay')
const payType = ref<TopupPayType>('alipay')
const submitting = ref(false)
const qrCodeURL = ref('')
const qrCodeRawPayload = ref('')
const qrCodeClientRendered = ref(false)
const orderNo = ref('')
const qrExpired = ref(false)
const countdown = ref(QR_TTL_SECONDS)
const activeOrderAmountYuan = ref(0)
const activeOrderCreditedAmountYuan = ref(0)
const affiliateWallet = ref<AffiliateWallet | null>(null)
const commissionWalletBusy = ref(false)
const commissionWalletIdempotencyKey = ref('')
const balanceWalletIdempotencyKey = ref('')
const { plans: monthlyCreditCardPlans, loadMonthlyCreditCardPlans } = useMonthlyCreditCardPlans()
const { loadOffers: loadNativeCheckoutOffers, findNativeOfferByCode } = useNativeCheckoutOffers()
const {
  state: newcomerOfferState,
  mode: newcomerOfferMode,
  refresh: refreshNewcomerOffer
} = useManualNewcomerOffer()

let pollTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null
let pollInFlight = false

// 根据公开设置决定用户侧可见支付渠道；关闭的渠道直接不展示。
// 优先使用 provider 无关的新开关，老后端未返回时回退到虎皮椒开关。
const topupAlipayEnabled = computed(
  () => appStore.cachedPublicSettings?.topup_alipay_enabled ?? appStore.cachedPublicSettings?.xunhu_alipay_enabled ?? false
)
const topupWechatEnabled = computed(
  () => appStore.cachedPublicSettings?.topup_wechat_enabled ?? appStore.cachedPublicSettings?.xunhu_wechat_enabled ?? false
)
const canUseAlipay = computed(() => topupAlipayEnabled.value)
const canUseWechat = computed(() => topupWechatEnabled.value)
const balanceProducts = computed<BalanceProduct[]>(() => {
  const standardProducts: BalanceProduct[] = presets.map((amount) => ({
    id: `qr-${amount}`,
    label: `¥${amount} 余额卡`,
    amountCny: amount
  }))
  const promotionalProducts: BalanceProduct[] = PROMOTIONAL_BALANCE_TOPUPS.map((product) => {
    return {
      id: `promotion-${product.paidAmountCny}`,
      label: product.newcomerOnly
        ? t('topup.newcomerCardTitle', { credited: product.creditedAmountCny })
        : t('topup.promotionalCardTitle', { paid: product.paidAmountCny }),
      amountCny: product.paidAmountCny,
      creditedAmountCny: product.creditedAmountCny,
      bonusAmountCny: product.bonusAmountCny,
      promotional: true,
      newcomerOnly: product.newcomerOnly
    }
  })
  const visiblePromotionalProducts = promotionalProducts.filter(
    (product) =>
      !product.newcomerOnly ||
      (newcomerOfferState.value === 'available' && newcomerOfferMode.value === 'native')
  )
  return [
    ...visiblePromotionalProducts.filter((product) => product.newcomerOnly),
    ...standardProducts,
    ...visiblePromotionalProducts.filter((product) => !product.newcomerOnly)
  ]
})
const selectedBalanceProduct = computed<BalanceProduct | undefined>(
  () => balanceProducts.value.find((product) => product.id === selectedBalanceProductId.value) ?? balanceProducts.value[0]
)
const supportsSelectedQuantity = computed(
  () => selectedProductKind.value === 'balance' && !!selectedBalanceProduct.value && !selectedBalanceProduct.value.newcomerOnly
)
const maxSelectedQuantity = computed(() => {
  const unitAmount = selectedBalanceProduct.value?.amountCny ?? 0
  if (!supportsSelectedQuantity.value || unitAmount <= 0) return 1
  return Math.max(1, Math.floor(3000 / unitAmount))
})
const selectedNewcomerNative = computed(
  () => selectedProductKind.value === 'balance' &&
    selectedBalanceProduct.value?.newcomerOnly === true &&
    newcomerOfferMode.value === 'native'
)
const selectedMonthlyPlan = computed(
  () => monthlyCreditCardPlans.value.find((plan) => plan.id === selectedMonthlyPlanId.value) ?? monthlyCreditCardPlans.value[0]
)
// 约定：月卡套餐的原生结账 offer code 与前端套餐 id 相同（plus/pro/max）。
// 只有可见且 provider=easypay 的订阅 offer 命中；未命中时保持关闭，
// 不再回退到外部卡密商城。
const selectedMonthlyNativeOffer = computed(() => {
  if (selectedProductKind.value !== 'monthly') return undefined
  const plan = selectedMonthlyPlan.value
  if (!plan) return undefined
  return findNativeOfferByCode(plan.id, 'subscription')
})
const selectedMonthlyPayMethod = computed<NativeCheckoutPaymentMethod | undefined>(() => {
  return selectedMonthlyTopupChannel.value === 'commission_wallet' ? undefined : selectedMonthlyTopupChannel.value
})
const selectedMonthlyDisplayPrice = computed(() => {
  const fen = selectedMonthlyNativeOffer.value?.pay_amount_cny_fen
  return typeof fen === 'number' && fen > 0 ? `¥${formatMoney(fen / 100, false)}` : '价格加载失败'
})
const commissionWalletChargeFen = computed(() => {
  const offerFen = selectedMonthlyNativeOffer.value?.pay_amount_cny_fen ?? 0
  const rateBPS = affiliateWallet.value?.wallet_purchase_rate_bps ?? 0
  if (offerFen <= 0 || rateBPS <= 0) return 0
  return Math.ceil((offerFen * rateBPS) / 10_000)
})
const affiliateWalletAvailableMicros = computed(() => affiliateWallet.value?.available_cash_micros ?? 0)
const affiliateWalletAvailableText = computed(() => `¥${(affiliateWalletAvailableMicros.value / 1_000_000).toFixed(2)}`)
const commissionWalletPriceText = computed(() => `¥${(commissionWalletChargeFen.value / 100).toFixed(2)}`)
const commissionWalletRemainingText = computed(() => {
  const remaining = affiliateWalletAvailableMicros.value - commissionWalletChargeFen.value * 10_000
  return `¥${(Math.max(remaining, 0) / 1_000_000).toFixed(2)}`
})
const balanceWalletChargeFen = computed(() => {
  const priceFen = Math.round(effectiveAmountYuan.value * 100)
  const rateBPS = affiliateWallet.value?.wallet_purchase_rate_bps ?? 0
  return priceFen > 0 && rateBPS > 0 ? Math.ceil((priceFen * rateBPS) / 10_000) : 0
})
const balanceWalletPriceText = computed(() => `¥${(balanceWalletChargeFen.value / 100).toFixed(2)}`)
const canPayMonthlyWithCommission = computed(() => {
  return !!affiliateWallet.value?.wallet_checkout_enabled &&
    !!selectedMonthlyNativeOffer.value && commissionWalletChargeFen.value > 0 &&
    affiliateWalletAvailableMicros.value >= commissionWalletChargeFen.value * 10_000
})
const showingQrTopup = computed(
  () => selectedProductKind.value === 'balance'
)
const canUseSelectedQrMethod = computed(() => {
  if (!showingQrTopup.value) return false
  if (selectedTopupChannel.value === 'commission_wallet') {
    return !!affiliateWallet.value?.wallet_checkout_enabled &&
      !selectedBalanceProduct.value?.promotional &&
      affiliateWalletAvailableMicros.value >= balanceWalletChargeFen.value * 10_000
  }
  return selectedTopupChannel.value === 'alipay' ? canUseAlipay.value : canUseWechat.value
})
const effectiveAmountYuan = computed<number>(() => {
  if (selectedProductKind.value !== 'balance') return 0
  const unitAmount = selectedBalanceProduct.value?.amountCny ?? 0
  return unitAmount * (supportsSelectedQuantity.value ? selectedQuantity.value : 1)
})

const displayAmountYuan = computed(() => {
  if (step.value === 2 && activeOrderAmountYuan.value > 0) {
    return activeOrderAmountYuan.value
  }
  return effectiveAmountYuan.value || 20
})

const displayAmountText = computed(() => formatMoney(displayAmountYuan.value, false))
const displayCreditedAmountYuan = computed(() => {
  if (step.value === 2 && activeOrderCreditedAmountYuan.value > 0) {
    return activeOrderCreditedAmountYuan.value
  }
  const unitAmount = selectedBalanceProduct.value?.amountCny ?? displayAmountYuan.value
  const quantity = supportsSelectedQuantity.value ? selectedQuantity.value : 1
  return getCreditedBalanceTopupProductAmount(unitAmount, quantity)
})
const displayCreditedText = computed(() => formatMoney(displayCreditedAmountYuan.value, true))

const amountError = computed<string>(() => {
  if (selectedProductKind.value !== 'balance' || !showingQrTopup.value) return ''
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
  if (showingQrTopup.value && amountError.value) return amountError.value
  if (showingQrTopup.value && !canUseSelectedQrMethod.value) return t('topup.noAvailablePayType')
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

function balanceProductDisplayLabel(product: BalanceProduct) {
  if (product.newcomerOnly) {
    return `新人特惠 ¥${product.amountCny} 到账 ¥${product.creditedAmountCny ?? product.amountCny}`
  }
  if (product.promotional) {
    return `实付 ¥${product.amountCny} 到账 ¥${product.creditedAmountCny ?? product.amountCny}`
  }
  return `¥${product.amountCny} 余额卡`
}

function selectCatalog(kind: SelectedProductKind) {
  selectedProductKind.value = kind
  if (kind === 'balance') syncTopupChannelWithSettings()
  else syncMonthlyTopupChannelWithSettings()
}

function selectBalanceProduct(product: BalanceProduct) {
  selectedProductKind.value = 'balance'
  selectedBalanceProductId.value = product.id
  selectedQuantity.value = 1
  balanceWalletIdempotencyKey.value = ''
  syncTopupChannelWithSettings()
}

function selectMonthlyPlan(plan: MonthlyCreditCardPlan) {
  selectedProductKind.value = 'monthly'
  selectedMonthlyPlanId.value = plan.id
  commissionWalletIdempotencyKey.value = ''
  syncMonthlyTopupChannelWithSettings()
}

function selectMonthlyTopupChannel(channel: MonthlyPaymentChannel) {
  if (channel === 'alipay' && (!selectedMonthlyNativeOffer.value || !canUseAlipay.value)) return
  if (channel === 'wechat' && (!selectedMonthlyNativeOffer.value || !canUseWechat.value)) return
  if (channel === 'commission_wallet' && !affiliateWallet.value?.wallet_checkout_enabled) return
  selectedMonthlyTopupChannel.value = channel
}

function syncMonthlyTopupChannelWithSettings() {
  if (selectedMonthlyNativeOffer.value && canUseAlipay.value) {
    selectedMonthlyTopupChannel.value = 'alipay'
    return
  }
  if (selectedMonthlyNativeOffer.value && canUseWechat.value) {
    selectedMonthlyTopupChannel.value = 'wechat'
  }
}

function monthlyPlanPrice(plan: MonthlyCreditCardPlan): string {
  const offer = findNativeOfferByCode(plan.id, 'subscription')
  return offer ? `¥${formatMoney(offer.pay_amount_cny_fen / 100, false)}` : '价格加载失败'
}

async function payMonthlyWithCommission() {
  const offer = selectedMonthlyNativeOffer.value
  if (!offer || !canPayMonthlyWithCommission.value) return
  commissionWalletBusy.value = true
  try {
    // Refresh immediately before the irreversible debit. The server performs
    // the same optimistic price check and rejects a changed catalog price.
    await loadNativeCheckoutOffers()
    const freshOffer = selectedMonthlyNativeOffer.value
    if (!freshOffer) throw new Error('月卡实时价格加载失败')
    if (!commissionWalletIdempotencyKey.value) {
      commissionWalletIdempotencyKey.value = affiliateIdempotencyKey(`monthly-${freshOffer.code}`)
    }
    const order = await createCommissionWalletCheckoutOrder(
      freshOffer.code,
      freshOffer.pay_amount_cny_fen,
      commissionWalletIdempotencyKey.value,
    )
    if (order.status !== 'completed') throw new Error('佣金钱包订单尚未完成，请稍后重试')
    affiliateWallet.value = await getAffiliateWallet()
    await Promise.all([loadMonthlyCreditCardPlans(), authStore.refreshUser()])
    commissionWalletIdempotencyKey.value = ''
    appStore.showSuccess(`${selectedMonthlyPlan.value?.name ?? '月卡'}已开通`)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '佣金钱包支付失败'))
  } finally {
    commissionWalletBusy.value = false
  }
}

function selectTopupChannel(channel: BalancePaymentChannel) {
  if (channel === 'alipay' && (!canUseAlipay.value || effectiveAmountYuan.value < 20)) return
  if (channel === 'wechat' && (!canUseWechat.value || effectiveAmountYuan.value < 20)) return
  if (channel === 'commission_wallet' && (!affiliateWallet.value?.wallet_checkout_enabled || selectedBalanceProduct.value?.promotional)) return
  selectedTopupChannel.value = channel
  if (channel !== 'commission_wallet') payType.value = channel
}

function decreaseQuantity() {
  selectedQuantity.value = Math.max(1, selectedQuantity.value - 1)
}

function increaseQuantity() {
  selectedQuantity.value = Math.min(maxSelectedQuantity.value, selectedQuantity.value + 1)
}

function syncTopupChannelWithSettings() {
  if (!selectedBalanceProductId.value && balanceProducts.value.length > 0) {
    selectedBalanceProductId.value = balanceProducts.value[0].id
  }
  const amountSupportsScan = effectiveAmountYuan.value >= 20
  if (selectedTopupChannel.value === 'alipay' && canUseAlipay.value && amountSupportsScan) {
    payType.value = 'alipay'
    return
  }
  if (selectedTopupChannel.value === 'wechat' && canUseWechat.value && amountSupportsScan) {
    payType.value = 'wechat'
    return
  }
  if (canUseAlipay.value && amountSupportsScan) {
    selectedTopupChannel.value = 'alipay'
    payType.value = 'alipay'
  } else if (canUseWechat.value && amountSupportsScan) {
    selectedTopupChannel.value = 'wechat'
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
  qrCodeRawPayload.value = ''
  qrCodeClientRendered.value = false
  orderNo.value = ''
  qrExpired.value = false
  countdown.value = QR_TTL_SECONDS
  activeOrderAmountYuan.value = 0
  activeOrderCreditedAmountYuan.value = 0
  submitting.value = false
}

function updateActiveOrderMeta(meta: {
  amount_cny_fen?: number
  credited_amount_cny_fen?: number
  pay_type?: TopupPayType
  qr_code_url?: string | null
}) {
  if (typeof meta.amount_cny_fen === 'number' && Number.isFinite(meta.amount_cny_fen) && meta.amount_cny_fen > 0) {
    activeOrderAmountYuan.value = meta.amount_cny_fen / 100
  }
  if (typeof meta.credited_amount_cny_fen === 'number' && meta.credited_amount_cny_fen > 0) {
    activeOrderCreditedAmountYuan.value = meta.credited_amount_cny_fen / 100
  }
  if (meta.pay_type === 'alipay' || meta.pay_type === 'wechat') {
    payType.value = meta.pay_type
  }
  if (typeof meta.qr_code_url === 'string' && meta.qr_code_url.trim() !== '') {
    void applyQrCodePayload(meta.qr_code_url)
  }
}

// 后端返回的 qr_code_url 可能是二维码图片地址，也可能是支付内容
//（如 easypay 的收银台链接或 weixin:// 协议串）；后者在客户端渲染为二维码。
async function applyQrCodePayload(payload: string) {
  const trimmed = payload.trim()
  if (!trimmed || trimmed === qrCodeRawPayload.value) return
  if (isDirectQrImageUrl(trimmed)) {
    qrCodeRawPayload.value = trimmed
    qrCodeURL.value = trimmed
    qrCodeClientRendered.value = false
    return
  }
  try {
    const dataUrl = await renderQrCodeDataUrl(trimmed)
    qrCodeRawPayload.value = trimmed
    qrCodeURL.value = dataUrl
    qrCodeClientRendered.value = true
  } catch (error) {
    console.error('Failed to render topup QR code:', error)
  }
}

async function submitOrder() {
  if (!showingQrTopup.value || !canUseSelectedQrMethod.value || effectiveAmountYuan.value < 20 || amountError.value) return

  submitting.value = true
  try {
    const orderAmountYuan = effectiveAmountYuan.value
    const unitAmountYuan = selectedBalanceProduct.value?.amountCny ?? orderAmountYuan
    const quantity = supportsSelectedQuantity.value ? selectedQuantity.value : 1
    if (selectedTopupChannel.value === 'commission_wallet') {
      if (!balanceWalletIdempotencyKey.value) {
        balanceWalletIdempotencyKey.value = affiliateIdempotencyKey('balance-purchase')
      }
      await purchaseBalanceWithAffiliateCommission(Math.round(orderAmountYuan * 100), balanceWalletIdempotencyKey.value)
      affiliateWallet.value = await getAffiliateWallet()
      await authStore.refreshUser()
      balanceWalletIdempotencyKey.value = ''
      appStore.showSuccess(`已充值余额 ¥${orderAmountYuan.toFixed(2)}`)
      return
    }
    payType.value = selectedTopupChannel.value
    const response = await createTopupOrder(Math.round(orderAmountYuan * 100), payType.value, {
      productAmountCnyFen: Math.round(unitAmountYuan * 100),
      quantity,
    })
    orderNo.value = response.order_no
    activeOrderAmountYuan.value = orderAmountYuan
    activeOrderCreditedAmountYuan.value = getCreditedBalanceTopupProductAmount(unitAmountYuan, quantity)
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
      try {
        await authStore.refreshUser()
      } catch (error) {
        console.error('Failed to refresh user balance after topup:', error)
      }
      appStore.showSuccess(t('topup.paySuccessWithCredit', { amount: displayCreditedText.value }))
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

void Promise.all([
  appStore.fetchPublicSettings(),
  loadMonthlyCreditCardPlans(),
  refreshNewcomerOffer(),
  // 月卡原生扫码门控只读这份目录；失败时保持关闭，不回退外部收银台。
  loadNativeCheckoutOffers().catch(() => {}),
  getAffiliateWallet().then((wallet) => { affiliateWallet.value = wallet }).catch(() => { affiliateWallet.value = null })
]).then(() => {
  syncTopupChannelWithSettings()
  syncMonthlyTopupChannelWithSettings()
})
</script>

<style scoped>
.topup-page {
  min-height: 100%;
  padding: 2rem 1.25rem 4rem;
  color: rgb(var(--color-gray-950));
  background: #fff;
}

.topup-shell {
  width: min(100%, 69rem);
  margin: 0 auto;
}

.topup-hero {
  margin-bottom: 1.75rem;
}

.topup-heading {
  color: rgb(var(--color-gray-950));
  font-size: clamp(2.15rem, 4vw, 3.15rem);
  font-weight: 650;
  letter-spacing: -0.045em;
  line-height: 1.08;
}

.topup-copy {
  max-width: 40rem;
  margin-top: 0.55rem;
  color: rgb(var(--color-gray-500));
  font-size: 0.96rem;
  line-height: 1.65;
}

.topup-copy--compact {
  margin-top: 0;
}

.topup-grid {
  display: grid;
  gap: 2.5rem;
  align-items: start;
}

.topup-main,
.topup-summary {
  min-width: 0;
}

.topup-catalog-tabs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 13px;
  background: #fff;
}

.topup-catalog-tab {
  min-height: 3.35rem;
  border: 1px solid transparent;
  border-radius: 12px;
  color: rgb(var(--color-gray-500));
  background: transparent;
  font-size: 0.95rem;
  font-weight: 650;
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease;
}

.topup-catalog-tab:hover {
  color: rgb(var(--color-gray-900));
}

.topup-catalog-tab--active {
  border-color: rgb(var(--color-primary-500) / 0.55);
  color: rgb(var(--color-primary-700));
  background: rgb(var(--color-primary-50) / 0.36);
}

.topup-section {
  margin-top: 1.65rem;
}

.topup-label {
  margin-bottom: 0.75rem;
  color: rgb(var(--color-gray-900));
  font-size: 0.95rem;
  font-weight: 680;
  letter-spacing: -0.01em;
}

.topup-product-list {
  display: grid;
  gap: 0.65rem;
}

.topup-product {
  display: grid;
  grid-template-columns: auto minmax(0, auto) minmax(0, 1fr) auto;
  gap: 0.85rem;
  align-items: center;
  width: 100%;
  min-height: 4.55rem;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 14px;
  padding: 0.85rem 1rem;
  color: rgb(var(--color-gray-900));
  background: #fff;
  text-align: left;
  transition: border-color 160ms ease, background-color 160ms ease;
}

.topup-product:hover {
  border-color: rgb(var(--color-gray-300));
  background: rgb(var(--color-gray-50) / 0.45);
}

.topup-product:focus-visible,
.topup-catalog-tab:focus-visible,
.topup-summary-method:focus-visible,
.topup-primary-action:focus-visible,
.topup-secondary-action:focus-visible,
.topup-quantity button:focus-visible {
  outline: 3px solid rgb(var(--color-primary-500) / 0.2);
  outline-offset: 2px;
}

.topup-product--active {
  border-color: rgb(var(--color-primary-500) / 0.72);
  background: rgb(var(--color-primary-50) / 0.28);
}

.topup-radio {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.35rem;
  height: 1.35rem;
  flex: 0 0 auto;
  border: 1.5px solid rgb(var(--color-gray-300));
  border-radius: 999px;
  background: #fff;
}

.topup-product--active .topup-radio,
.topup-summary-method--active .topup-radio {
  border-color: rgb(var(--color-primary-600));
}

.topup-product--active .topup-radio > span,
.topup-summary-method--active .topup-radio > span {
  width: 0.63rem;
  height: 0.63rem;
  border-radius: 999px;
  background: rgb(var(--color-primary-600));
}

.topup-product-name {
  min-width: 0;
  color: rgb(var(--color-gray-900));
  font-size: 0.96rem;
  font-weight: 650;
  white-space: nowrap;
}

.topup-product-price {
  justify-self: end;
  color: rgb(var(--color-gray-900));
  font-size: 0.96rem;
  font-weight: 680;
  white-space: nowrap;
}

.topup-promotion-badge {
  justify-self: start;
  border: 1px solid rgb(var(--color-primary-500) / 0.28);
  border-radius: 6px;
  padding: 0.16rem 0.42rem;
  color: rgb(var(--color-primary-700));
  background: rgb(var(--color-primary-50) / 0.72);
  font-size: 0.68rem;
  font-weight: 700;
  line-height: 1.35;
  white-space: nowrap;
}

.topup-monthly-catalog-note {
  margin-top: 0.85rem;
  color: rgb(var(--color-gray-500));
  font-size: 0.78rem;
  line-height: 1.6;
}

.topup-monthly-catalog-note a {
  color: rgb(var(--color-primary-700));
  font-weight: 650;
}

.topup-summary-card {
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 17px;
  padding: 1.7rem;
  background: #fff;
}

.topup-summary-kicker {
  color: rgb(var(--color-gray-900));
  font-size: 1.05rem;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.topup-summary-amount {
  margin-top: 1.1rem;
  color: rgb(var(--color-gray-950));
  font-size: clamp(2.6rem, 5vw, 3.5rem);
  font-weight: 690;
  letter-spacing: -0.055em;
  line-height: 1;
}

.topup-summary-rows {
  display: grid;
  gap: 0.95rem;
  margin-top: 1.7rem;
}

.topup-summary-row {
  display: flex;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
  color: rgb(var(--color-gray-500));
  font-size: 0.88rem;
}

.topup-summary-row strong {
  max-width: 66%;
  color: rgb(var(--color-gray-900));
  font-weight: 650;
  text-align: right;
}

.topup-summary-row--quantity {
  min-height: 2.55rem;
}

.topup-summary-divider {
  height: 1px;
  margin: 0.15rem 0;
  background: rgb(var(--color-gray-200));
}

.topup-credit-amount {
  color: rgb(var(--color-primary-700)) !important;
}

.topup-quantity {
  display: grid;
  grid-template-columns: 2.55rem 2.55rem 2.55rem;
  overflow: hidden;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 9px;
  background: #fff;
}

.topup-quantity button,
.topup-quantity output {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.35rem;
  color: rgb(var(--color-gray-900));
  background: transparent;
  font-size: 1rem;
  font-weight: 650;
}

.topup-quantity output {
  border-right: 1px solid rgb(var(--color-gray-200));
  border-left: 1px solid rgb(var(--color-gray-200));
  font-size: 0.9rem;
}

.topup-quantity button:hover:not(:disabled) {
  background: rgb(var(--color-gray-50));
}

.topup-quantity button:disabled {
  cursor: not-allowed;
  color: rgb(var(--color-gray-300));
}

.topup-summary-payment {
  margin-top: 1.7rem;
}

.topup-summary-methods {
  overflow: hidden;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 11px;
  background: #fff;
}

.topup-summary-method {
  display: grid;
  grid-template-columns: auto auto 1fr;
  gap: 0.72rem;
  align-items: center;
  width: 100%;
  min-height: 4rem;
  border-top: 1px solid rgb(var(--color-gray-200));
  padding: 0.75rem 0.85rem;
  color: rgb(var(--color-gray-900));
  background: transparent;
  text-align: left;
  transition: background-color 160ms ease;
}

.topup-summary-method:first-child {
  border-top: 0;
}

.topup-summary-method:hover:not(:disabled),
.topup-summary-method--active {
  background: rgb(var(--color-gray-50) / 0.6);
}

.topup-summary-method:disabled {
  cursor: not-allowed;
  opacity: 0.46;
}

.topup-summary-method :deep(.payment-method-icon) {
  width: 1.65rem;
  height: 1.65rem;
}

.topup-summary-method strong {
  font-size: 0.9rem;
  font-weight: 650;
}

.topup-primary-action,
.topup-secondary-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 3.5rem;
  border-radius: 10px;
  padding: 0.8rem 1rem;
  font-size: 0.94rem;
  font-weight: 680;
  transition: background-color 160ms ease, border-color 160ms ease, opacity 160ms ease;
}

.topup-primary-action {
  margin-top: 1.35rem;
  border: 1px solid rgb(var(--color-gray-950));
  color: #fff;
  background: rgb(var(--color-gray-950));
}

.topup-primary-action:hover:not(:disabled) {
  background: rgb(var(--color-gray-800));
}

.topup-secondary-action {
  margin-top: 0.7rem;
  border: 1px solid rgb(var(--color-gray-200));
  color: rgb(var(--color-gray-900));
  background: #fff;
}

.topup-secondary-action:hover:not(:disabled) {
  background: rgb(var(--color-gray-50));
}

.topup-primary-action:disabled,
.topup-secondary-action:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.topup-action-with-icon {
  gap: 0.55rem;
}

.topup-warning {
  margin-top: 0.7rem;
  color: rgb(var(--color-amber-700));
  font-size: 0.74rem;
  line-height: 1.5;
}

.topup-success {
  margin-top: 1.35rem;
  border-radius: 10px;
  padding: 0.9rem;
  color: rgb(var(--color-green-700));
  background: rgb(var(--color-green-50));
  font-size: 0.84rem;
  text-align: center;
}

.topup-newcomer-actions {
  margin-top: 1.35rem;
}

.topup-monthly-actions {
  margin-top: 0;
}

.topup-newcomer-actions :deep(.trial-offer),
.topup-monthly-actions :deep(.trial-offer) {
  border-color: rgb(var(--color-gray-200));
  border-radius: 12px;
  background: rgb(var(--color-gray-50) / 0.45);
  box-shadow: none;
}

.topup-qr-panel {
  display: grid;
  justify-items: center;
  min-height: 32rem;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 16px;
  padding: 2rem;
  background: #fff;
  text-align: center;
}

.topup-qr-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 1.5rem 0;
}

.topup-qr-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: min(18rem, 72vw);
  aspect-ratio: 1;
  border: 1px solid rgb(var(--color-gray-200));
  border-radius: 14px;
  padding: 1rem;
  background: #fff;
}

.topup-qr-image {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.topup-qr-empty {
  color: rgb(var(--color-gray-500));
  font-size: 0.88rem;
}

.dark .topup-page {
  color: rgb(var(--color-dark-50));
  background: rgb(var(--color-dark-950));
}

.dark .topup-heading,
.dark .topup-label,
.dark .topup-product-name,
.dark .topup-product-price,
.dark .topup-summary-kicker,
.dark .topup-summary-amount,
.dark .topup-summary-row strong,
.dark .topup-summary-method,
.dark .topup-quantity button,
.dark .topup-quantity output {
  color: rgb(var(--color-dark-50));
}

.dark .topup-copy,
.dark .topup-summary-row,
.dark .topup-monthly-catalog-note {
  color: rgb(var(--color-dark-300));
}

.dark .topup-catalog-tabs,
.dark .topup-product,
.dark .topup-summary-card,
.dark .topup-summary-methods,
.dark .topup-summary-method,
.dark .topup-quantity,
.dark .topup-quantity button,
.dark .topup-quantity output,
.dark .topup-secondary-action,
.dark .topup-qr-panel,
.dark .topup-qr-box {
  border-color: rgb(var(--color-dark-700));
  background: rgb(var(--color-dark-900));
}

.dark .topup-product:hover,
.dark .topup-summary-method:hover:not(:disabled),
.dark .topup-summary-method--active,
.dark .topup-quantity button:hover:not(:disabled),
.dark .topup-secondary-action:hover:not(:disabled) {
  background: rgb(var(--color-dark-800));
}

.dark .topup-product--active,
.dark .topup-catalog-tab--active {
  border-color: rgb(var(--color-primary-500) / 0.72);
  background: rgb(var(--color-primary-950) / 0.2);
}

.dark .topup-radio {
  border-color: rgb(var(--color-dark-500));
  background: rgb(var(--color-dark-900));
}

.dark .topup-primary-action {
  border-color: rgb(var(--color-dark-50));
  color: rgb(var(--color-dark-950));
  background: rgb(var(--color-dark-50));
}

@media (min-width: 1200px) {
  .topup-page {
    padding-top: 2.5rem;
  }

  .topup-grid {
    grid-template-columns: minmax(0, 1fr) minmax(19rem, 23rem);
    gap: clamp(2.5rem, 5vw, 4.5rem);
  }

  .topup-summary {
    position: sticky;
    top: 1.5rem;
  }
}

@media (max-width: 720px) {
  .topup-page {
    padding: 1.25rem 0.85rem 3rem;
  }

  .topup-heading {
    font-size: 2.2rem;
  }

  .topup-grid {
    gap: 1.5rem;
  }

  .topup-product {
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 0.65rem;
    min-height: 4.25rem;
  }

  .topup-promotion-badge,
  .topup-monthly-credits {
    grid-column: 2 / 3;
  }

  .topup-product-price {
    grid-column: 3;
    grid-row: 1 / span 2;
  }

  .topup-product-name {
    white-space: normal;
  }

  .topup-summary-card {
    padding: 1.25rem;
  }
}
</style>
