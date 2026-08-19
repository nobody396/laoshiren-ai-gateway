import { ref } from 'vue'

import {
  listNativeCheckoutOffers,
  type NativeCheckoutOffer,
} from '@/api/nativeCheckout'

// 站内原生结账 offer 列表是小而共享的目录数据：新人卡组件、订阅页门控都读它。
// 模块级 ref + in-flight 去重，避免同一页面多个消费者并发重复拉取；
// 每次 loadOffers 仍会回源，页面重进时数据始终新鲜。
const offers = ref<NativeCheckoutOffer[]>([])
let inFlight: Promise<NativeCheckoutOffer[]> | null = null

export function useNativeCheckoutOffers() {
  async function loadOffers(): Promise<NativeCheckoutOffer[]> {
    if (inFlight) return inFlight
    inFlight = listNativeCheckoutOffers()
    try {
      offers.value = await inFlight
      return offers.value
    } finally {
      inFlight = null
    }
  }

  // 约定：原生 offer 的 code 与其承接的商品 id 相同（月卡套餐 plus/pro/max、
  // 新人商品 newcomer-balance-5-to-10）。只有可见（enabled 或测试账号放行）且
  // 由 easypay 通道提供的 offer 才会出现在列表里并命中这里。
  function findNativeOfferByCode(code: string, productKind?: 'balance' | 'subscription') {
    return offers.value.find(
      (item) =>
        item.code === code &&
        item.provider === 'easypay' &&
        (!productKind || item.product_kind === productKind)
    )
  }

  return { offers, loadOffers, findNativeOfferByCode }
}
