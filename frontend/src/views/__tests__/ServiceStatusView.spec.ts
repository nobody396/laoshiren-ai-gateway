import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ServiceStatusView from '../ServiceStatusView.vue'

const mocks = vi.hoisted(() => ({
  getServiceStatus: vi.fn(),
  getPublicIncidents: vi.fn(),
  loadMarkdown: vi.fn()
}))

vi.mock('@/api/serviceStatus', () => ({
  getServiceStatus: (...args: unknown[]) => mocks.getServiceStatus(...args),
  getPublicIncidents: (...args: unknown[]) => mocks.getPublicIncidents(...args)
}))

vi.mock('@/docs/config', () => ({
  loadMarkdown: (...args: unknown[]) => mocks.loadMarkdown(...args)
}))

vi.mock('@/composables/useMarkdownRenderer', () => ({
  useMarkdownRenderer: (source: { value: string }) => ({ renderedHtml: source })
}))

const statuses = [
  ['operational', '正常'],
  ['degraded_performance', '性能下降'],
  ['partial_outage', '部分中断'],
  ['major_outage', '大范围中断'],
  ['maintenance', '维护中'],
  ['monitoring', '观察中']
] as const

const snapshot = (enabled = true) => ({
  enabled,
  generated_at: new Date().toISOString(),
  families: enabled
    ? [
        {
          code: 'openai-codex',
          display_name: 'OpenAI / Codex',
          products: statuses.map(([status, label], index) => ({
            code: `product-${index}`,
            display_name: `服务 ${label}`,
            status,
            reason: 'test',
            evidence_at: new Date().toISOString(),
            computed_at: new Date().toISOString(),
            components: [{
              code: `component-${index}`,
              display_name: `服务 ${label} · HTTP`,
              access_mode: 'http',
              status,
              reason: 'test',
              computed_at: new Date().toISOString()
            }]
          }))
        },
        {
          code: 'other',
          display_name: '其他服务',
          products: [{
            code: 'other-one',
            display_name: '其他模型 API',
            status: 'operational',
            reason: 'fresh_success',
            computed_at: new Date().toISOString(),
            components: []
          }]
        }
      ]
    : []
})

