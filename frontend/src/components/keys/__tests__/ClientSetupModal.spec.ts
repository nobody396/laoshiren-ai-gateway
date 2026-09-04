import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ClientSetupModal from '../ClientSetupModal.vue'
import type { ClientSetupPlan } from '@/api/resources'
const mocks = vi.hoisted(() => ({ plans: vi.fn(), ticket: vi.fn() }))
vi.mock('@/api/resources', () => ({ getClientSetupPlans: mocks.plans, createClientSetupTicketForPlan: mocks.ticket }))
const plan = (client = 'claude-code', os = 'macos'): ClientSetupPlan => ({ client_id: client, client_version_key: 'cli:2.1.258', os: os as ClientSetupPlan['os'], target: client === 'codex' ? 'codex' : 'claude', base_url: 'https://api.laoshirenai.com', group_ids: [6, 5], models: [{ id: 'claude-opus-5', protocol: 'messages' }], default_model: 'claude-opus-5', available: true, fingerprint: 'fixture-fingerprint' })
let wrapper: VueWrapper
const open = () => (wrapper = mount(ClientSetupModal, { props: { show: true, apiKeyId: 42, keyName: '测试 Key' }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } } }))
beforeEach(() => {
 vi.useFakeTimers(); mocks.plans.mockReset().mockImplementation((_id, os) => Promise.resolve([plan('claude-code', os), { ...plan('qoder', os), available: false, target: '', reason: '适配尚未开放' }]))
 mocks.ticket.mockReset().mockResolvedValue({ ticket: 'fixture-ticket', target: 'claude', expires_in: 600 })
})
afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })
describe('key client setup picker', () => {
 it('shows all 14 icons, reuses existing key scope and enables install-if-missing', async () => {
  const w = open(); await flushPromises()
  expect(w.findAll('.setup-tools img')).toHaveLength(14)
  expect(mocks.plans).toHaveBeenCalledWith(42, expect.any(String))
  expect(w.text()).toContain('2 个授权分组')
  await w.get('.btn-primary').trigger('click'); await flushPromises()
  expect(mocks.ticket).toHaveBeenCalledWith(42, expect.objectContaining({ fingerprint: 'fixture-fingerprint' }))
  expect(w.get('code').text()).toBe('claude-opus-5')
  expect(w.get('.docs-terminal-command code').text()).toContain("LAOSHIRENAI_SKIP_CLIENT_INSTALL='0'")
  expect(w.text()).not.toContain('sk-')
  vi.advanceTimersByTime(600_000); await w.vm.$nextTick()
  expect(w.find('.docs-terminal-command').exists()).toBe(false)
 })
 it('never generates a command for an unverified tool', async () => {
  const w = open(); await flushPromises(); await w.get('[data-client="qoder"]').trigger('click')
  expect(w.text()).toContain('不能跳过授权')
  expect(w.find('.btn-primary').exists()).toBe(false)
  expect(mocks.ticket).not.toHaveBeenCalled()
 })
 it('discards a ticket response after changing client or closing', async () => {
  let resolve!: (v: unknown) => void
  mocks.ticket.mockImplementation(() => new Promise(r => { resolve = r }))
  const w = open(); await flushPromises(); await w.get('.btn-primary').trigger('click')
  await w.get('[data-client="qoder"]').trigger('click')
  resolve({ ticket: 'stale-token', target: 'claude', expires_in: 600 }); await flushPromises()
  expect(w.text()).not.toContain('stale-token')
  expect(w.find('.docs-terminal-command').exists()).toBe(false)
 })
 it('discards a previous OS response and requests plans again', async () => {
  let resolve!: (v: ClientSetupPlan[]) => void
  mocks.plans.mockImplementationOnce(() => new Promise(r => { resolve = r }))
  const w = open(); await flushPromises(); await w.findAll('.setup-os button')[2].trigger('click'); await flushPromises()
  resolve([plan('claude-code', 'macos')]); await flushPromises()
  await w.get('.btn-primary').trigger('click'); await flushPromises()
  expect(mocks.ticket).toHaveBeenCalledWith(42, expect.objectContaining({ os: 'windows' }))
  expect(w.get('.docs-terminal-command code').text()).toContain('$env:LAOSHIRENAI_SETUP_TOKEN')
 })
 it('fails closed on plan load errors and does not retain an old command', async () => {
  const w = open(); await flushPromises(); await w.get('.btn-primary').trigger('click'); await flushPromises()
  mocks.plans.mockRejectedValueOnce(new Error('fixture error')); await w.findAll('.setup-os button')[2].trigger('click'); await flushPromises()
  expect(w.find('[role="alert"]').exists()).toBe(true)
  expect(w.find('.docs-terminal-command').exists()).toBe(false)
 })
})
