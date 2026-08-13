import type { ModelStat, UsageStatsResponse } from '@/types'

export interface UsageReceiptData {
  receiptNumber: string
  generatedAt: Date
  startDate: string
  endDate: string
  displayName: string
  inviteCode: string
  inviteUrl: string
  qrDataUrl: string
  stats: UsageStatsResponse
  models: ModelStat[]
  exchangeRate: number
}

export interface UsageReceiptPreferences {
  showDisplayName: boolean
  showSavings: boolean
  showModelBreakdown: boolean
}

export interface ReceiptModelLine {
  model: string
  requests: number
  totalTokens: number
  actualCost: number
}
