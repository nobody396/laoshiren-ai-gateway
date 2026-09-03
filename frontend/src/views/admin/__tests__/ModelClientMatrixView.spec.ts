import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getPublicModelPricing, getAdminModelClientMatrix } = vi.hoisted(() => ({
  getPublicModelPricing: vi.fn(),
  getAdminModelClientMatrix: vi.fn(),
}))
vi.mock('@/api/publicPricing', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/publicPricing')>()
  return { ...actual, getPublicModelPricing }
})
vi.mock('@/api/admin/modelClientMatrix', () => ({ getAdminModelClientMatrix }))

import { clientMatrix } from '@/generated/clientMatrix'
import { modelDocContracts } from '@/generated/modelDocContracts'

import router from '@/router'
import ModelClientMatrixView from '../ModelClientMatrixView.vue'

describe('ModelClientMatrixView', () => {
  beforeEach(() => {
    getPublicModelPricing.mockReset()
    getAdminModelClientMatrix.mockReset()
    getPublicModelPricing.mockResolvedValue({ updated_at: '2026-08-31', currency: 'CNY', unit: 'per_1m_tokens', groups: [] })
    getAdminModelClientMatrix.mockResolvedValue({
      schema_version: 1,
      counts: { models: 34, clients: 17, intersections: 578 },
      contracts: modelDocContracts,
      client_matrix: { clients: clientMatrix.map((client, index) => ({
        id: `fixture-${index}`,
        name: client.name,
        slug: client.slug,
        icon: client.icon,
        one_click_status: client.one_click_status,
        client_protocol: { protocols: ['responses', 'chat_completions', 'messages', 'generate_content'].map(protocol => ({ protocol, support: client.protocols.includes(protocol as never) ? 'supported' : 'unsupported' })) },
        client_reasoning: { control_kind: client.reasoning.mode, level_control: { values: client.reasoning.levels } },
        client_config_os: { release: { version_key: client.version, display: client.version }, os_support: [{ os: 'macos', support: 'documented', config_files: [] }], endpoint: { base_url_rule: '', credential_location: '' }, model_discovery: '', model_slots: [], owned_fields: [], mutation: { merge_strategy: '' } },
      })) },
    })
  })

  function mountView() {
    return mount(ModelClientMatrixView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          ModelMatrixDetail: { props: ['row'], template: '<div data-testid="detail">{{ row.model.model.id }}</div>' },
        },
      },
    })
  }

  it('registers the matrix as an authenticated administrator-only route', () => {
    const route = router.getRoutes().find(item => item.path === '/admin/model-client-matrix')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.permission).toBe('api:GET:/admin/model-client-matrix')
  })

  it('renders candidate intersections, filters by model and expands a row', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('模型 × 协议 × 客户端')
    expect(wrapper.findAll('.matrix-row').length).toBeGreaterThan(0)

    await wrapper.get('input[aria-label="筛选模型"]').setValue('grok-4.6')
    const rows = wrapper.findAll('.matrix-row')
    expect(rows.length).toBeGreaterThan(0)
    expect(rows.every(row => row.text().includes('grok-4.6'))).toBe(true)

    await rows[0]!.get('button').trigger('click')
    expect(wrapper.get('[data-testid="detail"]').text()).toBe('grok-4.6')
  })

  it('can reveal unsupported intersections and falls back to contract pricing when the public readback fails', async () => {
    getPublicModelPricing.mockRejectedValue(new Error('offline'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('公共价格失败 · 展示合同快照')
    const candidateToggle = wrapper.get('input[type="checkbox"]')
    await candidateToggle.setValue(false)
    await wrapper.get('select[aria-label="筛选状态"]').setValue('unsupported')
    expect(wrapper.findAll('[data-status="unsupported"]').length).toBeGreaterThan(0)
  })
})
