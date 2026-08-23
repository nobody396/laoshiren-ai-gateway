import { apiClient } from '../client'
import type { ServiceStatus } from '@/api/serviceStatus'

export interface ChannelMonitoringCompleteness {
  enabled: boolean
  running: boolean
  dropped: number
  failed: number
  queue_depth: number
  in_flight: number
  oldest_pending_at?: string | null
}

export interface ChannelMonitoringBinding {
  binding_key: string
  group_id?: number
  group_name?: string
  platform?: string
  model_pattern?: string
  route_fingerprint?: string
}

export interface ChannelMonitoringComponent {
  code: string
  display_name: string
  access_mode: 'http'
  computed_status: ServiceStatus
  reason: string
  bindings: ChannelMonitoringBinding[]
}

export interface ChannelMonitoringProduct {
  code: string
  display_name: string
  computed_status: ServiceStatus
  effective_status: ServiceStatus
  computed_reason: string
  effective_reason: string
  customer_availability: { total: number; succeeded: number; failed: number; rate?: number }
  probe_availability: { total: number; succeeded: number; failed: number; rate?: number }
  components: ChannelMonitoringComponent[]
}

export interface ChannelMonitoringFamily {
  code: string
  display_name: string
  products: ChannelMonitoringProduct[]
}

export interface ChannelMonitoringEvidence {
  fact_type: 'customer_request' | 'upstream_attempt' | 'active_probe'
  platform: string
  model: string
  request_class: string
  protocol: string
  group_id?: number
  group_name?: string
  account_id?: number
  account_name?: string
  route_fingerprint?: string
  sample_count: number
  success_count: number
  failure_count: number
  recovered_count: number
  customer_impact_count: number
  availability?: number
  average_latency_ms: number
  p95_latency_ms: number
  samples_per_minute: number
  last_observed_at: string
  last_failure_at?: string
  last_success_at?: string
  last_recovery_at?: string
  product_codes: string[]
  component_codes: string[]
}

export interface ChannelMonitoringSnapshot {
  generated_at: string
  window_start: string
  window_end: string
  window_minutes: number
  completeness: ChannelMonitoringCompleteness
  status: {
    enabled: boolean
    public_enabled: boolean
    evaluation_ready: boolean
    families: ChannelMonitoringFamily[]
  }
  evidence: ChannelMonitoringEvidence[]
  evidence_bucket_total: number
  evidence_truncated: boolean
  totals: {
    sample_count: number
    failure_count: number
    customer_request_count: number
    customer_success_count: number
  }
}

export interface OpenAIShadowAuditSummary {
  stats: {
    total: number
    evaluated: number
    diverged: number
    last_decision_at?: string
    evaluation_duration_p95_us: number
  }
  health: {
    ready: boolean
    storage_ready: boolean
    completeness: number
    attempted: number
    written: number
    failed: number
    dropped: number
    last_success_at?: string
  }
}

export async function getChannelMonitoring(windowMinutes = 15, signal?: AbortSignal): Promise<ChannelMonitoringSnapshot> {
  const response = await apiClient.get<ChannelMonitoringSnapshot>('/admin/ops/channel-monitoring', {
    params: { window_minutes: windowMinutes }, signal
  })
  return response.data
}

export async function getOpenAIShadowAudit(signal?: AbortSignal): Promise<OpenAIShadowAuditSummary> {
  const [stats, health] = await Promise.all([
    apiClient.get<OpenAIShadowAuditSummary['stats']>('/admin/ops/openai-route-shadow/stats', { params: { time_range: '24h', policy_mode: 'shadow' }, signal }),
    apiClient.get<OpenAIShadowAuditSummary['health']>('/admin/ops/openai-route-shadow/health', { signal })
  ])
  return { stats: stats.data, health: health.data }
}
