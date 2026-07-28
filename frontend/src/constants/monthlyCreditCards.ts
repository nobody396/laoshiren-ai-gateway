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
  gptDisplayRate: string
  claudeDisplayRate: string
  gptWeeklyUsage: string
  claudeWeeklyUsage: string
  gptMonthlyUsage: string
  claudeMonthlyUsage: string
  gptMonthlyTokensText: string
  claudeMonthlyTokensText: string
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

const defaultGptCreditsPerUsd = 0.5
const defaultClaudeCreditsPerUsd = 2.4
const millionTokens = 1_000_000
const gptWeightedUsdPerMillionTokens = 1.035
const claudeWeightedUsdPerMillionTokens = 1.0175

function formatUsd(value: number): string {
  const rounded = Math.round(value * 100) / 100
  if (Number.isInteger(rounded)) return `${rounded} 刀`
  if (Number.isInteger(rounded * 10)) return `${rounded.toFixed(1)} 刀`
  return `${rounded.toFixed(2)} 刀`
}

function formatEstimatedTokens(usdValue: number, usdPerMillionTokens: number): string {
  if (!Number.isFinite(usdValue) || usdValue <= 0 || usdPerMillionTokens <= 0) return '约 0 token'
  const tokens = (usdValue / usdPerMillionTokens) * millionTokens
  const yi = tokens / 100_000_000
  if (yi >= 1) return `约 ${yi.toFixed(yi >= 10 ? 1 : 2).replace(/\.0$/, '')} 亿 token`
  const wan = tokens / 10_000
  return `约 ${wan.toFixed(wan >= 100 ? 0 : 1).replace(/\.0$/, '')} 万 token`
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
  const gptCreditsPerUsd = defaultGptCreditsPerUsd
  const claudeCreditsPerUsd = defaultClaudeCreditsPerUsd
  const gptMonthlyUsd = monthlyCredits / gptCreditsPerUsd
  const claudeMonthlyUsd = monthlyCredits / claudeCreditsPerUsd
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
    showWeeklyLimit: false,
    gptDisplayRate: `${formatSubscriptionCredits(gptCreditsPerUsd)} AI credits / 刀`,
    claudeDisplayRate: `${formatSubscriptionCredits(claudeCreditsPerUsd)} AI credits / 刀`,
    gptWeeklyUsage: '不设周限额',
    claudeWeeklyUsage: '不设周限额',
    gptMonthlyUsage: `约 ${formatUsd(gptMonthlyUsd)} / 31 天`,
    claudeMonthlyUsage: `约 ${formatUsd(claudeMonthlyUsd)} / 31 天`,
    gptMonthlyTokensText: formatEstimatedTokens(gptMonthlyUsd, gptWeightedUsdPerMillionTokens),
    claudeMonthlyTokensText: formatEstimatedTokens(claudeMonthlyUsd, claudeWeightedUsdPerMillionTokens)
  }
}

const monthlyCreditCardPlanInputs: PlanInput[] = [
  {
    id: 'plus',
    name: 'Plus',
    priceCny: 259,
    directPriceCny: 249,
    monthlyCredits: 220,
    description: '轻量但完整的 31 天开发额度，适合日常编码、问答与短任务。',
    accent: 'plus',
    cardShopUrl: ''
  },
  {
    id: 'pro',
    name: 'Pro',
    priceCny: 729,
    directPriceCny: 699,
    monthlyCredits: 650,
    description: '面向稳定高频开发与多轮代理任务，整月额度可自由安排。',
    accent: 'pro',
    cardShopUrl: ''
  },
  {
    id: 'max',
    name: 'Max',
    priceCny: 1549,
    directPriceCny: 1499,
    monthlyCredits: 1400,
    description: '为大型重构、长上下文与连续高强度开发保留更大额度。',
    accent: 'max',
    cardShopUrl: ''
  }
]

export function buildMonthlyCreditCardPlans(
  entitlements: MonthlyCreditCardPlanEntitlement[] | null | undefined
): MonthlyCreditCardPlan[] {
  const entitlementByID = new Map((entitlements ?? []).map((item) => [item.id, item]))
  return monthlyCreditCardPlanInputs.map((input) => createMonthlyCreditCardPlan(input, entitlementByID.get(input.id)))
}

export const monthlyCreditCardPlans = buildMonthlyCreditCardPlans(null)
