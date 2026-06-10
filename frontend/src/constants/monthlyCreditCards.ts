export type MonthlyCreditCardPlan = {
  id: 'lite' | 'pro' | 'max' | 'ultra'
  name: string
  priceCny: number
  price: string
  dailyCredits: number
  monthlyCredits: number
  displayDailyCredits: number
  displayMonthlyCredits: number
  displayDailyCreditsText: string
  displayMonthlyCreditsText: string
  gptDisplayRate: string
  claudeDisplayRate: string
  gptUsage: string
  claudeUsage: string
  gptMonthlyUsage: string
  claudeMonthlyUsage: string
  description: string
  accent: string
}

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
    | 'monthlyCredits'
    | 'displayDailyCredits'
    | 'displayMonthlyCredits'
    | 'displayDailyCreditsText'
    | 'displayMonthlyCreditsText'
    | 'gptDisplayRate'
    | 'claudeDisplayRate'
    | 'gptUsage'
    | 'claudeUsage'
    | 'gptMonthlyUsage'
    | 'claudeMonthlyUsage'
  >
): MonthlyCreditCardPlan {
  const monthlyCredits = input.dailyCredits * monthlyCardDays
  const displayDailyCredits = input.dailyCredits * displayCreditScale
  const displayMonthlyCredits = monthlyCredits * displayCreditScale
  return {
    ...input,
    price: `¥${input.priceCny}`,
    monthlyCredits,
    displayDailyCredits,
    displayMonthlyCredits,
    displayDailyCreditsText: formatCredits(displayDailyCredits),
    displayMonthlyCreditsText: formatCredits(displayMonthlyCredits),
    gptDisplayRate: `${formatCredits(gptCreditsPerUsd * displayCreditScale)} AI credits / 刀`,
    claudeDisplayRate: `${formatCredits(claudeCreditsPerUsd * displayCreditScale)} AI credits / 刀`,
    gptUsage: `约 ${formatUsd(input.dailyCredits / gptCreditsPerUsd)} / 天`,
    claudeUsage: `约 ${formatUsd(input.dailyCredits / claudeCreditsPerUsd)} / 天`,
    gptMonthlyUsage: `约 ${formatUsd(monthlyCredits / gptCreditsPerUsd)} / 月`,
    claudeMonthlyUsage: `约 ${formatUsd(monthlyCredits / claudeCreditsPerUsd)} / 月`
  }
}

export const monthlyCreditCardPlans: MonthlyCreditCardPlan[] = [
  createMonthlyCreditCardPlan({
    id: 'lite',
    name: 'Lite 月卡',
    priceCny: 265,
    dailyCredits: 15,
    description: '适合首次尝鲜，一份额度池同时覆盖 GPT Pro 与 Claude Max。',
    accent: 'lite'
  }),
  createMonthlyCreditCardPlan({
    id: 'pro',
    name: 'Pro 月卡',
    priceCny: 509,
    dailyCredits: 30,
    description: '适合稳定日常开发，两个高阶分组共用同一份总额度。',
    accent: 'pro'
  }),
  createMonthlyCreditCardPlan({
    id: 'max',
    name: 'Max 月卡',
    priceCny: 685,
    dailyCredits: 40,
    description: '适合重度开发者，共享池在复杂任务和长会话里留出余量。',
    accent: 'max'
  }),
  createMonthlyCreditCardPlan({
    id: 'ultra',
    name: 'Ultra 月卡',
    priceCny: 879,
    dailyCredits: 50,
    description: '适合长期高频使用，两条高阶渠道共用同一份月度额度。',
    accent: 'ultra'
  })
]
