import { SUBSCRIPTION_CREDIT_DISPLAY_SCALE, formatSubscriptionCredits } from '@/utils/subscriptionCredits'

export type MonthlyCreditCardPlan = {
  id: 'plus' | 'pro' | 'max'
  name: string
  priceCny: number
  directPriceCny: number
  price: string
  directPrice: string
  dailyCredits: number
  weeklyCredits: number
  monthlyCredits: number
  displayDailyCredits: number
  displayWeeklyCredits: number
  displayMonthlyCredits: number
  displayDailyCreditsText: string
  displayWeeklyCreditsText: string
  displayMonthlyCreditsText: string
  showWeeklyLimit: boolean
  description: string
  legendaryCopy?: string
  rarityLabel?: string
  accent: string
  cardShopUrl: string
}

export type MonthlyCreditCardPlanGroupEntitlement = {
  id: number
  name: string
  platform: string
  rate_multiplier: number
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
}

export type MonthlyCreditCardPlanEntitlement = {
  id: MonthlyCreditCardPlan['id']
  name?: string
  gpt_group?: MonthlyCreditCardPlanGroupEntitlement | null
  claude_group?: MonthlyCreditCardPlanGroupEntitlement | null
}

type PlanInput = Pick<MonthlyCreditCardPlan,
  'id' | 'name' | 'priceCny' | 'directPriceCny' | 'monthlyCredits' | 'description' | 'accent' | 'cardShopUrl'
>

function createMonthlyCreditCardPlan(input: PlanInput, entitlement?: MonthlyCreditCardPlanEntitlement): MonthlyCreditCardPlan {
  // Catalog limits are immutable SKU values.  The live entitlement is used for
  // availability/status only; stale group values must never silently change a
  // published quota or multiplier.
  const monthlyCredits = input.monthlyCredits
  void entitlement
  return {
    ...input,
    price: `¥${input.priceCny}`,
    directPrice: `¥${input.directPriceCny}`,
    dailyCredits: 0,
    weeklyCredits: 0,
    displayDailyCredits: 0,
    displayWeeklyCredits: 0,
    displayMonthlyCredits: monthlyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE,
    displayDailyCreditsText: formatSubscriptionCredits(0),
    displayWeeklyCreditsText: formatSubscriptionCredits(0),
    displayMonthlyCreditsText: formatSubscriptionCredits(monthlyCredits),
    showWeeklyLimit: false
  }
}

const monthlyCreditCardPlanInputs: PlanInput[] = [
  {
    id: 'plus',
    name: 'Plus',
    priceCny: 259,
    directPriceCny: 255,
    monthlyCredits: 300,
    description: '轻量但完整的 31 天开发额度，适合日常编码、问答与短任务。',
    accent: 'plus',
    cardShopUrl: 'https://pay.ldxp.cn/item/c0dudc'
  },
  {
    id: 'pro',
    name: 'Pro',
    priceCny: 729,
    directPriceCny: 715,
    monthlyCredits: 900,
    description: '面向稳定高频开发与多轮代理任务，整月额度可自由安排。',
    accent: 'pro',
    cardShopUrl: 'https://pay.ldxp.cn/item/x4dup4'
  },
  {
    id: 'max',
    name: 'Max',
    priceCny: 1549,
    directPriceCny: 1525,
    monthlyCredits: 2000,
    description: '为大型重构、长上下文与连续高强度开发保留更大额度。',
    accent: 'max',
    cardShopUrl: 'https://pay.ldxp.cn/item/db9f6w'
  }
]

export function buildMonthlyCreditCardPlans(
  entitlements: MonthlyCreditCardPlanEntitlement[] | null | undefined
): MonthlyCreditCardPlan[] {
  const entitlementByID = new Map((entitlements ?? []).map((item) => [item.id, item]))
  return monthlyCreditCardPlanInputs.map((input) => createMonthlyCreditCardPlan(input, entitlementByID.get(input.id)))
}

export const monthlyCreditCardPlans = buildMonthlyCreditCardPlans(null)
