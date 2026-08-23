import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { getOpenAIShadowAudit } from '@/api/admin/channelMonitoring'

describe('Channel Monitoring Shadow API projection', () => {
  beforeEach(() => get.mockReset())

  it('preserves the backend decision snapshot field used by the comparison UI', async () => {
    const snapshot = {
      reliability_evidence_adapter_enabled: true,
      reliability_evidence_adapter_applied: true,
      reliability_evidence_adapter_reason: 'reliability_evidence_ready',
      candidates: [{ account_id: 53, reliability_evidence_samples: 12 }],
    }
    get
      .mockResolvedValueOnce({ data: { total: 1, evaluated: 1, diverged: 0, evaluation_duration_p95_us: 10 } })
      .mockResolvedValueOnce({ data: { ready: true, storage_ready: true, completeness: 1, attempted: 1, written: 1, failed: 0, dropped: 0 } })
      .mockResolvedValueOnce({ data: { items: [{ decision_id: 'decision-1', model: 'gpt-5.6-sol', snapshot }] } })

    const result = await getOpenAIShadowAudit()

    expect(result.decisions[0]?.snapshot).toEqual(snapshot)
    expect(result.decisions[0]).not.toHaveProperty('audit')
  })
})
