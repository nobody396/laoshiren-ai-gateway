import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getIncidentAdminSnapshot: vi.fn(), updateIncidentSettings: vi.fn(), confirmIncidentCandidate: vi.fn(),
  dismissIncidentCandidate: vi.fn(), transitionIncident: vi.fn(), addIncidentUpdate: vi.fn(), publishIncidentUpdate: vi.fn(),
  acknowledgeIncidentEvidenceGap: vi.fn()
}))
vi.mock('@/api/admin/incidents', () => api)
import IncidentsView from '../IncidentsView.vue'

const snapshot = {
  settings: { enabled: true, public_enabled: false }, generated_at: new Date().toISOString(),
  candidates: [{ id: 7, state: 'open', first_observed_at: new Date().toISOString(), last_observed_at: new Date().toISOString(), products: [{ product_code: 'codex', product_name: 'Codex API', latest_status: 'partial_outage', first_observed_at: new Date().toISOString(), last_observed_at: new Date().toISOString() }] }],
  incidents: [{ id: 9, public_id: 'public-9', phase: 'monitoring', title: 'Codex incident', internal_summary: 'internal only', observation_started_at: new Date().toISOString(), version: 2, evidence_gap: false, products: [{ id: 1, product_code: 'codex', product_name: 'Codex API', current_status: 'monitoring', affected_at: new Date().toISOString(), compensable_seconds: 300, segments: [{ id: 1, started_at: new Date().toISOString(), ended_at: new Date().toISOString(), duration_seconds: 300 }] }], updates: [{ id: 1, phase: 'monitoring', kind: 'system', internal_message: 'recovered into monitoring', created_at: new Date().toISOString() }], public_timeline: [] }]
}

function mountView() { return mount(IncidentsView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } }) }
beforeEach(() => { vi.clearAllMocks(); api.getIncidentAdminSnapshot.mockResolvedValue(structuredClone(snapshot)); api.updateIncidentSettings.mockResolvedValue({ enabled: true, public_enabled: false }); api.confirmIncidentCandidate.mockResolvedValue(snapshot.incidents[0]); for (const key of ['dismissIncidentCandidate', 'transitionIncident', 'addIncidentUpdate', 'publishIncidentUpdate', 'acknowledgeIncidentEvidenceGap'] as const) api[key].mockResolvedValue(undefined) })

describe('IncidentsView', () => {
  it('renders candidates, affected products, segments and internal timeline', async () => {
    const wrapper = mountView(); await flushPromises()
    expect(wrapper.text()).toContain('Candidate #7')
    expect(wrapper.text()).toContain('Codex incident')
    expect(wrapper.text()).toContain('客户影响 5 分钟')
    expect(wrapper.text()).toContain('recovered into monitoring')
  })
  it('confirms a candidate through the editor', async () => {
    const wrapper = mountView(); await flushPromises()
    await wrapper.get('[aria-label="Incident 标题"]').setValue('Confirmed incident')
    await wrapper.get('[aria-label="内部摘要"]').setValue('evidence summary')
    await wrapper.get('[data-test="confirm-candidate"]').trigger('click')
    await flushPromises()
    expect(api.confirmIncidentCandidate).toHaveBeenCalledWith(7, 'Confirmed incident', 'evidence summary')
  })
  it('shows an explicit empty state and API errors', async () => {
    api.getIncidentAdminSnapshot.mockResolvedValue({ settings: { enabled: false, public_enabled: false }, generated_at: new Date().toISOString(), candidates: [], incidents: [] })
    const empty = mountView(); await flushPromises(); expect(empty.text()).toContain('当前没有待确认候选'); expect(empty.text()).toContain('尚无 Incident')
    api.getIncidentAdminSnapshot.mockRejectedValueOnce(new Error('temporary failure'))
    const failed = mountView(); await flushPromises(); expect(failed.get('[role="alert"]').text()).toContain('temporary failure')
  })
  it('makes an evidence gap explicit to operators', async () => {
    const data = structuredClone(snapshot); data.incidents[0].evidence_gap = true; api.getIncidentAdminSnapshot.mockResolvedValue(data)
    const wrapper = mountView(); await flushPromises(); expect(wrapper.get('[data-test="evidence-gap"]').text()).toContain('禁止 Resolved')
    await wrapper.get('[aria-label="证据缺口复核原因"]').setValue('已核对离线账本与恢复证据')
    await wrapper.get('[data-test="acknowledge-gap"]').trigger('click'); await flushPromises()
    expect(api.acknowledgeIncidentEvidenceGap).toHaveBeenCalledWith(9, '已核对离线账本与恢复证据')
  })
})
