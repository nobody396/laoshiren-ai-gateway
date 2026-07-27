import { describe, expect, it } from 'vitest'
import { buildMonthlyCreditCardPlans, monthlyCreditCardPlans } from '../monthlyCreditCards'

describe('monthlyCreditCardPlans', () => {
  it('keeps weekly limits hidden for every monthly card', () => {
    for (const plan of monthlyCreditCardPlans) {
      expect(plan.showWeeklyLimit, plan.id).toBe(false)
      expect(plan.weeklyCredits, plan.id).toBe(0)
      expect(plan.displayWeeklyCredits, plan.id).toBe(0)
    }
  })

  it('ignores weekly entitlement snapshots while preserving monthly limits', () => {
    const plans = buildMonthlyCreditCardPlans([
      {
        id: 'lite',
        gpt_group: {
          id: 7,
          name: 'Lite GPT',
          platform: 'openai',
          rate_multiplier: 0.4,
          weekly_limit_usd: 105,
          monthly_limit_usd: 450
        },
        claude_group: {
          id: 11,
          name: 'Lite Claude',
          platform: 'claude',
          rate_multiplier: 1.25,
          weekly_limit_usd: 105,
          monthly_limit_usd: 450
        }
      }
    ])

    const lite = plans.find((plan) => plan.id === 'lite')
    expect(lite?.showWeeklyLimit).toBe(false)
    expect(lite?.weeklyCredits).toBe(0)
    expect(lite?.monthlyCredits).toBe(450)
  })

  it('publishes the three margin-gated plans with continuous entry pricing', () => {
    expect(monthlyCreditCardPlans.map((plan) => ({
      id: plan.id,
      shop: plan.priceCny,
      direct: plan.directPriceCny,
      daily: plan.displayDailyCredits,
      monthly: plan.displayMonthlyCredits
    }))).toEqual([
      { id: 'starter', shop: 259, direct: 249, daily: 80, monthly: 2400 },
      { id: 'lite', shop: 469, direct: 459, daily: 150, monthly: 4500 },
      { id: 'pro', shop: 869, direct: 839, daily: 280, monthly: 8500 }
    ])
  })
})
