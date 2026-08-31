import { apiClient } from './client'

export type MonthlyCardStatus = 'ok' | 'slow' | 'rate_limited' | 'failed' | 'not_schedulable' | 'missing' | ''

export interface MonthlyCardStatusPoint {
  status: MonthlyCardStatus
  checked_at: string
}

export interface MonthlyCardStatusAccount {
  display_name: string
  channel: string
  status: MonthlyCardStatus
  latest_checked_at: string | null
  uptime: number
  points: MonthlyCardStatusPoint[]
}

export interface MonthlyCardPlanGroup {
  id: number
  name: string
  platform: string
  rate_multiplier: number
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
}

export interface MonthlyCardPlanEntitlement {
  id: 'plus' | 'pro' | 'max'
  name: string
  gpt_group?: MonthlyCardPlanGroup | null
  claude_group?: MonthlyCardPlanGroup | null
  grok_group?: MonthlyCardPlanGroup | null
}

export interface MonthlyCardStatusSnapshot {
  enabled: boolean
  visible_to_users: boolean
  visible_channels?: string[]
  window_minutes: number
  probe_interval_seconds: number
  generated_at: string
  plans?: MonthlyCardPlanEntitlement[]
  accounts: MonthlyCardStatusAccount[]
}

export async function getMonthlyCardStatus(windowMinutes = 60): Promise<MonthlyCardStatusSnapshot> {
  const response = await apiClient.get<MonthlyCardStatusSnapshot>('/monthly-card/status', {
    params: { window_minutes: windowMinutes }
  })
  return response.data
}

export default {
  getMonthlyCardStatus
}
