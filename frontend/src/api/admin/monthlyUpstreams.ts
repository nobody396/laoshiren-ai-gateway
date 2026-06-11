import { apiClient } from '../client'

export type MonthlyUpstreamProbeStatus = 'ok' | 'slow' | 'rate_limited' | 'failed' | 'not_schedulable'

export interface MonthlyUpstreamProbePoint {
  account_id: number
  account_name: string
  platform: string
  model: string
  probe_path: 'gateway' | 'direct_upstream'
  status: MonthlyUpstreamProbeStatus
  http_status: number | null
  latency_ms: number
  error_code: string
  error_message: string
  checked_at: string
}

export interface MonthlyUpstreamProbeDiagnostic {
  probe_path: 'gateway' | 'direct_upstream'
  status: MonthlyUpstreamProbeStatus
  http_status: number | null
  latency_ms: number
  error_code: string
  error_message: string
  checked_at: string | null
}

export interface MonthlyUpstreamProbeCostEstimate {
  currency: string
  rate_multiplier: number
  input_tokens: number
  output_tokens: number
  input_cost_per_token: number
  output_cost_per_token: number
  standard_cost_per_probe: number
  actual_cost_per_probe: number
  actual_cost_per_minute: number
  actual_cost_per_hour: number
  actual_cost_per_day: number
  probe_interval_seconds: number
  estimate_note: string
}

export interface MonthlyUpstreamProbeAccount {
  account_id: number
  account_name: string
  platform: string
  model: string
  latest_status: MonthlyUpstreamProbeStatus | ''
  latest_http_status: number | null
  latest_latency_ms: number
  latest_error_code: string
  latest_error: string
  latest_checked_at: string | null
  uptime: number
  success_count: number
  total_count: number
  cost_estimate?: MonthlyUpstreamProbeCostEstimate
  latest_direct_upstream?: MonthlyUpstreamProbeDiagnostic
  points: MonthlyUpstreamProbePoint[]
}

export interface MonthlyUpstreamProbeSnapshot {
  enabled: boolean
  public_status_enabled: boolean
  window_minutes: number
  generated_at: string
  accounts: MonthlyUpstreamProbeAccount[]
}

export interface MonthlyUpstreamProbeSettings {
  enabled: boolean
  public_status_enabled: boolean
}

export type MonthlyUpstreamProbeSettingsUpdate = Partial<MonthlyUpstreamProbeSettings>

export async function getSnapshot(windowMinutes = 60): Promise<MonthlyUpstreamProbeSnapshot> {
  const { data } = await apiClient.get<MonthlyUpstreamProbeSnapshot>('/admin/ops/monthly-upstreams', {
    params: { window_minutes: windowMinutes }
  })
  return data
}

export async function updateSettings(settings: MonthlyUpstreamProbeSettingsUpdate): Promise<MonthlyUpstreamProbeSettings> {
  const { data } = await apiClient.put<MonthlyUpstreamProbeSettings>('/admin/ops/monthly-upstreams/settings', settings)
  return data
}

const monthlyUpstreamsAPI = { getSnapshot, updateSettings }
export default monthlyUpstreamsAPI
