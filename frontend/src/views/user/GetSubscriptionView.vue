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
                      :class="{
                        'topup-product--active': selectedProductKind === 'balance' && selectedBalanceProduct?.id === product.id,
                        'topup-product--promotion': product.promotional
                      }"
                    >
                      <span class="topup-product-title topup-product-title--row">
                        <span>{{ product.label }}</span>
                        <span v-if="product.newcomerOnly" class="topup-promotion-badge">{{ t('topup.newcomerCardBadge') }}</span>
                        <span v-else-if="product.promotional" class="topup-promotion-badge">{{ t('topup.promotionalCardBadge') }}</span>
                      </span>
                      <span class="topup-product-desc">
                        <template v-if="product.newcomerOnly">
                          {{ t('topup.newcomerCardSummary', { paid: product.amountCny, credited: product.creditedAmountCny }) }}
                        </template>
                        <template v-else-if="product.promotional">
                          {{ t('topup.promotionalCardCredit', { credited: product.creditedAmountCny }) }} ·
                          {{ t('topup.promotionalCardBonus', { bonus: product.bonusAmountCny }) }}
                        </template>
                        <template v-else>{{ t('topup.cardShopAmount', { amount: product.amountCny }) }}</template>
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
                        {
                          'topup-monthly-product--active': selectedProductKind === 'monthly' && selectedMonthlyPlan?.id === plan.id,
                        }
                      ]"
                    >
                      <span class="topup-monthly-product__head">
                        <span>
                          <span v-if="plan.rarityLabel" class="topup-monthly-product__rarity">{{ plan.rarityLabel }}</span>
                          <span class="topup-product-title">{{ plan.name }}</span>
                        </span>
                        <span class="topup-status topup-status--available">{{ t('topup.monthlyPlanStatus') }}</span>
                      </span>
                      <span class="topup-monthly-product__price">{{ plan.directPrice }} <small>/ 31 天</small></span>
                      <span class="topup-monthly-product__credits">
                        {{ t('topup.monthlyPlanCreditsValue', { amount: plan.displayMonthlyCreditsText }) }}
                      </span>
                      <span v-if="plan.legendaryCopy" class="topup-monthly-product__legend">
                        {{ plan.description }}
                      </span>
                    </button>
                  </div>
                  <p class="topup-monthly-catalog-note">
                    {{ t('topup.monthlyPlanSharedPool') }}。
                    {{ t('topup.monthlyPlanModelsPrefix') }}
                    <RouterLink to="/models">{{ t('topup.monthlyPlanModelsLink') }}</RouterLink>。
                  </p>
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
                <p v-if="qrCodeClientRendered && !qrExpired" class="topup-copy topup-copy--compact">
                  {{ t('topup.qrContentHint') }}
                </p>
                <div class="topup-amount-row">
                  <div>
                    <p class="topup-meta-label">{{ t('topup.selectAmount') }}</p>
                    <p class="topup-meta-value">¥{{ displayAmountText }}</p>
                  </div>
                  <div>
                    <p class="topup-meta-label">平台额度</p>
                    <p class="topup-meta-value topup-meta-value--accent">⚡{{ displayCreditedText }}</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="topup-summary">
              <div
                class="topup-summary-card"
              >
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
                    ⚡{{ displayCreditedText }}
                  </div>
                  <div v-else-if="selectedProductKind === 'monthly'" class="topup-price-chip topup-price-chip--muted">
                    {{ selectedMonthlyPlan?.rarityLabel ?? t('topup.monthlyPlanStatus') }}
                  </div>
                </div>

                <div class="topup-summary-rows">
                  <template v-if="selectedProductKind === 'monthly'">
                    <section
                      class="topup-monthly-detail"
                    >
                      <p v-if="selectedMonthlyPlan?.legendaryCopy" class="topup-apex-lore">
                        {{ selectedMonthlyPlan.legendaryCopy }}
                      </p>

                      <div class="topup-monthly-prices">
                        <div>
                          <span>{{ t('topup.monthlyPlanPrice') }}</span>
                          <strong>{{ selectedMonthlyPlan?.price }} / 31 天</strong>
                        </div>
                        <div>
                          <span>{{ t('topup.monthlyPlanDirectPrice') }}</span>
                          <strong>{{ selectedMonthlyPlan?.directPrice }} / 31 天</strong>
                        </div>
                      </div>

                      <div
                        class="topup-quota-grid"
                        :class="{ 'topup-quota-grid--single': !selectedMonthlyPlan?.showWeeklyLimit }"
                      >
                        <div v-if="selectedMonthlyPlan?.showWeeklyLimit" class="topup-monthly-quota">
                          <span>{{ t('topup.monthlyPlanWeeklyLimit') }}</span>
                          <strong>{{ selectedMonthlyPlan?.displayWeeklyCreditsText }} AI credits / 周</strong>
                          <small>{{ t('topup.monthlyPlanSharedPool') }}</small>
                        </div>
                        <div class="topup-monthly-quota">
                          <span>{{ t('topup.monthlyPlanMonthlyLimit') }}</span>
                          <strong>
                            {{ t('topup.monthlyPlanCreditsValue', { amount: selectedMonthlyPlan?.displayMonthlyCreditsText }) }}
                          </strong>
                          <small>{{ t('topup.monthlyPlanSharedPool') }}</small>
                          <small class="topup-monthly-quota__models">
                            {{ t('topup.monthlyPlanModelsPrefix') }}
                            <RouterLink to="/models">{{ t('topup.monthlyPlanModelsLink') }}</RouterLink>。
                          </small>
                        </div>
                      </div>

                    </section>
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

                <section
                  v-if="selectedProductKind === 'balance' && !selectedNewcomerNative && step === 1"
                  class="topup-summary-payment"
                  aria-labelledby="topup-summary-payment-title"
                >
                  <p id="topup-summary-payment-title" class="topup-label">
                    {{ t('topup.selectPayType') }}
                  </p>
                  <div class="topup-summary-methods">
                    <button
                      type="button"
                      data-testid="topup-method-alipay"
                      @click="selectTopupChannel('alipay')"
                      :disabled="!canUseAlipay || effectiveAmountYuan < 20"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': selectedTopupChannel === 'alipay' }"
                    >
                      <PaymentMethodIcon kind="alipay" />
                      <span>
                        <strong>{{ t('topup.alipayScanTitle') }}</strong>
                        <small>{{ canUseAlipay && effectiveAmountYuan >= 20 ? t('topup.availableNow') : t('topup.comingSoon') }}</small>
                      </span>
                    </button>

                    <button
                      type="button"
                      data-testid="topup-method-wechat"
                      @click="selectTopupChannel('wechat')"
                      :disabled="!canUseWechat || effectiveAmountYuan < 20"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': selectedTopupChannel === 'wechat' }"
                    >
                      <PaymentMethodIcon kind="wechat" />
                      <span>
                        <strong>{{ t('topup.wechatScanTitle') }}</strong>
                        <small>{{ canUseWechat && effectiveAmountYuan >= 20 ? t('topup.availableNow') : t('topup.comingSoon') }}</small>
                      </span>
                    </button>

                    <button
                      type="button"
                      data-testid="topup-method-card_shop"
                      @click="selectTopupChannel('card_shop')"
                      :disabled="!canUseCardShopForSelected"
                      class="topup-summary-method"
                      :class="{ 'topup-summary-method--active': showingCardShop }"
                    >
                      <PaymentMethodIcon kind="backup" />
                      <span>
                        <strong>{{ t('topup.cardShopChannelTitle') }}</strong>
                        <small>{{ canUseCardShopForSelected ? t('topup.availableNow') : t('topup.comingSoon') }}</small>
                      </span>
                    </button>
                  </div>
                  <p v-if="paymentNotice" class="topup-warning">{{ paymentNotice }}</p>
                </section>

                <div v-if="selectedProductKind === 'monthly'" class="topup-monthly-actions">
                  <NativeCheckoutTrialOffer
                    v-if="selectedMonthlyNativeOffer"
                    :key="selectedMonthlyPlan.id"
                    :offer-code="selectedMonthlyPlan.id"
                    compact
                  />
                  <button
                    @click="openSelectedMonthlyCardShop"
                    :disabled="!canOpenSelectedMonthlyCardShop"
                    class="topup-primary-action topup-monthly-action topup-action-with-icon"
                  >
                    <PaymentMethodIcon kind="backup" />
                    <span>{{ t('topup.cardShopAction') }}</span>
                  </button>
                </div>

                <div v-else-if="selectedNewcomerNative" class="topup-newcomer-actions">
                  <NativeCheckoutTrialOffer compact />
                  <button
                    v-if="canUseCardShopForSelected"
                    type="button"
                    @click="openSelectedCardShopProduct"
                    :disabled="openingCardShop"
                    class="topup-secondary-action topup-backup-action topup-action-with-icon"
                  >
                    <PaymentMethodIcon kind="backup" />
                    <span>{{ t('topup.cardShopAction') }}</span>
                  </button>
                  <button
                    type="button"
                    @click="goRedeem"
                    class="topup-secondary-action topup-secondary-action--stacked"
                  >
                    {{ t('topup.cardShopGoRedeem') }}
                  </button>
                  <p class="topup-warning">{{ t('topup.newcomerCardLimit') }}</p>
                </div>

                <template v-else-if="showingCardShop">
                  <button
                    @click="openSelectedCardShopProduct"
                    :disabled="!selectedCardShopProduct || openingCardShop"
                    class="topup-primary-action topup-action-with-icon"
                  >
                    <PaymentMethodIcon kind="backup" />
                    <span>{{ t('topup.cardShopAction') }}</span>
                  </button>
                  <button
                    @click="goRedeem"
                    class="topup-secondary-action topup-secondary-action--stacked"
                  >
                    {{ t('topup.cardShopGoRedeem') }}
                  </button>
                  <p v-if="selectedBalanceProduct?.newcomerOnly" class="topup-warning">
                    {{ t('topup.newcomerCardLimit') }}
                  </p>
                </template>

                <section v-else-if="step === 1" class="topup-inline-pay">
                  <button
                    data-testid="topup-submit"
                    @click="submitOrder"
                    :disabled="submitting || !canUseSelectedQrMethod || !!amountError || effectiveAmountYuan < 20"
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PaymentMethodIcon from '@/components/user/PaymentMethodIcon.vue'
import { createTopupOrder, queryTopupOrderStatus, type TopupPayType } from '@/api/topup'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CardShopProduct } from '@/types'
import {
  BALANCE_TOPUP_PRESETS,
  PROMOTIONAL_BALANCE_TOPUPS,
  getCreditedBalanceTopupAmount,
  isNewcomerBalanceTopup,
  isSupportedBalanceTopupAmount
} from '@/constants/balanceTopups'
import { type MonthlyCreditCardPlan } from '@/constants/monthlyCreditCards'
import { useMonthlyCreditCardPlans } from '@/composables/useMonthlyCreditCardPlans'
import { shouldShowManualNewcomerProduct, useManualNewcomerOffer } from '@/composables/useManualNewcomerOffer'
import { useNativeCheckoutOffers } from '@/composables/useNativeCheckoutOffers'
import NativeCheckoutTrialOffer from '@/components/user/NativeCheckoutTrialOffer.vue'
import { isDirectQrImageUrl, renderQrCodeDataUrl } from '@/utils/qrImage'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const router = useRouter()

