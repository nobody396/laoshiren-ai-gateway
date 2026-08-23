import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
const api = vi.hoisted(() => ({ getCustomerTiers: vi.fn(), getCustomerTierUser: vi.fn(), updateCustomerTierSettings: vi.fn(), createCustomerTierOverride: vi.fn() }))
vi.mock('@/api/admin/customerTiers', () => api)
import CustomerTiersView from '../CustomerTiersView.vue'

const current = { user_id: 7, email: 'customer@example.com', calculated_tier: 'priority', effective_tier: 'priority', verified_paid_value_cny_fen: 25000, verified_paid_consumption_micros: 1000000, last_evaluation_id: 11, evaluated_at: new Date().toISOString() }
const snapshot = { enabled: true, generated_at: new Date().toISOString(), customers: [current], page: 1, page_size: 50, total: 1 }
const explanation = { current, evaluation: { id: 11, policy_version: 1, calculated_tier: 'priority', effective_tier: 'priority', resolution_reason: 'upgrade', verified_paid_value_cny_fen: 25000, verified_paid_consumption_micros: 1000000, evidence_hash: 'a'.repeat(64), evaluated_at: new Date().toISOString(), evidence: { window_started_at: new Date().toISOString(), window_ended_at: new Date().toISOString(), paid_sources: [{ source_type: 'topup_order', source_id: 1, gross_amount_cny_fen: 30000, refunded_amount_cny_fen: 5000, amount_cny_fen: 25000, occurred_at: new Date().toISOString(), included: true }, { source_type: 'redeem_code', source_id: 2, gross_amount_cny_fen: 0, refunded_amount_cny_fen: 0, amount_cny_fen: 0, occurred_at: new Date().toISOString(), included: false, exclusion_reason: 'unresolved_paid_value' }], balance_paid_consumption_micros: 400000, builder_pass_consumption_micros: 600000, verified_paid_consumption_micros: 1000000 } }, history: [{ id: 1, previous_tier: 'standard', new_tier: 'priority', change_reason: 'upgrade', changed_at: new Date().toISOString() }], overrides: [{ id: 3, tier: 'strategic', reason: 'contract term', starts_at: new Date().toISOString(), expires_at: new Date(Date.now() + 86400000).toISOString(), created_by_user_id: 9, created_at: new Date().toISOString(), refresh_status: 'applied' }], benefit: { multiplier: 1.25, cap_cny_fen: 5000 } }
function mountView() { return mount(CustomerTiersView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } }) }
beforeEach(() => { vi.clearAllMocks(); api.getCustomerTiers.mockResolvedValue(structuredClone(snapshot)); api.getCustomerTierUser.mockResolvedValue(structuredClone(explanation)); api.updateCustomerTierSettings.mockResolvedValue(undefined); api.createCustomerTierOverride.mockResolvedValue(undefined) })

describe('CustomerTiersView', () => {
  it('renders current tier and explanation-first paid evidence', async () => {
    const wrapper = mountView(); await flushPromises(); expect(wrapper.text()).toContain('customer@example.com'); expect(wrapper.text()).toContain('¥250.00')
    await wrapper.get('[data-test="tier-row"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('1.25x'); expect(wrapper.text()).toContain('unresolved_paid_value'); expect(wrapper.text()).toContain('Standard → Priority'); expect(wrapper.text()).toContain('contract term'); expect(wrapper.text()).toContain('操作者 #9')
  })
  it('creates a time-bounded audited override', async () => {
    const wrapper = mountView(); await flushPromises(); await wrapper.get('[data-test="tier-row"]').trigger('click'); await flushPromises()
    await wrapper.get('[aria-label="Override 等级"]').setValue('strategic'); await wrapper.get('[aria-label="Override 原因"]').setValue('contractual recovery handling'); await wrapper.get('[aria-label="Override 天数"]').setValue(14)
    await wrapper.findAll('button').find((button) => button.text().includes('创建审计'))!.trigger('click'); await flushPromises()
    expect(api.createCustomerTierOverride).toHaveBeenCalledWith(7, 'strategic', 'contractual recovery handling', 14)
  })
  it('shows empty and error states', async () => {
    api.getCustomerTiers.mockResolvedValueOnce({ enabled: false, generated_at: new Date().toISOString(), customers: [], page: 1, page_size: 50, total: 0 }); const empty = mountView(); await flushPromises(); expect(empty.text()).toContain('尚无客户等级评估')
    api.getCustomerTiers.mockRejectedValueOnce(new Error('temporary failure')); const failed = mountView(); await flushPromises(); expect(failed.get('[role="alert"]').text()).toContain('temporary failure')
  })

  it('keeps newest detail response during rapid selection', async () => {
    const wrapper = mountView(); await flushPromises()
    let resolveOld!: (value: typeof explanation) => void; let resolveNew!: (value: typeof explanation) => void
    api.getCustomerTierUser.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockImplementationOnce(() => new Promise(resolve => { resolveNew = resolve }))
    await wrapper.get('[data-test="tier-row"]').trigger('click'); await wrapper.get('[data-test="tier-row"]').trigger('click')
    resolveNew({ ...structuredClone(explanation), current: { ...current, email: 'newest@example.com' } }); await flushPromises()
    resolveOld({ ...structuredClone(explanation), current: { ...current, email: 'stale@example.com' } }); await flushPromises()
    expect(wrapper.text()).toContain('newest@example.com'); expect(wrapper.text()).not.toContain('stale@example.com')
  })

  it('keeps newest list response and aborts requests on unmount', async () => {
    const wrapper = mountView(); await flushPromises()
    let resolveOld!: (value: typeof snapshot) => void; let resolveNew!: (value: typeof snapshot) => void
    api.getCustomerTiers.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockImplementationOnce(() => new Promise(resolve => { resolveNew = resolve }))
    const searchButton = wrapper.findAll('button').find(button => button.text() === '搜索')!
    await searchButton.trigger('click'); await searchButton.trigger('click')
    resolveNew({ ...structuredClone(snapshot), customers: [{ ...current, email: 'latest-list@example.com' }] }); await flushPromises()
    resolveOld({ ...structuredClone(snapshot), customers: [{ ...current, email: 'stale-list@example.com' }] }); await flushPromises()
    expect(wrapper.text()).toContain('latest-list@example.com'); expect(wrapper.text()).not.toContain('stale-list@example.com')
    const signal = api.getCustomerTiers.mock.calls.at(-1)?.[0] as AbortSignal
    wrapper.unmount(); expect(signal.aborted).toBe(true)
  })

  it('does not launch follow-up reads when unmounted during a mutation', async () => {
    const wrapper = mountView(); await flushPromises(); await wrapper.get('[data-test="tier-row"]').trigger('click'); await flushPromises()
    let resolveAction!: () => void; api.createCustomerTierOverride.mockImplementationOnce(() => new Promise<void>(resolve => { resolveAction = resolve }))
    await wrapper.get('[aria-label="Override 原因"]').setValue('pending mutation')
    await wrapper.findAll('button').find(button => button.text().includes('创建审计'))!.trigger('click'); await flushPromises()
    expect(api.createCustomerTierOverride).toHaveBeenCalledTimes(1); wrapper.unmount(); resolveAction(); await flushPromises()
    expect(api.getCustomerTiers).toHaveBeenCalledTimes(1); expect(api.getCustomerTierUser).toHaveBeenCalledTimes(1)
  })
})
