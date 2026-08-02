import { describe, expect, it } from 'vitest'
import {
  BALANCE_TOPUP_PRESETS,
  PROMOTIONAL_BALANCE_TOPUPS,
  getCreditedBalanceTopupAmount,
  getPromotionalBalanceTopup,
  isSupportedBalanceTopupAmount
} from '../balanceTopups'

describe('balanceTopups', () => {
  it('keeps only the approved customer-facing denominations', () => {
    expect(BALANCE_TOPUP_PRESETS).toEqual([20, 50, 100])
  })

  it('allows only ordinary and promotional shop denominations', () => {
    for (const amount of [1, 5, 10, 200, 300, 600, 2000]) {
      expect(isSupportedBalanceTopupAmount(amount), String(amount)).toBe(false)
    }

    for (const amount of BALANCE_TOPUP_PRESETS) {
      expect(isSupportedBalanceTopupAmount(amount), String(amount)).toBe(true)
    }
    for (const product of PROMOTIONAL_BALANCE_TOPUPS) {
      expect(isSupportedBalanceTopupAmount(product.paidAmountCny), String(product.paidAmountCny)).toBe(true)
    }
  })

  it('defines only the approved promotional top-up cards', () => {
    expect(PROMOTIONAL_BALANCE_TOPUPS).toEqual([
      { paidAmountCny: 500, bonusAmountCny: 75, creditedAmountCny: 575, bonusPercent: 15 },
      { paidAmountCny: 1000, bonusAmountCny: 200, creditedAmountCny: 1200, bonusPercent: 20 }
    ])
    expect(getPromotionalBalanceTopup(500)?.bonusAmountCny).toBe(75)
    expect(getPromotionalBalanceTopup(1000)?.bonusAmountCny).toBe(200)
    expect(getPromotionalBalanceTopup(600)).toBeUndefined()
    expect(getCreditedBalanceTopupAmount(500)).toBe(575)
    expect(getCreditedBalanceTopupAmount(1000)).toBe(1200)
    expect(getCreditedBalanceTopupAmount(499)).toBe(499)
  })
})