const presets = BALANCE_TOPUP_PRESETS
const QR_TTL_SECONDS = 300
type TopupChannel = 'card_shop' | TopupPayType
type SelectedProductKind = 'balance' | 'monthly'
type BalanceProduct = {
  id: string
  label: string
  amountCny: number
  cardShopProduct?: CardShopProduct
  creditedAmountCny?: number
  bonusAmountCny?: number
  promotional?: boolean
  newcomerOnly?: boolean
}

const step = ref<1 | 2>(1)
const selectedTopupChannel = ref<TopupChannel>('alipay')
const selectedProductKind = ref<SelectedProductKind>('balance')
const selectedBalanceProductId = ref('')
const selectedMonthlyPlanId = ref<MonthlyCreditCardPlan['id']>('plus')
const payType = ref<TopupPayType>('alipay')
const submitting = ref(false)
const openingCardShop = ref(false)
const qrCodeURL = ref('')
const qrCodeRawPayload = ref('')
const qrCodeClientRendered = ref(false)
const orderNo = ref('')
const qrExpired = ref(false)
const countdown = ref(QR_TTL_SECONDS)
const activeOrderAmountYuan = ref(0)
const activeOrderCreditedAmountYuan = ref(0)
const { plans: monthlyCreditCardPlans, loadMonthlyCreditCardPlans } = useMonthlyCreditCardPlans()
const { loadOffers: loadNativeCheckoutOffers, findNativeOfferByCode } = useNativeCheckoutOffers()
const {
  state: newcomerOfferState,
  mode: newcomerOfferMode,
  refresh: refreshNewcomerOffer,
  requestPurchaseURL: requestNewcomerPurchaseURL,
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
const configuredCardShopProducts = computed<CardShopProduct[]>(() =>
  [...(appStore.cachedPublicSettings?.card_shop_products ?? [])]
    .filter(
      (product) =>
        product.enabled &&
        product.url &&
        isSupportedBalanceTopupAmount(product.amount_cny)
    )
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.amount_cny - b.amount_cny)
)
const activeCardShopProducts = computed<CardShopProduct[]>(() =>
  configuredCardShopProducts.value.filter((product) =>
    shouldShowManualNewcomerProduct(product.amount_cny, newcomerOfferState.value, newcomerOfferMode.value)
  )
)
const newcomerBackupProduct = computed(() => {
  if (newcomerOfferState.value !== 'available') return undefined
  return configuredCardShopProducts.value.find((product) => isNewcomerBalanceTopup(product.amount_cny))
})
const cardShopMode = computed(
  () => (appStore.cachedPublicSettings?.card_shop_enabled ?? false) && activeCardShopProducts.value.length > 0
)
const balanceProducts = computed<BalanceProduct[]>(() => {
  const promotionalAmounts = new Set(PROMOTIONAL_BALANCE_TOPUPS.map((product) => product.paidAmountCny))
  const standardProducts: BalanceProduct[] = activeCardShopProducts.value.length > 0
    ? activeCardShopProducts.value
        .filter((product) => !promotionalAmounts.has(product.amount_cny))
        .map((product) => ({
          id: product.id,
          label: product.label || `¥${product.amount_cny} 余额卡`,
          amountCny: product.amount_cny,
          cardShopProduct: product
        }))
    : presets.map((amount) => ({
        id: `qr-${amount}`,
        label: `¥${amount} 余额卡`,
        amountCny: amount
      }))
  const promotionalProducts: BalanceProduct[] = PROMOTIONAL_BALANCE_TOPUPS.map((product) => {
    const cardShopProduct = product.newcomerOnly && newcomerOfferMode.value === 'native'
      ? newcomerBackupProduct.value
      : activeCardShopProducts.value.find((candidate) => candidate.amount_cny === product.paidAmountCny)
    return {
      id: `promotion-${product.paidAmountCny}`,
      label: product.newcomerOnly
        ? t('topup.newcomerCardTitle', { credited: product.creditedAmountCny })
        : t('topup.promotionalCardTitle', { paid: product.paidAmountCny }),
      amountCny: product.paidAmountCny,
      creditedAmountCny: product.creditedAmountCny,
      bonusAmountCny: product.bonusAmountCny,
      promotional: true,
      newcomerOnly: product.newcomerOnly,
      cardShopProduct
    }
  })
  const visiblePromotionalProducts = promotionalProducts.filter(
    (product) =>
      !product.newcomerOnly ||
      (newcomerOfferState.value === 'available' && (
        newcomerOfferMode.value === 'native' || !!product.cardShopProduct
      ))
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
const selectedCardShopProduct = computed(() => selectedBalanceProduct.value?.cardShopProduct)
const selectedNewcomerNative = computed(
  () => selectedProductKind.value === 'balance' &&
    selectedBalanceProduct.value?.newcomerOnly === true &&
    newcomerOfferMode.value === 'native'
)
const selectedMonthlyPlan = computed(
  () => monthlyCreditCardPlans.value.find((plan) => plan.id === selectedMonthlyPlanId.value) ?? monthlyCreditCardPlans.value[0]
)
// 约定：月卡套餐的原生结账 offer code 与前端套餐 id 相同（plus/pro/max）。
// 只有可见且 provider=easypay 的订阅 offer 命中；命中后购买入口切换为站内
// 扫码（NativeCheckoutTrialOffer），未命中保持链动小铺外链回退。
const selectedMonthlyNativeOffer = computed(() => {
  if (selectedProductKind.value !== 'monthly') return undefined
  const plan = selectedMonthlyPlan.value
  if (!plan) return undefined
  return findNativeOfferByCode(plan.id, 'subscription')
})
const selectedMonthlyCardShopUrl = computed(() => selectedMonthlyPlan.value?.cardShopUrl || '')
const canOpenSelectedMonthlyCardShop = computed(() => selectedMonthlyCardShopUrl.value.trim() !== '')
const canUseCardShopForSelected = computed(() => cardShopMode.value && !!selectedCardShopProduct.value)
const showingCardShop = computed(
  () => selectedProductKind.value === 'balance' && selectedTopupChannel.value === 'card_shop' && canUseCardShopForSelected.value
)
const showingQrTopup = computed(
  () => selectedProductKind.value === 'balance' && (selectedTopupChannel.value === 'alipay' || selectedTopupChannel.value === 'wechat')
)
const canUseSelectedQrMethod = computed(() => {
  if (!showingQrTopup.value) return false
  return selectedTopupChannel.value === 'alipay' ? canUseAlipay.value : canUseWechat.value
})
const selectedTopupMethodLabel = computed(() => {
  if (selectedNewcomerNative.value) return t('nativeCheckout.qrPayment')
  if (showingCardShop.value) return t('topup.cardShopChannelTitle')
  if (selectedTopupChannel.value === 'alipay') return t('topup.alipayScanTitle')
  if (selectedTopupChannel.value === 'wechat') return t('topup.wechatScanTitle')
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
const displayCreditedAmountYuan = computed(() => {
  if (step.value === 2 && activeOrderCreditedAmountYuan.value > 0) {
    return activeOrderCreditedAmountYuan.value
  }
  return getCreditedBalanceTopupAmount(displayAmountYuan.value)
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

function selectBalanceProduct(product: BalanceProduct) {
  selectedProductKind.value = 'balance'
  selectedBalanceProductId.value = product.id
  syncTopupChannelWithSettings()
}

function selectMonthlyPlan(plan: MonthlyCreditCardPlan) {
  selectedProductKind.value = 'monthly'
  selectedMonthlyPlanId.value = plan.id
}

function selectTopupChannel(channel: TopupChannel) {
  if (channel === 'card_shop' && !canUseCardShopForSelected.value) return
  if (channel === 'alipay' && (!canUseAlipay.value || effectiveAmountYuan.value < 20)) return
  if (channel === 'wechat' && (!canUseWechat.value || effectiveAmountYuan.value < 20)) return
  selectedTopupChannel.value = channel
  if (channel === 'alipay' || channel === 'wechat') {
    payType.value = channel
  }
}

async function openSelectedCardShopProduct() {
  const product = selectedCardShopProduct.value
  if (!product?.url || openingCardShop.value) return
  if (!isNewcomerBalanceTopup(product.amount_cny)) {
    window.location.assign(product.url)
    return
  }
  openingCardShop.value = true
  try {
    await refreshNewcomerOffer()
    if (newcomerOfferState.value === 'claimed') {
      appStore.showInfo(t('topup.newcomerCardClaimed'))
      return
    }
    if (newcomerOfferMode.value === 'native' && newcomerOfferState.value === 'available') {
      window.location.assign(product.url)
      return
    }
    const purchaseURL = await requestNewcomerPurchaseURL()
    if (!purchaseURL) {
      syncTopupChannelWithSettings()
      if (String(newcomerOfferState.value) === 'claimed') {
        appStore.showInfo(t('topup.newcomerCardClaimed'))
      } else {
        appStore.showError(t('topup.newcomerCardStatusUnavailable'))
      }
      return
    }
    window.location.assign(purchaseURL)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('topup.newcomerCardStatusUnavailable')))
  } finally {
    openingCardShop.value = false
  }
}

function openSelectedMonthlyCardShop() {
  const url = selectedMonthlyCardShopUrl.value.trim()
  if (!url) return
  window.location.assign(url)
}

function goRedeem() {
  router.push('/redeem')
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
  if (selectedTopupChannel.value === 'card_shop' && canUseCardShopForSelected.value) {
    return
  }
  if (canUseAlipay.value && amountSupportsScan) {
    selectedTopupChannel.value = 'alipay'
    payType.value = 'alipay'
  } else if (canUseWechat.value && amountSupportsScan) {
    selectedTopupChannel.value = 'wechat'
    payType.value = 'wechat'
  } else if (canUseCardShopForSelected.value) {
    selectedTopupChannel.value = 'card_shop'
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
  payType.value = selectedTopupChannel.value as TopupPayType

  submitting.value = true
  try {
    const orderAmountYuan = effectiveAmountYuan.value
    const response = await createTopupOrder(Math.round(orderAmountYuan * 100), payType.value)
    orderNo.value = response.order_no
    activeOrderAmountYuan.value = orderAmountYuan
    activeOrderCreditedAmountYuan.value = getCreditedBalanceTopupAmount(orderAmountYuan)
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
  // 月卡原生扫码门控只读这份目录；失败时保持链动小铺外链回退，不影响页面。
  loadNativeCheckoutOffers().catch(() => {})
]).then(() => {
  syncTopupChannelWithSettings()
})
</script>

<style scoped>
.topup-page {
  position: relative;
  overflow: hidden;
  padding: 1rem 0.75rem;
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
    radial-gradient(circle at 35% 0%, rgb(var(--color-terracotta) / 0.14), transparent 58%),
    radial-gradient(circle at 70% 24%, rgb(var(--color-laurel) / 0.12), transparent 52%);
}

.dark .topup-page::before {
  opacity: 0.26;
}

.topup-shell {
  position: relative;
  max-width: 76rem;
  margin: 0 auto;
}

.topup-panel {
  overflow: hidden;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background:
    linear-gradient(135deg, var(--admin-surface, rgb(var(--color-marble) / 0.96)), var(--admin-surface-soft, rgb(var(--color-stone) / 0.78)));
  box-shadow: var(--admin-shadow, 0 18px 48px rgb(var(--shadow-ink) / 0.11));
  color: var(--admin-ink, rgb(var(--color-ink)));
}

.topup-grid {
  display: grid;
}

.topup-main {
  padding: 1.25rem;
  border-bottom: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
}

.topup-summary {
  padding: 1.25rem;
  background:
    linear-gradient(180deg, rgb(var(--color-terracotta) / 0.045), rgb(var(--color-laurel) / 0.035)),
    var(--admin-surface-soft, rgb(var(--color-stone) / 0.78));
}

.topup-eyebrow {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  border-radius: 4px;
  padding: 0.35rem 0.65rem;
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  background: rgb(var(--color-terracotta) / 0.08);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.topup-heading {
  margin-top: 0.65rem;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: clamp(1.75rem, 3vw, 2.2rem);
  font-weight: 650;
  line-height: 1.16;
  letter-spacing: 0;
}

.topup-copy {
  max-width: 38rem;
  margin-top: 0.45rem;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.95rem;
  line-height: 1.55;
}

.topup-copy--compact {
  margin-top: 0;
}

.topup-section {
  margin-top: 1.1rem;
}

.topup-section--stack {
  display: grid;
  gap: 1.15rem;
}

.topup-label {
  margin-bottom: 0.55rem;
  color: var(--admin-ink, rgb(var(--color-ink)));
  font-size: 0.92rem;
  font-weight: 650;
}

.topup-channel-grid,
.topup-products,
.topup-presets,
.topup-plan-grid {
  display: grid;
  gap: 0.55rem;
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

.topup-choice,
.topup-product,
.topup-pay-option {
  width: 100%;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  color: var(--admin-ink, rgb(var(--color-ink)));
  text-align: left;
  transition:
    border-color var(--duration-base) var(--ease-standard),
    background-color var(--duration-base) var(--ease-standard),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.topup-choice:hover,
.topup-product:hover,
.topup-pay-option:hover {
  border-color: var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  background: var(--admin-parchment, rgb(var(--color-parchment)));
  transform: translateY(-1px);
  box-shadow: var(--admin-shadow-sm, 0 8px 24px rgb(var(--shadow-ink) / 0.08));
}

.topup-choice:focus-visible,
.topup-product:focus-visible,
.topup-monthly-product:focus-visible,
.topup-pay-option:focus-visible,
.topup-input:focus-visible,
.topup-primary-action:focus-visible,
.topup-secondary-action:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--admin-ring, rgb(var(--color-terracotta) / 0.24));
}

.topup-choice {
  display: flex;
  gap: 0.65rem;
  align-items: flex-start;
  min-height: 5.4rem;
  padding: 0.7rem;
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
  border-color: var(--admin-terracotta, rgb(var(--color-terracotta)));
  background:
    linear-gradient(180deg, rgb(var(--color-terracotta) / 0.1), transparent 100%),
    var(--admin-control, rgb(var(--color-vellum) / 0.95));
  box-shadow: inset 0 0 0 1px rgb(var(--color-terracotta) / 0.12);
}

.topup-choice-icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: rgb(var(--color-terracotta) / 0.08);
}

.topup-choice-icon--warm {
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
}

.topup-choice-icon--laurel {
  color: var(--admin-laurel, rgb(var(--color-laurel)));
  background: rgb(var(--color-laurel) / 0.1);
}

.topup-choice-icon--alipay {
  color: rgb(var(--color-info));
  background: rgb(var(--color-info) / 0.1);
}

.topup-choice-icon--wechat {
  color: rgb(var(--color-laurel));
  background: rgb(var(--color-laurel) / 0.1);
}

.topup-choice-content {
  min-width: 0;
}

.topup-choice-title,
.topup-product-title {
  display: block;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-weight: 700;
  line-height: 1.4;
}

.topup-choice-title {
  font-size: 0.9rem;
}

.topup-choice-desc,
.topup-product-desc {
  display: block;
  margin-top: 0.2rem;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.75rem;
  line-height: 1.35;
}

.topup-status {
  display: inline-flex;
  margin-top: 0.45rem;
  border: 1px solid transparent;
  border-radius: 4px;
  padding: 0.18rem 0.42rem;
  font-size: 0.68rem;
  font-weight: 700;
}

.topup-status--available {
  border-color: rgb(var(--color-laurel) / 0.24);
  background: rgb(var(--color-laurel) / 0.1);
  color: var(--admin-laurel-dark, rgb(var(--color-laurel-dark)));
}

.topup-status--disabled {
  border-color: var(--admin-border, rgb(var(--color-ink) / 0.14));
  background: var(--admin-surface-soft, rgb(var(--color-stone) / 0.78));
  color: var(--admin-muted, rgb(var(--color-muted)));
}

.topup-product {
  min-height: 4rem;
  padding: 0.7rem 0.8rem;
}

.topup-product--preset {
  min-height: 4.4rem;
}

.topup-product-title {
  font-size: 0.95rem;
}

.topup-product-title--row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: flex-start;
  gap: 0.75rem;
}

.topup-product-title--row > span:first-child {
  flex: 1 1 100%;
  min-width: 0;
}

.topup-product--promotion {
  border-color: rgb(var(--color-terracotta) / 0.42);
  background:
    linear-gradient(135deg, rgb(var(--color-terracotta) / 0.1), transparent 64%),
    var(--admin-control, rgb(var(--color-vellum) / 0.95));
}

.topup-promotion-badge {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--color-terracotta) / 0.28);
  border-radius: 999px;
  padding: 0.18rem 0.5rem;
  background: rgb(var(--color-terracotta) / 0.1);
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  font-size: 0.7rem;
  font-weight: 750;
}

.topup-monthly-product {
  position: relative;
  overflow: hidden;
  min-height: 6.75rem;
  width: 100%;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  color: var(--admin-ink, rgb(var(--color-ink)));
  padding: 0.75rem;
  text-align: left;
  transition:
    border-color var(--duration-base) var(--ease-standard),
    background-color var(--duration-base) var(--ease-standard),
    box-shadow var(--duration-base) var(--ease-out),
    transform var(--duration-base) var(--ease-out);
}

.topup-monthly-product::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 0.25rem;
  background: var(--admin-muted, rgb(var(--color-muted)));
  /* 顶栏是这张卡的身份色，hover 时加粗到三倍——比整卡换底色克制，
   * 但视线一定会被它带过去。transform 而不是改 height，避免触发布局。 */
  transform-origin: top;
  transition: transform var(--duration-base) var(--ease-out);
}

.topup-monthly-product:not(.topup-monthly-product--apex):hover::before,
.topup-monthly-product:not(.topup-monthly-product--apex).topup-monthly-product--active::before {
  transform: scaleY(3);
}

/* 扫光：一道极淡的斜向高光从左掠过，只在 hover 时跑一次。
 * 纸面系统里不能做成"发光卡"，所以峰值只有 7%，掠过即止。 */
.topup-monthly-product:not(.topup-monthly-product--apex)::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(
    105deg,
    transparent 38%,
    rgb(var(--color-vellum) / 0.07) 48%,
    transparent 58%
  );
  transform: translateX(-120%);
  opacity: 0;
}

/* apex 卡自己有一条 7s 常驻扫光（topup-apex-sweep），不叠这一层 */
.topup-monthly-product:not(.topup-monthly-product--apex):hover::after {
  animation: cardSheen 620ms var(--ease-out) both;
}

@keyframes cardSheen {
  from { transform: translateX(-120%); opacity: 1; }
  to   { transform: translateX(120%); opacity: 1; }
}

.topup-monthly-product--lite::before {
  background: rgb(var(--color-laurel));
}

.topup-monthly-product--pro::before {
  background: rgb(var(--color-info));
}

.topup-monthly-product--max::before {
  background: rgb(var(--color-terracotta));
}

.topup-monthly-product--ultra::before {
  background: rgb(var(--color-ink-deep));
}

.topup-monthly-product--apex {
  grid-column: 1 / -1;
  isolation: isolate;
  min-height: 12.4rem;
  border-color: rgb(var(--gild-600) / 0.52);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.14), rgb(var(--color-red-700) / 0.12) 42%, transparent 70%),
    linear-gradient(118deg, rgb(var(--lacquer-rest-top)) 0%, rgb(var(--lacquer-rest-mid)) 48%, rgb(var(--lacquer-base)) 100%);
  color: rgb(var(--gild-100));
  box-shadow:
    0 18px 48px rgb(var(--shadow-ink) / 0.2),
    inset 0 0 0 1px rgb(var(--gild-500) / 0.14);
}