function mountView() {
  return mount(ServiceStatusView, {
    global: {
      stubs: {
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
        DocsContent: { props: ['markdown', 'loading', 'notFound'], template: '<div data-test="fallback-doc">{{ markdown }}</div>' }
      }
    }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.loadMarkdown.mockResolvedValue('# 服务支持说明')
  mocks.getServiceStatus.mockResolvedValue(snapshot())
  mocks.getPublicIncidents.mockResolvedValue({ enabled: false, generated_at: new Date().toISOString(), incidents: [] })
})

afterEach(() => vi.useRealTimers())

describe('ServiceStatusView', () => {
  it('renders every public state and keeps common services before Other', async () => {
    const wrapper = mountView()
    await flushPromises()

    for (const [, label] of statuses) expect(wrapper.text()).toContain(label)
    expect(wrapper.text().indexOf('OpenAI / Codex')).toBeLessThan(wrapper.text().indexOf('其他服务'))
    expect(wrapper.find('[data-test="service-status-live"]').exists()).toBe(true)
  })

  it('renders only the sanitized public incident timeline', async () => {
    mocks.getPublicIncidents.mockResolvedValue({ enabled: true, generated_at: new Date().toISOString(), incidents: [{ id: 'public-id', phase: 'monitoring', started_at: new Date().toISOString(), affected_products: ['Codex API'], timeline: [{ phase: 'monitoring', message: '相关服务已经恢复，用户无需进行额外操作。', published_at: new Date().toISOString() }] }] })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('事件时间线')
    expect(wrapper.text()).toContain('相关服务已经恢复，用户无需进行额外操作。')
    expect(wrapper.text()).not.toContain('supplier')
  })

  it('shows only affected model details for a partial model impact', async () => {
    const data = snapshot()
    data.families[0].products = [{
      code: 'codex',
      display_name: 'Codex API',
      status: 'partial_outage',
      reason: 'component_rollup',
      computed_at: new Date().toISOString(),
      components: [
        { code: 'gpt-5', display_name: 'GPT-5 系列', access_mode: 'http', status: 'operational', reason: 'fresh_success', computed_at: new Date().toISOString() },
        { code: 'o3', display_name: 'o3 系列', access_mode: 'http', status: 'partial_outage', reason: 'customer_impact', computed_at: new Date().toISOString() }
      ]
    }]
    mocks.getServiceStatus.mockResolvedValue(data)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('受影响范围')
    expect(wrapper.text()).toContain('o3 系列')
    expect(wrapper.text()).not.toContain('GPT-5 系列')
  })

  it('marks stale evaluation data instead of presenting it as current', async () => {
    const data = snapshot()
    data.families = [data.families[0]]
    data.families[0].products = [data.families[0].products[0]]
    data.families[0].products[0].computed_at = new Date(Date.now() - 10 * 60_000).toISOString()
    mocks.getServiceStatus.mockResolvedValue(data)

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('数据更新延迟')
    expect(wrapper.text()).toContain('观察中')
    expect(wrapper.text()).not.toContain('当前公开服务运行正常')
  })

  it('projects stale severe status as Monitoring consistently across overview, rail and row', async () => {
    const data = snapshot()
    const staleMajor = data.families[0].products.find((product) => product.status === 'major_outage')!
    staleMajor.computed_at = new Date(Date.now() - 10 * 60_000).toISOString()
    const freshOperational = data.families[0].products.find((product) => product.status === 'operational')!
    data.families = [{ ...data.families[0], products: [staleMajor, freshOperational] }]
    mocks.getServiceStatus.mockResolvedValue(data)

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('.status-overview').classes()).toContain('service-status-tone-monitoring')
    expect(wrapper.get('article[data-status="monitoring"]').text()).toContain('观察中')
    expect(wrapper.find('.availability-rail .service-status-tone-major_outage').exists()).toBe(false)
    expect(wrapper.find('.service-status-tone-major_outage').exists()).toBe(false)
  })

  it('keeps the existing support document when public status is disabled', async () => {
    mocks.getServiceStatus.mockResolvedValue(snapshot(false))
    const wrapper = mountView()
    await flushPromises()

    expect(mocks.loadMarkdown).toHaveBeenCalledWith('sla-support')
    expect(wrapper.get('[data-test="fallback-doc"]').text()).toContain('服务支持说明')
    expect(wrapper.find('[data-test="service-status-live"]').exists()).toBe(false)
  })

  it('falls back safely on API errors and never renders internal selectors', async () => {
    mocks.getServiceStatus.mockRejectedValue(new Error('supplier account route_fingerprint group_id base_url'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('实时状态暂不可用')
    for (const secret of ['supplier', 'account', 'route_fingerprint', 'group_id', 'base_url']) {
      expect(wrapper.text()).not.toContain(secret)
    }
    expect(wrapper.find('[data-test="fallback-doc"]').exists()).toBe(true)
  })

  it('renders an explicit empty catalog state', async () => {
    mocks.getServiceStatus.mockResolvedValue({ enabled: true, generated_at: new Date().toISOString(), families: [] })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('暂未发布可展示的服务')
    expect(wrapper.text()).not.toContain('当前公开服务运行正常')
    expect(wrapper.find('.status-overview').exists()).toBe(false)
  })

  it('uses static emergency copy when the fallback document also fails', async () => {
    mocks.getServiceStatus.mockRejectedValue(new Error('status unavailable'))
    mocks.loadMarkdown.mockRejectedValue(new Error('chunk unavailable'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('完整排查文档暂不可用')
    expect(wrapper.find('[data-test="fallback-doc"]').exists()).toBe(true)
  })

  it('does not overlap polling requests and clears the timer on unmount', async () => {
    vi.useFakeTimers()
    let resolveFirst!: (value: ReturnType<typeof snapshot>) => void
    mocks.getServiceStatus
      .mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
      .mockResolvedValue(snapshot())

    const wrapper = mountView()
    await Promise.resolve()
    vi.advanceTimersByTime(30_000)
    expect(mocks.getServiceStatus).toHaveBeenCalledTimes(1)

    resolveFirst(snapshot())
    await flushPromises()
    vi.advanceTimersByTime(30_000)
    await flushPromises()
    expect(mocks.getServiceStatus).toHaveBeenCalledTimes(2)

    wrapper.unmount()
    vi.advanceTimersByTime(60_000)
    expect(mocks.getServiceStatus).toHaveBeenCalledTimes(2)
  })
})
