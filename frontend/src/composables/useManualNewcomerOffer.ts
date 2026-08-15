import { computed, ref } from 'vue'

import {
  getNativeCheckoutManualOfferStatus,
  requestNativeCheckoutManualOfferPurchase,
} from '@/api/nativeCheckout'
import { isNewcomerBalanceTopup } from '@/constants/balanceTopups'

export const MANUAL_NEWCOMER_OFFER_CODE = 'newcomer-balance-5-to-10'

export type ManualNewcomerOfferState = 'loading' | 'available' | 'claimed' | 'unavailable'

export function shouldShowManualNewcomerProduct(amountCny: number, state: ManualNewcomerOfferState) {
  return !isNewcomerBalanceTopup(amountCny) || state === 'available'
}

export function useManualNewcomerOffer() {
  const state = ref<ManualNewcomerOfferState>('loading')
  const canPurchase = computed(() => state.value === 'available')

  async function refresh() {
    state.value = 'loading'
    try {
      const status = await getNativeCheckoutManualOfferStatus(MANUAL_NEWCOMER_OFFER_CODE)
      state.value = status.claimed ? 'claimed' : 'available'
    } catch {
      // Fail closed: an account whose one-time status cannot be confirmed must
      // not be sent to the anonymous external checkout.
      state.value = 'unavailable'
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
    canPurchase,
    refresh,
    requestPurchaseURL,
  }
}
