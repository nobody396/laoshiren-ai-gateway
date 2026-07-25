import { SUBSCRIPTION_CREDIT_DISPLAY_SCALE, formatSubscriptionCredits } from '@/utils/subscriptionCredits'

export type MonthlyCreditCardPlan = {
  id: 'lite' | 'pro' | 'max' | 'ultra' | 'apex'
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
  disableWeeklyLimit?: boolean
  retired?: boolean
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

const monthlyCardDays = 31
const defaultGptCreditsPerUsd = 0.4
const defaultClaudeCreditsPerUsd = 1.25
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
  if (yi >= 1) {
    return `约 ${yi.toFixed(yi >= 10 ? 1 : 2).replace(/\.0$/, '')} 亿 token`
  }
  const wan = tokens / 10_000
  return `约 ${wan.toFixed(wan >= 100 ? 0 : 1).replace(/\.0$/, '')} 万 token`
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
    | 'showWeeklyLimit'
    | 'gptDisplayRate'
    | 'claudeDisplayRate'
    | 'gptWeeklyUsage'
    | 'claudeWeeklyUsage'
    | 'gptMonthlyUsage'
    | 'claudeMonthlyUsage'
    | 'gptMonthlyTokensText'
    | 'claudeMonthlyTokensText'
  >,
  entitlement?: MonthlyCreditCardPlanEntitlement
): MonthlyCreditCardPlan {
  const weeklyCredits = input.disableWeeklyLimit
    ? 0
    : resolveSharedLimit(entitlement, 'weekly_limit_usd') ?? 0
  const monthlyCredits = resolveSharedLimit(entitlement, 'monthly_limit_usd') ?? input.dailyCredits * monthlyCardDays
  const displayDailyCredits = input.dailyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const displayWeeklyCredits = weeklyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const displayMonthlyCredits = monthlyCredits * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
  const gptCreditsPerUsd = normalizeCreditsPerUsd(entitlement?.gpt_group?.rate_multiplier, defaultGptCreditsPerUsd)
  const claudeCreditsPerUsd = normalizeCreditsPerUsd(entitlement?.claude_group?.rate_multiplier, defaultClaudeCreditsPerUsd)
  const gptMonthlyUsd = monthlyCredits / gptCreditsPerUsd
  const claudeMonthlyUsd = monthlyCredits / claudeCreditsPerUsd
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
    showWeeklyLimit: weeklyCredits > 0,
    gptDisplayRate: `${formatSubscriptionCredits(gptCreditsPerUsd)} AI credits / 刀`,
    claudeDisplayRate: `${formatSubscriptionCredits(claudeCreditsPerUsd)} AI credits / 刀`,
    gptWeeklyUsage: `约 ${formatUsd(weeklyCredits / gptCreditsPerUsd)} / 周`,
    claudeWeeklyUsage: `约 ${formatUsd(weeklyCredits / claudeCreditsPerUsd)} / 周`,
    gptMonthlyUsage: `约 ${formatUsd(gptMonthlyUsd)} / 月`,
    claudeMonthlyUsage: `约 ${formatUsd(claudeMonthlyUsd)} / 月`,
    gptMonthlyTokensText: formatEstimatedTokens(gptMonthlyUsd, gptWeightedUsdPerMillionTokens),
    claudeMonthlyTokensText: formatEstimatedTokens(claudeMonthlyUsd, claudeWeightedUsdPerMillionTokens)
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
    priceCny: 329,
    directPriceCny: 319,
    dailyCredits: 15,
    description: '适合首次尝鲜，一份额度池同时覆盖 GPT Pro 与 Claude Max。',
    accent: 'lite',
    cardShopUrl: 'https://pay.ldxp.cn/item/dinyum',
    disableWeeklyLimit: true
  },
  {
    id: 'pro',
    name: 'Pro 月卡',
    priceCny: 639,
    directPriceCny: 619,
    dailyCredits: 30,
    description: '适合稳定日常开发，两个高阶分组共用同一份总额度。',
    accent: 'pro',
    cardShopUrl: 'https://pay.ldxp.cn/item/b1e0f5',
    disableWeeklyLimit: true
  },
  {
    id: 'max',
    name: 'Max 月卡',
    priceCny: 699,
    directPriceCny: 685,
    dailyCredits: 40,
    description: '适合重度开发者，共享池在复杂任务和长会话里留出余量。',
    accent: 'max',
    cardShopUrl: 'https://pay.ldxp.cn/item/lhd7pa',
    disableWeeklyLimit: true,
    retired: true
  },
  {
    id: 'ultra',
    name: 'Ultra 月卡',
    priceCny: 899,
    directPriceCny: 879,
    dailyCredits: 50,
    description: '适合长期高频使用，两条高阶渠道共用同一份月度额度。',
    accent: 'ultra',
    cardShopUrl: 'https://pay.ldxp.cn/item/kqbjn9',
    disableWeeklyLimit: true,
    retired: true
  },
  {
    id: 'apex',
    name: 'Apex 月卡',
    priceCny: 1299,
    directPriceCny: 1275,
    dailyCredits: 96.6,
    description: '传说级长任务通行证，面向连续编排、海量审查与整月高频开发。',
    legendaryCopy: '黑金权限已铸成：适合把大型重构、长上下文代理和批量审查一次推到底。',
    rarityLabel: 'Legendary Apex',
    accent: 'apex',
    cardShopUrl: 'https://pay.ldxp.cn/item/pb4se8',
    disableWeeklyLimit: true,
    retired: true
  }
] satisfies Array<Parameters<typeof createMonthlyCreditCardPlan>[0]>

export function buildMonthlyCreditCardPlans(
  entitlements: MonthlyCreditCardPlanEntitlement[] | null | undefined
): MonthlyCreditCardPlan[] {
  const entitlementByID = new Map((entitlements ?? []).map((item) => [item.id, item]))
  return monthlyCreditCardPlanInputs
    .filter((input) => !input.retired)
    .map((input) => createMonthlyCreditCardPlan(input, entitlementByID.get(input.id)))
}

export const monthlyCreditCardPlans: MonthlyCreditCardPlan[] = [
  ...buildMonthlyCreditCardPlans(null)
]
