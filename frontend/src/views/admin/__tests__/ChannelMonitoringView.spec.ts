import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelMonitoringView from '../ChannelMonitoringView.vue'

const api = vi.hoisted(() => ({ getChannelMonitoring: vi.fn(), getOpenAIShadowAudit: vi.fn() }))
vi.mock('@/api/admin/channelMonitoring', () => api)

const now = new Date().toISOString()
const snapshot = {
  generated_at: now,
  window_start: now,
  window_end: now,
  window_minutes: 15,
  completeness: { enabled: true, running: true, dropped: 0, failed: 0, queue_depth: 0, in_flight: 0 },
  status: {
    enabled: true,
    public_enabled: false,
    evaluation_ready: true,
    families: [{
      code: 'openai-codex',
      display_name: 'OpenAI / Codex',
      products: [{
        code: 'openai-api',
        display_name: 'OpenAI / Codex API',
        computed_status: 'degraded_performance',
        effective_status: 'degraded_performance',
        computed_reason: 'repeated_customer_impact',
        effective_reason: 'repeated_customer_impact',
        customer_availability: { total: 20, succeeded: 18, failed: 2, rate: 0.9 },
        probe_availability: { total: 10, succeeded: 9, failed: 1, rate: 0.9 },
        components: [{
          code: 'openai-http', display_name: 'OpenAI / Codex API · HTTP', access_mode: 'http', computed_status: 'degraded_performance', reason: 'repeated_customer_impact',
          bindings: [{ binding_key: 'platform:openai', platform: 'openai' }]
        }]
      }]
    }]
  },
  evidence: [{
    fact_type: 'active_probe', platform: 'openai', model: 'gpt-5.6', request_class: 'text', protocol: 'http',
    account_id: 53, account_name: 'Pomo JP', route_fingerprint: '0123456789abcdef0123456789abcdef', sample_count: 10,
    success_count: 9, failure_count: 1, recovered_count: 0, customer_impact_count: 0, availability: 0.9,
    average_latency_ms: 320, p95_latency_ms: 600, samples_per_minute: 0.666, last_observed_at: now, last_success_at: now,
    product_codes: ['openai-api'], component_codes: ['openai-http']
  }, {
    fact_type: 'active_probe', platform: 'openai', model: 'gpt-5.6', request_class: 'text', protocol: 'http_direct',
    account_id: 53, account_name: 'Pomo JP', route_fingerprint: 'fedcba9876543210fedcba9876543210', sample_count: 1,
    success_count: 1, failure_count: 0, recovered_count: 0, customer_impact_count: 0, availability: 1,
    average_latency_ms: 100, p95_latency_ms: 100, samples_per_minute: 0.06, last_observed_at: now,
    product_codes: [], component_codes: []
  }],
  evidence_bucket_total: 2,
  evidence_truncated: false,
  totals: { sample_count: 11, failure_count: 1, customer_request_count: 20, customer_success_count: 18 }
}

function mountView() {
  return mount(ChannelMonitoringView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' }
  } } })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.getChannelMonitoring.mockResolvedValue(snapshot)
  api.getOpenAIShadowAudit.mockResolvedValue({
    stats: { total: 100, evaluated: 90, diverged: 4, evaluation_duration_p95_us: 120 },
    health: { ready: true, storage_ready: true, completeness: 1, attempted: 100, written: 100, failed: 0, dropped: 0 },
    decisions: [{ decision_id: 'decision-123456789', model: 'gpt-5.6', snapshot: {
      reliability_evidence_adapter_enabled: true,
      reliability_evidence_adapter_applied: true,
      reliability_evidence_adapter_reason: 'reliability_evidence_ready',
      candidates: [{ account_id: 53, legacy_success_lower_bound: 0.8, reliability_evidence_success_lower_bound: 0.95, reliability_evidence_applied: true }]
    } }]
  })
})

describe('ChannelMonitoringView', () => {
  it('renders a read-only product-to-route drilldown and reliability completeness', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(api.getChannelMonitoring).toHaveBeenCalledWith(15, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('OpenAI / Codex API')
    expect(wrapper.text()).toContain('Pomo JP')
    expect(wrapper.text()).toContain('01234567…abcdef')
    expect(wrapper.text()).toContain('状态证据可用于评估')
    expect(wrapper.text()).toContain('未映射到公开产品的内部证据')
    expect(wrapper.text()).toContain('OpenAI 智能路由 Shadow 审计')
    expect(wrapper.text()).toContain('24h 决策')
    expect(wrapper.text()).toContain('80.0% → 95.0%')
    expect(wrapper.text()).toContain('0.67/min')
    expect(wrapper.text()).not.toContain('禁用渠道')
    expect(wrapper.text()).not.toContain('切换 Base URL')
  })

  it('filters evidence without hiding the product context', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('input[placeholder*="搜索服务"]').setValue('no-such-route')
    expect(wrapper.text()).toContain('OpenAI / Codex API')
    expect(wrapper.text()).toContain('当前筛选范围内没有证据')
  })

  it('keeps a usable error state when the monitoring query fails', async () => {
    api.getChannelMonitoring.mockRejectedValue(new Error('query timeout'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('渠道监控数据暂不可用')
  })

  it('shows an explicit initial loading state', async () => {
    api.getChannelMonitoring.mockReturnValue(new Promise(() => {}))
    const wrapper = mountView()
    await Promise.resolve()
    expect(wrapper.text()).toContain('正在加载渠道监控证据')
  })

  it('keeps rapid refresh ownership with the newest request', async () => {
    let resolveSecond!: (value: typeof snapshot) => void
    api.getChannelMonitoring
      .mockImplementationOnce((_window: number, signal: AbortSignal) => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSecond = resolve }))
    const wrapper = mountView()
    await Promise.resolve()
    await wrapper.get('select[aria-label="证据时间窗口"]').setValue('60')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('正在加载渠道监控证据')
    resolveSecond(snapshot)
    await flushPromises()
    expect(wrapper.text()).toContain('OpenAI / Codex API')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('aborts monitoring and Shadow requests on unmount', async () => {
    let captured!: AbortSignal
    api.getChannelMonitoring.mockImplementation((_window: number, signal: AbortSignal) => {
      captured = signal
      return new Promise(() => {})
    })
    const wrapper = mountView()
    await Promise.resolve()
    expect(captured.aborted).toBe(false)
    wrapper.unmount()
    expect(captured.aborted).toBe(true)
  })

  it('renders primary monitoring without waiting for a slow optional Shadow audit', async () => {
    api.getOpenAIShadowAudit.mockReturnValue(new Promise(() => {}))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('OpenAI / Codex API')
    expect(wrapper.text()).toContain('Shadow 审计加载中')
    expect(wrapper.get('button').text()).toContain('刷新证据')
    wrapper.unmount()
  })

  it('surfaces a fast monitoring error without waiting for slow Shadow audit', async () => {
    api.getChannelMonitoring.mockRejectedValue(new Error('query timeout'))
    api.getOpenAIShadowAudit.mockReturnValue(new Promise(() => {}))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('query timeout')
    wrapper.unmount()
  })
})
