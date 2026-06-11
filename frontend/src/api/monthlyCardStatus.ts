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

export interface MonthlyCardStatusSnapshot {
  enabled: boolean
  visible_to_users: boolean
  window_minutes: number
  probe_interval_seconds: number
  generated_at: string
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
