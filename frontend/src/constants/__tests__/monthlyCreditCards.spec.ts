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
    const [lite] = buildMonthlyCreditCardPlans([
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

    expect(lite.showWeeklyLimit).toBe(false)
    expect(lite.weeklyCredits).toBe(0)
    expect(lite.monthlyCredits).toBe(450)
  })
})