.topup-monthly-product--apex::before {
  height: 0.32rem;
  background: linear-gradient(90deg, rgb(var(--gild-900)), rgb(var(--gild-600)) 25%, rgb(var(--gild-400)) 52%, rgb(var(--gild-700)) 78%, rgb(var(--lacquer-base)));
  box-shadow: 0 0 22px rgb(var(--gild-600) / 0.42);
}

.topup-monthly-product--apex::after {
  content: "";
  position: absolute;
  z-index: -1;
  inset: 0;
  opacity: 0.58;
  background:
    linear-gradient(95deg, transparent 0%, rgb(var(--gild-500) / 0.2) 45%, transparent 58%),
    repeating-linear-gradient(90deg, rgb(var(--gild-500) / 0.08) 0 1px, transparent 1px 2.8rem);
  transform: translateX(-46%);
  animation: topup-apex-sweep 7s ease-in-out infinite;
}

/* 抬起 1px 等于没抬 —— 这是"卡片没有动效"的直接原因。6px 加上一层随之
 * 变深、变散的投影，才读得出"纸被拿起来了"。位移之外不加 scale：
 * 纸不会变大，而且缩放会让 1px 边框和小字发虚。 */
.topup-monthly-product:hover {
  border-color: var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  background: var(--admin-parchment, rgb(var(--color-parchment)));
  transform: translateY(-6px);
  box-shadow: 0 16px 34px rgb(var(--shadow-ink) / 0.16);
}

