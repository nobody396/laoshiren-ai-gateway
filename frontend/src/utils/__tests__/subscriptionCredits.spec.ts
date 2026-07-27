import { describe, expect, it } from 'vitest'

import type { Group } from '@/types'
import {
  SUBSCRIPTION_CREDIT_DISPLAY_SCALE,
  formatSubscriptionCredits,
  formatSubscriptionUsageDisplay,
  formatSubscriptionUsagePercent,
  formatSubscriptionUsageRatio,
  formatSubscriptionUsageValue,
  subscriptionUsagePercent,
  toSubscriptionDisplayCredits
} from '@/utils/subscriptionCredits'

const creditGroup = {
  subscription_type: 'credit'
} as Group

const subscriptionGroup = {
  subscription_type: 'subscription'
} as Group

describe('subscriptionCredits utils', () => {
  it('uses the shared display scale for credit subscriptions', () => {
    expect(SUBSCRIPTION_CREDIT_DISPLAY_SCALE).toBe(10)
    expect(toSubscriptionDisplayCredits(105)).toBe(1050)
    expect(formatSubscriptionCredits(105)).toBe('1,050')
  })

  it('formats credit subscription usage without using rate multipliers', () => {
    const group = {
      subscription_type: 'credit',
      rate_multiplier: 0.12
    } as Group

    expect(formatSubscriptionUsageDisplay(1.81, 105, group)).toBe(
      '18.1 / 1,050 credits · 已用 1.7%'
    )
    expect(formatSubscriptionUsageRatio(1.81, 105, group)).toBe('18.1 / 1,050 credits')
    expect(formatSubscriptionUsageValue(1.81, group)).toBe('18.1 credits')
  })

  it('keeps percentages based on the original usage and limit fields', () => {
    expect(subscriptionUsagePercent(1.81, 105)).toBeCloseTo(1.7238, 4)
    expect(formatSubscriptionUsagePercent(1.81, 105)).toBe('1.7%')
  })

  it('keeps legacy subscription groups in USD display', () => {
    expect(formatSubscriptionUsageDisplay(1.81, 105, subscriptionGroup)).toBe(
      '⚡1.81 / ⚡105.00 · 已用 1.7%'
    )
  })

  it('handles missing and zero limits safely', () => {
    expect(formatSubscriptionUsageDisplay(1.81, null, creditGroup)).toBe(
      '18.1 / ∞ credits · 已用 0.0%'
    )
    expect(subscriptionUsagePercent(1.81, 0)).toBe(0)
    expect(subscriptionUsagePercent(150, 100)).toBe(100)
  })
})
