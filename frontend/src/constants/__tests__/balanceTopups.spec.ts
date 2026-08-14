import { describe, expect, it } from 'vitest'
import {
  BALANCE_TOPUP_PRESETS,
  PROMOTIONAL_BALANCE_TOPUPS,
  getCreditedBalanceTopupAmount,
  getPromotionalBalanceTopup,
  isNewcomerBalanceTopup,
  isSupportedBalanceTopupAmount
} from '../balanceTopups'

describe('balanceTopups', () => {
  it('keeps only the approved customer-facing denominations', () => {
    expect(BALANCE_TOPUP_PRESETS).toEqual([20, 50, 100])
  })

  it('allows only ordinary and promotional shop denominations', () => {
    for (const amount of [1, 10, 200, 300, 600, 2000]) {
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
      {
        paidAmountCny: 5,
        bonusAmountCny: 5,
        creditedAmountCny: 10,
        bonusPercent: 100,
        newcomerOnly: true
      },
      { paidAmountCny: 500, bonusAmountCny: 50, creditedAmountCny: 550, bonusPercent: 10 },
      { paidAmountCny: 1000, bonusAmountCny: 100, creditedAmountCny: 1100, bonusPercent: 10 }
    ])
    expect(getPromotionalBalanceTopup(5)?.creditedAmountCny).toBe(10)
    expect(getPromotionalBalanceTopup(500)?.bonusAmountCny).toBe(50)
    expect(getPromotionalBalanceTopup(1000)?.bonusAmountCny).toBe(100)
    expect(getPromotionalBalanceTopup(600)).toBeUndefined()
    expect(getCreditedBalanceTopupAmount(500)).toBe(550)
    expect(getCreditedBalanceTopupAmount(1000)).toBe(1100)
    expect(getCreditedBalanceTopupAmount(5)).toBe(10)
    expect(getCreditedBalanceTopupAmount(499)).toBe(499)
  })

  it('marks only the once-per-account newcomer card', () => {
    expect(isNewcomerBalanceTopup(5)).toBe(true)
    expect(isNewcomerBalanceTopup(500)).toBe(false)
  })
})
