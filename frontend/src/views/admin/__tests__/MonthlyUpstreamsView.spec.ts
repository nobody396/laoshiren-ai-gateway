import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { MonthlyUpstreamProbeSnapshot } from '@/api/admin/monthlyUpstreams'

const mockGetSnapshot = vi.fn()
const mockUpdateSettings = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    monthlyUpstreams: {
      getSnapshot: (...args: unknown[]) => mockGetSnapshot(...args),
      updateSettings: (...args: unknown[]) => mockUpdateSettings(...args)
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    template: '<main><slot /></main>'
  }
}))

vi.mock('@/components/common/Toggle.vue', () => ({
  default: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<button type="button" @click="$emit(\'update:modelValue\', !modelValue)"><slot /></button>'
  }
}))

import MonthlyUpstreamsView from '@/views/admin/MonthlyUpstreamsView.vue'

function snapshot(latestStatus: 'ok' | 'failed', gatewayCheckedAt: string): MonthlyUpstreamProbeSnapshot {
  return {
    enabled: true,
    public_status_enabled: false,
    public_status_channels: ['codex', 'claude', 'grok'],
    window_minutes: 60,
    generated_at: '2026-07-30T10:29:00Z',
    accounts: [{
      account_id: 24,
      account_name: 'monthly-codex-gateway',
      platform: 'openai',
      model: 'gpt-5.4-mini',
      latest_status: latestStatus,
      latest_http_status: latestStatus === 'ok' ? 200 : 503,
      latest_latency_ms: latestStatus === 'ok' ? 2635 : 8216,
      latest_error_code: '',
      latest_error: '',
      latest_checked_at: gatewayCheckedAt,
      uptime: 0.44,
      success_count: 9,
      total_count: 27,
      latest_direct_upstream: {
        probe_path: 'direct_upstream',
        status: 'failed',
        http_status: 503,
        latency_ms: 8216,
        error_code: '',
        error_message: '503 Service Unavailable: No server is available to handle this request.',
        checked_at: '2026-07-30T10:18:04Z'
      },
      points: []
    }]
  }
}

afterEach(() => {
  vi.clearAllMocks()
})

describe('MonthlyUpstreamsView direct diagnostics', () => {
  it('hides an old direct 503 after a newer gateway recovery', async () => {
    mockGetSnapshot.mockResolvedValue(snapshot('ok', '2026-07-30T10:28:48Z'))

    const wrapper = mount(MonthlyUpstreamsView)
    await flushPromises()

    expect(wrapper.text()).toContain('正常')
    expect(wrapper.text()).toContain('HTTP 200')
    expect(wrapper.text()).not.toContain('503 Service Unavailable')
    expect(wrapper.text()).not.toContain('直连上游诊断')

    wrapper.unmount()
  })

  it('keeps the direct 503 visible while the gateway is still failing', async () => {
    mockGetSnapshot.mockResolvedValue(snapshot('failed', '2026-07-30T10:18:00Z'))

    const wrapper = mount(MonthlyUpstreamsView)
    await flushPromises()

    expect(wrapper.text()).toContain('直连上游诊断')
    expect(wrapper.text()).toContain('HTTP 503')
    expect(wrapper.text()).toContain('503 Service Unavailable')

    wrapper.unmount()
  })

  it('can hide only the Grok status card from users', async () => {
    const current = snapshot('ok', '2026-07-30T10:28:48Z')
    current.public_status_enabled = true
    mockGetSnapshot.mockResolvedValue(current)
    mockUpdateSettings.mockResolvedValue({
      enabled: true,
      public_status_enabled: true,
      public_status_channels: ['codex', 'claude']
    })

    const wrapper = mount(MonthlyUpstreamsView)
    await flushPromises()

    const grokLabel = wrapper.findAll('label').find((label) => label.text().includes('Grok 月卡'))
    expect(grokLabel).toBeDefined()
    await grokLabel!.find('button').trigger('click')
    await flushPromises()

    expect(mockUpdateSettings).toHaveBeenCalledWith({
      public_status_channels: ['codex', 'claude']
    })

    wrapper.unmount()
  })
})