.topup-monthly-product:active {
  transform: translateY(-2px);
  transition-duration: var(--duration-instant);
}

/* 选中态比 hover 更高、投影更实，并且不随鼠标移开而落下 —— 选中是状态，
 * hover 是反馈，两者叠加时选中态应当占上风。 */
.topup-monthly-product--active {
  border-color: var(--admin-terracotta, rgb(var(--color-terracotta)));
  background:
    linear-gradient(180deg, rgb(var(--color-terracotta) / 0.1), transparent 100%),
    var(--admin-control, rgb(var(--color-vellum) / 0.95));
  transform: translateY(-4px);
  box-shadow:
    inset 0 0 0 1px rgb(var(--color-terracotta) / 0.12),
    0 0 0 3px rgb(var(--color-terracotta) / 0.14),
    0 14px 30px rgb(var(--shadow-ink) / 0.14);
}

.topup-monthly-product--active:hover {
  transform: translateY(-7px);
  box-shadow:
    inset 0 0 0 1px rgb(var(--color-terracotta) / 0.16),
    0 0 0 3px rgb(var(--color-terracotta) / 0.2),
    0 18px 38px rgb(var(--shadow-ink) / 0.18);
}

.topup-monthly-product--apex:hover {
  border-color: rgb(var(--gild-500) / 0.82);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.2), rgb(var(--color-terracotta) / 0.14) 44%, transparent 72%),
    linear-gradient(118deg, rgb(var(--lacquer-hover-top)) 0%, rgb(var(--lacquer-hover-mid)) 48%, rgb(var(--lacquer-base)) 100%);
  box-shadow:
    0 22px 58px rgb(var(--shadow-ink) / 0.24),
    inset 0 0 0 1px rgb(var(--gild-500) / 0.18);
}

