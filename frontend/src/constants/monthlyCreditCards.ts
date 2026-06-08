export type MonthlyCreditCardPlan = {
  id: 'lite' | 'pro' | 'max' | 'ultra'
  name: string
  price: string
  dailyCredits: number
  gptUsage: string
  claudeUsage: string
  description: string
  accent: string
}

export const monthlyCreditCardPlans: MonthlyCreditCardPlan[] = [
  {
    id: 'lite',
    name: 'Lite 月卡',
    price: '¥249',
    dailyCredits: 15,
    gptUsage: '约 25 刀/天',
    claudeUsage: '约 10 刀/天',
    description: '适合首次尝鲜，先体验 GPT Pro 与 Claude Max 共用额度。',
    accent: 'lite'
  },
  {
    id: 'pro',
    name: 'Pro 月卡',
    price: '¥499',
    dailyCredits: 30,
    gptUsage: '约 50 刀/天',
    claudeUsage: '约 20 刀/天',
    description: '适合稳定日常开发，覆盖多数个人高频编码需求。',
    accent: 'pro'
  },
  {
    id: 'max',
    name: 'Max 月卡',
    price: '¥699',
    dailyCredits: 40,
    gptUsage: '约 66.67 刀/天',
    claudeUsage: '约 26.67 刀/天',
    description: '适合重度开发者，在复杂任务和长会话里留出更大余量。',
    accent: 'max'
  },
  {
    id: 'ultra',
    name: 'Ultra 月卡',
    price: '¥899',
    dailyCredits: 50,
    gptUsage: '约 83.33 刀/天',
    claudeUsage: '约 33.33 刀/天',
    description: '适合长期高频使用，把两条高阶渠道作为主力工作流。',
    accent: 'ultra'
  }
]
