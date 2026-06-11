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

const weeklyCardDays = 7
const monthlyCardDays = 30
const gptCreditsPerUsd = 0.8
const claudeCreditsPerUsd = 1.8
const displayCreditScale = 10

function formatUsd(value: number): string {
  const rounded = Math.round(value * 100) / 100
  if (Number.isInteger(rounded)) return `${rounded} 刀`
  if (Number.isInteger(rounded * 10)) return `${rounded.toFixed(1)} 刀`
  return `${rounded.toFixed(2)} 刀`
}

function formatCredits(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 0
  }).format(value)
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
  >
): MonthlyCreditCardPlan {
  const weeklyCredits = input.dailyCredits * weeklyCardDays
  const monthlyCredits = input.dailyCredits * monthlyCardDays
  const displayDailyCredits = input.dailyCredits * displayCreditScale
  const displayWeeklyCredits = weeklyCredits * displayCreditScale
  const displayMonthlyCredits = monthlyCredits * displayCreditScale
  return {
    ...input,
    price: `¥${input.priceCny}`,
    directPrice: `¥${input.directPriceCny}`,
    weeklyCredits,
    monthlyCredits,
    displayDailyCredits,
    displayWeeklyCredits,
    displayMonthlyCredits,
    displayDailyCreditsText: formatCredits(displayDailyCredits),
    displayWeeklyCreditsText: formatCredits(displayWeeklyCredits),
    displayMonthlyCreditsText: formatCredits(displayMonthlyCredits),
    gptDisplayRate: `${formatCredits(gptCreditsPerUsd * displayCreditScale)} AI credits / 刀`,
    claudeDisplayRate: `${formatCredits(claudeCreditsPerUsd * displayCreditScale)} AI credits / 刀`,
    gptWeeklyUsage: `约 ${formatUsd(weeklyCredits / gptCreditsPerUsd)} / 周`,
    claudeWeeklyUsage: `约 ${formatUsd(weeklyCredits / claudeCreditsPerUsd)} / 周`,
    gptMonthlyUsage: `约 ${formatUsd(monthlyCredits / gptCreditsPerUsd)} / 月`,
    claudeMonthlyUsage: `约 ${formatUsd(monthlyCredits / claudeCreditsPerUsd)} / 月`
  }
}

export const monthlyCreditCardPlans: MonthlyCreditCardPlan[] = [
  createMonthlyCreditCardPlan({
    id: 'lite',
    name: 'Lite 月卡',
    priceCny: 269,
    directPriceCny: 265,
    dailyCredits: 15,
    description: '适合首次尝鲜，一份额度池同时覆盖 GPT Pro 与 Claude Max。',
    accent: 'lite',
    cardShopUrl: 'https://pay.ldxp.cn/item/dinyum'
  }),
  createMonthlyCreditCardPlan({
    id: 'pro',
    name: 'Pro 月卡',
    priceCny: 519,
    directPriceCny: 509,
    dailyCredits: 30,
    description: '适合稳定日常开发，两个高阶分组共用同一份总额度。',
    accent: 'pro',
    cardShopUrl: 'https://pay.ldxp.cn/item/b1e0f5'
  }),
  createMonthlyCreditCardPlan({
    id: 'max',
    name: 'Max 月卡',
    priceCny: 699,
    directPriceCny: 685,
    dailyCredits: 40,
    description: '适合重度开发者，共享池在复杂任务和长会话里留出余量。',
    accent: 'max',
    cardShopUrl: 'https://pay.ldxp.cn/item/lhd7pa'
  }),
  createMonthlyCreditCardPlan({
    id: 'ultra',
    name: 'Ultra 月卡',
    priceCny: 899,
    directPriceCny: 879,
    dailyCredits: 50,
    description: '适合长期高频使用，两条高阶渠道共用同一份月度额度。',
    accent: 'ultra',
    cardShopUrl: 'https://pay.ldxp.cn/item/kqbjn9'
  })
]