.topup-monthly-product--apex.topup-monthly-product--active {
  border-color: rgb(var(--gild-500));
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.24), rgb(var(--color-terracotta) / 0.16) 46%, transparent 72%),
    linear-gradient(118deg, rgb(var(--lacquer-active-top)) 0%, rgb(var(--lacquer-active-mid)) 50%, rgb(var(--lacquer-base)) 100%);
  box-shadow:
    0 0 0 1px rgb(var(--gild-500) / 0.36),
    0 22px 66px rgb(var(--shadow-ink) / 0.28),
    inset 0 0 34px rgb(var(--gild-500) / 0.1);
  transform: translateY(-2px);
}

.topup-monthly-product--apex-activate {
  animation: topup-apex-card-summon 0.94s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

.topup-monthly-product--apex-activate::before {
  animation: topup-apex-edge-flare 0.94s ease-out both;
}

.topup-monthly-product__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.topup-monthly-product__rarity {
  display: block;
  margin-bottom: 0.32rem;
  color: rgb(var(--gild-500));
  font-size: 0.64rem;
  font-weight: 900;
  letter-spacing: 0.16em;
  line-height: 1;
  text-transform: uppercase;
}

.topup-monthly-product--apex .topup-product-title {
  color: rgb(var(--gild-200));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.35rem;
  letter-spacing: 0;
  text-shadow: 0 0 18px rgb(var(--gild-500) / 0.2);
}

.topup-monthly-product--apex .topup-status--available {
  border-color: rgb(var(--gild-500) / 0.38);
  background: rgb(var(--gild-500) / 0.1);
  color: rgb(var(--gild-500));
}

.topup-monthly-product__price {
  display: block;
  margin-top: 0.75rem;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: 1.35rem;
  font-weight: 850;
  line-height: 1;
}

.topup-monthly-product--apex .topup-monthly-product__price {
  color: rgb(var(--gild-300));
  font-size: 2rem;
}

.topup-monthly-product__price small {
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.82rem;
  font-weight: 650;
}

.topup-monthly-product__credits {
  display: block;
  margin-top: 0.42rem;
  color: var(--admin-ink, rgb(var(--color-ink)));
  font-size: 0.78rem;
  font-weight: 780;
  line-height: 1.4;
}

.topup-monthly-catalog-note {
  margin: 0.45rem 0 0;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.8rem;
  font-weight: 620;
  line-height: 1.4;
}

.topup-monthly-catalog-note a,
.topup-monthly-quota__models a {
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  font-weight: 760;
  text-decoration: underline;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.18em;
}

.topup-monthly-product--apex .topup-monthly-product__price small {
  color: rgb(var(--gild-200) / 0.72);
}

.topup-monthly-product__legend {
  display: block;
  max-width: 34rem;
  margin-top: 0.75rem;
  color: rgb(var(--gild-200) / 0.72);
  font-size: 0.82rem;
  font-weight: 650;
  line-height: 1.7;
}

.topup-warning,
.topup-success {
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: rgb(var(--color-terracotta) / 0.08);
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  padding: 0.9rem 1rem;
  font-size: 0.9rem;
  line-height: 1.75;
}

.topup-warning {
  background: rgb(var(--color-warning) / 0.12);
  color: var(--admin-warning, rgb(var(--color-warning)));
}

.topup-success {
  background: rgb(var(--color-laurel) / 0.12);
  color: var(--admin-laurel-dark, rgb(var(--color-laurel-dark)));
}

.topup-input-wrap {
  position: relative;
}

.topup-currency {
  position: absolute;
  top: 50%;
  left: 1rem;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 1rem;
  font-weight: 700;
  transform: translateY(-50%);
}

.topup-input {
  width: 100%;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  padding: 0.95rem 1rem 0.95rem 2.8rem;
  font-size: 1rem;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease;
}

.topup-input::placeholder {
  color: var(--admin-muted, rgb(var(--color-muted)));
  opacity: 0.72;
}

.topup-input--active,
.topup-input:focus {
  border-color: var(--admin-terracotta, rgb(var(--color-terracotta)));
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
}

.topup-error {
  margin-top: 0.55rem;
  color: var(--admin-danger, rgb(var(--color-danger)));
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
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-surface-soft, rgb(var(--color-stone) / 0.78));
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
  border: 1px dashed var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
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
  color: var(--admin-muted, rgb(var(--color-muted)));
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
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  padding: 0.9rem 1rem;
}

