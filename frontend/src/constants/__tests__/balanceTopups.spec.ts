import { describe, expect, it } from 'vitest'
import { BALANCE_TOPUP_PRESETS, isSupportedBalanceTopupAmount } from '../balanceTopups'

describe('balanceTopups', () => {
  it('keeps only the approved customer-facing denominations', () => {
    expect(BALANCE_TOPUP_PRESETS).toEqual([20, 50, 100])
  })

  it('rejects legacy shop denominations', () => {
    for (const amount of [1, 5, 10, 200, 300, 500, 1000, 2000]) {
      expect(isSupportedBalanceTopupAmount(amount), String(amount)).toBe(false)
    }

    for (const amount of BALANCE_TOPUP_PRESETS) {
      expect(isSupportedBalanceTopupAmount(amount), String(amount)).toBe(true)
    }
  })
})
