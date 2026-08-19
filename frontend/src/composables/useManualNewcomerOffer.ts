import { computed, ref } from 'vue'

import {
  getNativeCheckoutManualOfferStatus,
  listNativeCheckoutOffers,
  requestNativeCheckoutManualOfferPurchase,
} from '@/api/nativeCheckout'
import { isNewcomerBalanceTopup } from '@/constants/balanceTopups'

export const MANUAL_NEWCOMER_OFFER_CODE = 'newcomer-balance-5-to-10'

export type ManualNewcomerOfferState = 'loading' | 'available' | 'claimed' | 'unavailable'

// manual：跳外链动小铺购买；native：易支付通道，站内扫码下单。
export type NewcomerOfferMode = 'manual' | 'native'

export function shouldShowManualNewcomerProduct(
  amountCny: number,
  state: ManualNewcomerOfferState,
  mode: NewcomerOfferMode = 'manual'
) {
  if (!isNewcomerBalanceTopup(amountCny)) return true
  // native 模式下新人优惠由站内 NativeCheckoutTrialOffer 卡片承接，
  // 不再展示会跳转外部收银台的卡密商品。
  if (mode === 'native') return false
  return state === 'available'
}

export function useManualNewcomerOffer() {
  const state = ref<ManualNewcomerOfferState>('loading')
  const mode = ref<NewcomerOfferMode>('manual')
  const canPurchase = computed(() => state.value === 'available')

  async function refresh() {
    state.value = 'loading'
    mode.value = 'manual'
    try {
      const status = await getNativeCheckoutManualOfferStatus(MANUAL_NEWCOMER_OFFER_CODE)
      state.value = status.claimed ? 'claimed' : 'available'
    } catch {
      // Fail closed: an account whose one-time status cannot be confirmed must
      // not be sent to the anonymous external checkout.
      state.value = 'unavailable'
    }
    // 仅当新人余额优惠可见且由易支付通道提供时切到站内扫码；
    // 任何失败都回退到现有的外链手动流程。
    try {
      const offers = await listNativeCheckoutOffers()
      const newcomerOffer = offers.find(
        (item) => item.product_kind === 'balance' && item.once_per_user
      )
      if (newcomerOffer && newcomerOffer.provider === 'easypay') {
        mode.value = 'native'
      }
    } catch {
      // keep manual mode
    }
  }

  async function requestPurchaseURL() {
    const status = await requestNativeCheckoutManualOfferPurchase(MANUAL_NEWCOMER_OFFER_CODE)
    if (status.claimed) {
      state.value = 'claimed'
      return ''
    }
    const purchaseURL = status.purchase_url?.trim() ?? ''
    if (!purchaseURL) {
      state.value = 'unavailable'
      return ''
    }
    state.value = 'available'
    return purchaseURL
  }

  return {
    state,
    mode,
    canPurchase,
    refresh,
    requestPurchaseURL,
  }
}