.topup-meta-label {
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.topup-meta-value {
  margin-top: 0.3rem;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: 1.12rem;
  font-weight: 700;
}

.topup-meta-value--accent {
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
}

.topup-summary-card {
  border: 1px solid var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  border-radius: 8px;
  background:
    linear-gradient(180deg, rgb(var(--color-terracotta) / 0.08), transparent 70%),
    var(--admin-surface, rgb(var(--color-marble) / 0.96));
  padding: 1.1rem;
  box-shadow: var(--admin-shadow-sm, 0 8px 24px rgb(var(--shadow-ink) / 0.08));
}

.topup-summary-card--apex {
  position: relative;
  overflow: hidden;
  isolation: isolate;
  border-color: rgb(var(--gild-500) / 0.5);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.14), rgb(var(--color-terracotta) / 0.1) 42%, transparent 72%),
    linear-gradient(120deg, rgb(var(--lacquer-rest-top)) 0%, rgb(var(--lacquer-rest-mid)) 52%, rgb(var(--lacquer-base)) 100%);
  color: rgb(var(--gild-100));
  box-shadow:
    0 24px 70px rgb(var(--shadow-ink) / 0.24),
    inset 0 0 0 1px rgb(var(--gild-500) / 0.16);
}

.topup-summary-card--apex::before {
  content: "";
  position: absolute;
  z-index: -1;
  inset: 0;
  opacity: 0.68;
  background:
    linear-gradient(90deg, transparent 0%, rgb(var(--gild-500) / 0.18) 48%, transparent 60%),
    repeating-linear-gradient(90deg, rgb(var(--gild-500) / 0.07) 0 1px, transparent 1px 3.25rem);
  transform: translateX(-44%);
  animation: topup-apex-sweep 7.4s ease-in-out infinite;
}

.topup-summary-card--apex-activate {
  animation: topup-apex-panel-summon 0.98s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

.topup-summary-card--apex-activate::after {
  content: "";
  position: absolute;
  inset: -1px;
  pointer-events: none;
  border-radius: inherit;
  background:
    linear-gradient(110deg, transparent 0%, rgb(var(--gild-400) / 0.52) 36%, rgb(var(--gild-700) / 0.36) 50%, transparent 66%),
    linear-gradient(180deg, rgb(var(--gild-500) / 0.32), transparent 36%, rgb(var(--gild-500) / 0.14));
  opacity: 0;
  animation: topup-apex-panel-flare 0.98s ease-out both;
}

.topup-summary-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.topup-summary-kicker {
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.86rem;
  font-weight: 650;
}

.topup-summary-card--apex .topup-summary-kicker {
  color: rgb(var(--gild-200) / 0.74);
}

.topup-summary-title {
  margin-top: 0.45rem;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: 1.85rem;
  font-weight: 650;
  line-height: 1.2;
}

.topup-summary-card--apex .topup-summary-title {
  color: rgb(var(--gild-200));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2rem, 4vw, 2.45rem);
  text-shadow: 0 0 20px rgb(var(--gild-500) / 0.2);
}

