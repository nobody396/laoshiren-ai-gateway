import { describe, expect, it } from 'vitest'
import type { MonthlyUpstreamProbeAccount } from '@/api/admin/monthlyUpstreams'
import { isDirectUpstreamDiagnosticStale } from '@/utils/monthlyUpstreamProbe'

function account(
  overrides: Partial<MonthlyUpstreamProbeAccount> = {}
): MonthlyUpstreamProbeAccount {
  return {
    account_id: 24,
    account_name: 'monthly-codex-gateway',
    platform: 'openai',
    model: 'gpt-5.4-mini',
    latest_status: 'ok',
    latest_http_status: 200,
    latest_latency_ms: 2635,
    latest_error_code: '',
    latest_error: '',
    latest_checked_at: '2026-07-30T10:28:48Z',
    uptime: 0.44,
    success_count: 9,
    total_count: 27,
    latest_direct_upstream: {
      probe_path: 'direct_upstream',
      status: 'failed',
      http_status: 503,
      latency_ms: 8216,
      error_code: '',
      error_message: 'No server is available to handle this request.',
      checked_at: '2026-07-30T10:18:04Z'
    },
    points: [],
    ...overrides
  }
}

describe('isDirectUpstreamDiagnosticStale', () => {
  it('treats an older direct failure as stale after a newer healthy gateway probe', () => {
    expect(isDirectUpstreamDiagnosticStale(account())).toBe(true)
  })

  it('keeps the direct failure visible while the latest gateway probe is degraded', () => {
    expect(isDirectUpstreamDiagnosticStale(account({ latest_status: 'slow' }))).toBe(false)
  })

  it('keeps a direct failure visible when it is newer than the gateway result', () => {
    expect(isDirectUpstreamDiagnosticStale(account({
      latest_checked_at: '2026-07-30T10:17:59Z'
    }))).toBe(false)
  })

  it('does not classify a successful direct diagnostic as a stale failure', () => {
    expect(isDirectUpstreamDiagnosticStale(account({
      latest_direct_upstream: {
        ...account().latest_direct_upstream!,
        status: 'ok',
        http_status: 200,
        error_message: ''
      }
    }))).toBe(false)
  })

  it('fails safe when timestamps are missing or invalid', () => {
    expect(isDirectUpstreamDiagnosticStale(account({ latest_checked_at: null }))).toBe(false)
    expect(isDirectUpstreamDiagnosticStale(account({
      latest_checked_at: 'invalid',
      latest_direct_upstream: {
        ...account().latest_direct_upstream!,
        checked_at: 'also-invalid'
      }
    }))).toBe(false)
  })
})
