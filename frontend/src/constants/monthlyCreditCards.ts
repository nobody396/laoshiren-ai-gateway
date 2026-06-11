import { SUBSCRIPTION_CREDIT_DISPLAY_SCALE, formatSubscriptionCredits } from '@/utils/subscriptionCredits'

export type MonthlyCreditCardPlan = {
  id: 'lite' | 'pro' | 'max' | 'ultra'
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
  gptDisplayRate: string
  claudeDisplayRate: string
  gptWeeklyUsage: string
  claudeWeeklyUsage: string
  gptMonthlyUsage: string
  claudeMonthlyUsage: string
  description: string
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

const weeklyCardDays = 7
const monthlyCardDays = 30
const defaultGptCreditsPerUsd = 0.4
const defaultClaudeCreditsPerUsd = 1.25

function formatUsd(value: number): string {
  const rounded = Math.round(value * 100) / 100
  if (Number.isInteger(rounded)) return `${rounded} 刀`
  if (Number.isInteger(rounded * 10)) return `${rounded.toFixed(1)} 刀`
  return `${rounded.toFixed(2)} 刀`
}

function createMonthlyCreditCardPlan(
  input: Omit<
    MonthlyCreditCardPlan,
    | 'price'
    | 'directPrice'
    | 'weeklyCredits'
    | 'monthlyCredits'
    | 'displayDailyCredits'
    | 'displayWeeklyCredits'
    | 'displayMonthlyCredits'
    | 'displayDailyCreditsText'
    | 'displayWeeklyCreditsText'
    | 'displayMonthlyCreditsText'
    | 'gptDisplayRate'
    | 'claudeDisplayRate'
    | 'gptWeeklyUsage'
    | 'claudeWeeklyUsage'
    | 'gptMonthlyUsage'
    | 'claudeMonthlyUsage'
  >,
  entitlement?: MonthlyCreditCardPlanEntitlement
): MonthlyCreditCardPlan {
  const weeklyCredits = resolveSharedLimit(entitlement, 'weekly_limit_usd') ?? input.dailyCredits * weeklyCardDays
  const monthlyCredits = resolveSharedLimit(entitlement, 'monthly_limit_usd') ?? input.dailyCredits * monthlyCardDays
  const displayDailyCredits = input.dailyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const displayWeeklyCredits = weeklyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const displayMonthlyCredits = monthlyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const gptCreditsPerUsd = normalizeCreditsPerUsd(entitlement?.gpt_group?.rate_multiplier, defaultGptCreditsPerUsd)
  const claudeCreditsPerUsd = normalizeCreditsPerUsd(entitlement?.claude_group?.rate_multiplier, defaultClaudeCreditsPerUsd)
  return {
    ...input,
    price: `¥${input.priceCny}`,
    directPrice: `¥${input.directPriceCny}`,
    weeklyCredits,
    monthlyCredits,
    displayDailyCredits,
    displayWeeklyCredits,
    displayMonthlyCredits,
    displayDailyCreditsText: formatSubscriptionCredits(input.dailyCredits),
    displayWeeklyCreditsText: formatSubscriptionCredits(weeklyCredits),
    displayMonthlyCreditsText: formatSubscriptionCredits(monthlyCredits),
    gptDisplayRate: `${formatSubscriptionCredits(gptCreditsPerUsd)} AI credits / 刀`,
    claudeDisplayRate: `${formatSubscriptionCredits(claudeCreditsPerUsd)} AI credits / 刀`,
    gptWeeklyUsage: `约 ${formatUsd(weeklyCredits / gptCreditsPerUsd)} / 周`,
    claudeWeeklyUsage: `约 ${formatUsd(weeklyCredits / claudeCreditsPerUsd)} / 周`,
    gptMonthlyUsage: `约 ${formatUsd(monthlyCredits / gptCreditsPerUsd)} / 月`,
    claudeMonthlyUsage: `约 ${formatUsd(monthlyCredits / claudeCreditsPerUsd)} / 月`
  }
}

function normalizeCreditsPerUsd(value: number | null | undefined, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : fallback
}

function resolveSharedLimit(
  entitlement: MonthlyCreditCardPlanEntitlement | undefined,
  field: 'weekly_limit_usd' | 'monthly_limit_usd'
): number | null {
  const limits = [entitlement?.gpt_group?.[field], entitlement?.claude_group?.[field]]
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0)
  if (limits.length === 0) return null
  return Math.min(...limits)
}

const monthlyCreditCardPlanInputs = [
  {
    id: 'lite',
    name: 'Lite 月卡',
    priceCny: 269,
    directPriceCny: 265,
    dailyCredits: 15,
    description: '适合首次尝鲜，一份额度池同时覆盖 GPT Pro 与 Claude Max。',
    accent: 'lite',
    cardShopUrl: 'https://pay.ldxp.cn/item/dinyum'
  },
  {
    id: 'pro',
    name: 'Pro 月卡',
    priceCny: 519,
    directPriceCny: 509,
    dailyCredits: 30,
    description: '适合稳定日常开发，两个高阶分组共用同一份总额度。',
    accent: 'pro',
    cardShopUrl: 'https://pay.ldxp.cn/item/b1e0f5'
  },
  {
    id: 'max',
    name: 'Max 月卡',
    priceCny: 699,
    directPriceCny: 685,
    dailyCredits: 40,
    description: '适合重度开发者，共享池在复杂任务和长会话里留出余量。',
    accent: 'max',
    cardShopUrl: 'https://pay.ldxp.cn/item/lhd7pa'
  },
  {
    id: 'ultra',
    name: 'Ultra 月卡',
    priceCny: 899,
    directPriceCny: 879,
    dailyCredits: 50,
    description: '适合长期高频使用，两条高阶渠道共用同一份月度额度。',
    accent: 'ultra',
    cardShopUrl: 'https://pay.ldxp.cn/item/kqbjn9'
  }
] satisfies Array<Parameters<typeof createMonthlyCreditCardPlan>[0]>

export function buildMonthlyCreditCardPlans(
  entitlements: MonthlyCreditCardPlanEntitlement[] | null | undefined
): MonthlyCreditCardPlan[] {
  const entitlementByID = new Map((entitlements ?? []).map((item) => [item.id, item]))
  return monthlyCreditCardPlanInputs.map((input) => createMonthlyCreditCardPlan(input, entitlementByID.get(input.id)))
}

export const monthlyCreditCardPlans: MonthlyCreditCardPlan[] = [
  ...buildMonthlyCreditCardPlans(null)
]