.topup-price-chip {
  flex: 0 0 auto;
  border: 1px solid rgb(var(--color-terracotta) / 0.2);
  border-radius: 4px;
  background: rgb(var(--color-terracotta) / 0.1);
  color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  padding: 0.45rem 0.65rem;
  font-size: 0.9rem;
  font-weight: 700;
}

.topup-price-chip--muted {
  border-color: var(--admin-border, rgb(var(--color-ink) / 0.14));
  background: var(--admin-surface-soft, rgb(var(--color-stone) / 0.78));
  color: var(--admin-muted, rgb(var(--color-muted)));
}

.topup-summary-card--apex .topup-price-chip--muted {
  border-color: rgb(var(--gild-500) / 0.42);
  background: rgb(var(--gild-500) / 0.1);
  color: rgb(var(--gild-500));
}

.topup-summary-rows {
  display: grid;
  gap: 0.7rem;
  margin-top: 1rem;
}

.topup-summary-payment {
  margin-top: 1.15rem;
  border-top: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  padding-top: 1rem;
}

.topup-summary-methods {
  display: grid;
  gap: 0.55rem;
}

.topup-summary-method {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  min-height: 3.4rem;
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  padding: 0.62rem 0.75rem;
  color: var(--admin-ink, rgb(var(--color-ink)));
  text-align: left;
  transition:
    border-color var(--duration-base) var(--ease-standard),
    background-color var(--duration-base) var(--ease-standard),
    box-shadow var(--duration-base) var(--ease-out);
}

.topup-summary-method > span:last-child {
  display: grid;
  min-width: 0;
  gap: 0.12rem;
}

.topup-summary-method strong {
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: 0.9rem;
  font-weight: 750;
}

.topup-summary-method small {
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.72rem;
  font-weight: 650;
}

.topup-summary-method:hover:not(:disabled),
.topup-summary-method--active {
  border-color: var(--admin-terracotta, rgb(var(--color-terracotta)));
  background: rgb(var(--color-terracotta) / 0.08);
  box-shadow: inset 0 0 0 1px rgb(var(--color-terracotta) / 0.08);
}

.topup-summary-method:disabled {
  cursor: not-allowed;
  opacity: 0.52;
}

.topup-summary-payment .topup-warning {
  margin-top: 0.65rem;
}

.topup-summary-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.92rem;
}

.topup-summary-row strong {
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-weight: 700;
  text-align: right;
}

.topup-summary-row .topup-summary-limit {
  display: grid;
  gap: 0.22rem;
}

.topup-summary-limit small {
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.74rem;
  font-weight: 650;
  line-height: 1.35;
}

.topup-monthly-detail {
  display: grid;
  gap: 0.7rem;
}

.topup-monthly-prices {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.topup-quota-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.topup-quota-grid--single {
  grid-template-columns: 1fr;
}

.topup-monthly-prices div,
.topup-monthly-quota {
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  border-radius: 8px;
  background: rgb(var(--color-vellum) / 0.58);
  padding: 0.65rem;
}

.topup-monthly-prices span,
.topup-monthly-quota span {
  display: block;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.35;
}

.topup-monthly-prices strong,
.topup-monthly-quota strong {
  display: block;
  margin-top: 0.42rem;
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
  font-size: 1.05rem;
  font-weight: 800;
  line-height: 1.28;
}

.topup-monthly-quota {
  background:
    linear-gradient(180deg, rgb(var(--color-terracotta) / 0.08), transparent 100%),
    rgb(var(--color-vellum) / 0.64);
}

.topup-monthly-quota strong {
  font-size: 1.22rem;
}

.topup-monthly-quota small {
  display: block;
  margin-top: 0.35rem;
  color: var(--admin-muted, rgb(var(--color-muted)));
  font-size: 0.76rem;
  font-weight: 650;
  line-height: 1.45;
}

.topup-apex-lore {
  margin: 0;
  border-left: 2px solid rgb(var(--gild-500) / 0.56);
  padding-left: 0.85rem;
  color: rgb(var(--gild-300) / 0.88);
  font-size: 0.9rem;
  font-weight: 650;
  line-height: 1.85;
}

.topup-monthly-detail--apex .topup-monthly-prices div,
.topup-monthly-detail--apex .topup-monthly-quota {
  border-color: rgb(var(--gild-500) / 0.24);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.12), rgb(var(--color-primary-400) / 0.07)),
    rgb(var(--lacquer-base) / 0.46);
}

.topup-monthly-detail--apex .topup-monthly-prices span,
.topup-monthly-detail--apex .topup-monthly-quota span {
  color: rgb(var(--gild-200) / 0.72);
}

.topup-monthly-detail--apex .topup-monthly-prices strong,
.topup-monthly-detail--apex .topup-monthly-quota strong {
  color: rgb(var(--gild-100));
}

.topup-monthly-detail--apex .topup-monthly-quota small {
  color: rgb(var(--gild-200) / 0.7);
}

.dark .topup-monthly-prices div,
.dark .topup-monthly-quota {
  border-color: rgb(var(--color-vellum) / 0.12);
  background: rgb(var(--color-gray-900) / 0.86);
}

.dark .topup-monthly-quota {
  background:
    linear-gradient(180deg, rgb(var(--color-primary-400) / 0.12), transparent 100%),
    rgb(var(--color-gray-900) / 0.9);
}

.dark .topup-monthly-prices span,
.dark .topup-monthly-quota span {
  color: rgb(var(--color-stone) / 0.78);
}

.dark .topup-monthly-prices strong,
.dark .topup-monthly-quota strong {
  color: rgb(var(--gild-50));
}

.dark .topup-monthly-quota small {
  color: rgb(var(--color-stone) / 0.72);
}

.dark .topup-monthly-detail--apex .topup-monthly-prices div,
.dark .topup-monthly-detail--apex .topup-monthly-quota {
  border-color: rgb(var(--gild-500) / 0.24);
  background:
    linear-gradient(135deg, rgb(var(--gild-500) / 0.12), rgb(var(--color-primary-400) / 0.07)),
    rgb(var(--lacquer-base) / 0.5);
}

.dark .topup-monthly-detail--apex .topup-monthly-prices span,
.dark .topup-monthly-detail--apex .topup-monthly-quota span {
  color: rgb(var(--gild-200) / 0.72);
}

.dark .topup-monthly-detail--apex .topup-monthly-prices strong,
.dark .topup-monthly-detail--apex .topup-monthly-quota strong {
  color: rgb(var(--gild-100));
}

