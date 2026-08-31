import { describe, expect, it } from 'vitest'
import { buildMonthlyCreditCardPlans, monthlyCreditCardPlans } from '../monthlyCreditCards'

describe('monthlyCreditCardPlans V3', () => {
  it('publishes Plus, Pro and Max with monthly-only limits', () => {
    expect(monthlyCreditCardPlans.map((plan) => ({
      id: plan.id,
      shop: plan.priceCny,
      direct: plan.directPriceCny,
      daily: plan.displayDailyCredits,
      weekly: plan.displayWeeklyCredits,
      monthly: plan.displayMonthlyCredits
    }))).toEqual([
      {
        id: 'plus',
        shop: 259,
        direct: 255,
        daily: 0,
        weekly: 0,
        monthly: 3000
      },
      {
        id: 'pro',
        shop: 729,
        direct: 715,
        daily: 0,
        weekly: 0,
        monthly: 9000
      },
      {
        id: 'max',
        shop: 1549,
        direct: 1525,
        daily: 0,
        weekly: 0,
        monthly: 20000
      }
    ])
  })

  it('uses the live shared entitlement limit for customer display', () => {
    const plans = buildMonthlyCreditCardPlans([{
      id: 'plus',
      gpt_group: {
        id: 101,
        name: 'GPT Plus 月卡组',
        platform: 'openai',
        rate_multiplier: 999,
        weekly_limit_usd: 999,
        monthly_limit_usd: 999
      },
      claude_group: {
        id: 102,
        name: 'Claude Plus 月卡组',
        platform: 'anthropic',
        rate_multiplier: 999,
        weekly_limit_usd: 999,
        monthly_limit_usd: 999
      }
    }])
    const plus = plans[0]
    expect(plus.id).toBe('plus')
    expect(plus.monthlyCredits).toBe(999)
    expect(plus.weeklyCredits).toBe(0)
    expect(plus.displayMonthlyCreditsText).toBe('9,990')
    expect(Object.keys(plus)).not.toContain('gptMonthlyUsage')
    expect(Object.keys(plus)).not.toContain('claudeMonthlyUsage')
  })
})