.dark .topup-monthly-detail--apex .topup-monthly-quota small {
  color: rgb(var(--gild-200) / 0.7);
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
  border: 1px solid var(--admin-terracotta, rgb(var(--color-terracotta)));
  background: var(--admin-terracotta, rgb(var(--color-terracotta)));
  color: var(--admin-marble, rgb(var(--color-marble))) !important;
  box-shadow: 0 12px 24px rgb(var(--color-terracotta) / 0.18);
}

.topup-primary-action:hover:not(:disabled) {
  border-color: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
  background: var(--admin-terracotta-dark, rgb(var(--color-terracotta-dark)));
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
  grid-template-columns: 1fr;
  gap: 0.75rem;
  margin-top: 1rem;
}

.topup-newcomer-actions {
  display: grid;
  gap: 0.75rem;
  margin-top: 1rem;
}

.topup-backup-action {
  gap: 0.6rem;
}

.topup-action-with-icon {
  gap: 0.6rem;
}

.topup-newcomer-actions .topup-secondary-action,
.topup-newcomer-actions .topup-warning {
  margin-top: 0;
}

.topup-monthly-actions .topup-primary-action,
.topup-monthly-actions .topup-secondary-action {
  margin-top: 0;
}

.topup-monthly-actions :deep(.trial-offer) {
  margin-top: 0;
  gap: 0.75rem;
  padding: 0.9rem;
}

.topup-monthly-actions :deep(.trial-offer h2) {
  margin-top: 0.45rem;
  font-size: 1.1rem;
}

.topup-monthly-actions :deep(.trial-offer__amounts strong) {
  font-size: 1.3rem;
}

.topup-monthly-actions :deep(.trial-offer__paymethod),
.topup-monthly-actions :deep(.trial-offer__action) {
  margin-top: 0.55rem;
}

.topup-monthly-actions :deep(.trial-offer__action) {
  padding-block: 0.65rem;
}

.topup-summary-card--apex .topup-primary-action {
  border-color: rgb(var(--gild-700));
  background:
    linear-gradient(135deg, rgb(var(--gild-500)) 0%, rgb(var(--gild-700)) 44%, rgb(var(--gild-900)) 100%);
  color: rgb(var(--color-dark-950)) !important;
  box-shadow: 0 16px 34px rgb(var(--gild-700) / 0.28);
}

.topup-summary-card--apex .topup-primary-action:disabled {
  border-color: rgb(var(--gild-500) / 0.2);
  background: rgb(var(--gild-500) / 0.12);
  color: rgb(var(--gild-200) / 0.52) !important;
  opacity: 1;
  box-shadow: none;
}

.topup-summary-card--apex .topup-secondary-action {
  border-color: rgb(var(--gild-500) / 0.28);
  background: rgb(var(--gild-200) / 0.06);
  color: rgb(var(--gild-300));
}

.topup-summary-card--apex .topup-secondary-action:hover {
  border-color: rgb(var(--gild-500) / 0.5);
  background: rgb(var(--gild-500) / 0.12);
  color: rgb(var(--gild-100));
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
  border: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
  background: var(--admin-control, rgb(var(--color-vellum) / 0.95));
  color: var(--admin-ink, rgb(var(--color-ink)));
}

.topup-secondary-action:hover {
  border-color: var(--admin-border-strong, rgb(var(--color-ink) / 0.32));
  background: var(--admin-parchment, rgb(var(--color-parchment)));
  color: var(--admin-ink-deep, rgb(var(--color-ink-deep)));
}

.topup-secondary-action--stacked {
  margin-top: 0.75rem;
}

.dark .topup-primary-action {
  color: var(--admin-marble, rgb(var(--color-dark-800))) !important;
}

@keyframes topup-apex-sweep {
  0%,
  58% {
    transform: translateX(-48%);
    opacity: 0.36;
  }
  82% {
    transform: translateX(44%);
    opacity: 0.86;
  }
  100% {
    transform: translateX(48%);
    opacity: 0.36;
  }
}

@keyframes topup-apex-card-summon {
  0% {
    transform: translateY(0) scale(1);
    box-shadow:
      0 0 0 rgb(var(--gild-500) / 0),
      inset 0 0 0 1px rgb(var(--gild-500) / 0.12);
  }
  34% {
    transform: translateY(-4px) scale(1.015);
    box-shadow:
      0 0 0 5px rgb(var(--gild-500) / 0.15),
      0 26px 78px rgb(var(--gild-700) / 0.34),
      inset 0 0 44px rgb(var(--gild-500) / 0.16);
  }
  100% {
    transform: translateY(-2px) scale(1);
  }
}

@keyframes topup-apex-panel-summon {
  0% {
    transform: translateY(0) scale(1);
    box-shadow:
      0 24px 70px rgb(var(--shadow-ink) / 0.24),
      inset 0 0 0 1px rgb(var(--gild-500) / 0.16);
  }
  32% {
    transform: translateY(-5px) scale(1.012);
    box-shadow:
      0 0 0 5px rgb(var(--gild-500) / 0.16),
      0 32px 86px rgb(var(--gild-700) / 0.32),
      inset 0 0 54px rgb(var(--gild-500) / 0.18);
  }
  100% {
    transform: translateY(0) scale(1);
  }
}

@keyframes topup-apex-panel-flare {
  0% {
    opacity: 0;
    transform: translateX(-46%);
  }
  28% {
    opacity: 0.94;
  }
  100% {
    opacity: 0;
    transform: translateX(46%);
  }
}

@keyframes topup-apex-edge-flare {
  0%,
  100% {
    filter: brightness(1);
  }
  38% {
    filter: brightness(1.75);
  }
}

@media (prefers-reduced-motion: reduce) {
  .topup-monthly-product--apex::after,
  .topup-monthly-product--apex-activate,
  .topup-summary-card--apex::before,
  .topup-summary-card--apex-activate {
    animation: none;
    transform: none;
  }

  .topup-summary-card--apex-activate::after {
    display: none;
  }
}

@media (max-width: 520px) {
  .topup-monthly-prices,
  .topup-quota-grid {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 640px) {
  .topup-page {
    padding-inline: 1.5rem;
  }

  .topup-main,
  .topup-summary {
    padding: 1.25rem;
  }

  .topup-channel-grid,
  .topup-products,
  .topup-plan-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .topup-presets {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

}

@media (min-width: 1024px) {
  .topup-page {
    padding: 1rem;
  }

  .topup-grid {
    grid-template-columns: minmax(0, 1.7fr) minmax(19rem, 0.72fr);
  }

  .topup-main {
    border-right: 1px solid var(--admin-border, rgb(var(--color-ink) / 0.14));
    border-bottom: 0;
  }

  .topup-summary-card {
    position: sticky;
    top: 5rem;
  }
}

@media (min-width: 1280px) {
  .topup-products {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }

  .topup-plan-grid,
  .topup-channel-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
</style>
