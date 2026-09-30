import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
const api = vi.hoisted(() => {
  vi.stubGlobal('localStorage', { getItem: vi.fn(() => null), setItem: vi.fn() })
  return { list: vi.fn(), listEnabledDefinitions: vi.fn().mockResolvedValue([]), getAll: vi.fn().mockResolvedValue([]) }
})
vi.mock('@/api/admin', () => ({ adminAPI: { users: { list: api.list }, userAttributes: { listEnabledDefinitions: api.listEnabledDefinitions }, groups: { getAll: api.getAll } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
import UsersView from '../UsersView.vue'

function mountView() {
  return shallowMount(UsersView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/></div>' },
    DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id" :data-user="row.id"><slot name="cell-status" :value="row.status" :row="row"/><slot name="cell-balance" :value="row.balance" :row="row"/><slot name="cell-actions" :row="row"/></div></div>' }
  } } })
}

beforeEach(() => {
  api.list.mockReset().mockResolvedValue({ items: [], total: 0, pages: 0 })
})

describe('deleted user audit view', () => {
  it('defaults to active records and opts in through a visible checkbox', async () => {
    const wrapper = mountView(); await flushPromises()
    expect(api.list.mock.lastCall?.[2].include_deleted).toBe(false)
    const checkbox = wrapper.get('input[type="checkbox"]')
    await checkbox.setValue(true); await flushPromises()
    expect(api.list.mock.lastCall?.[2].include_deleted).toBe(true)
    await checkbox.setValue(false); await flushPromises()
    expect(api.list.mock.lastCall?.[2].include_deleted).toBe(false)
    wrapper.unmount()
  })
  it('labels deleted users and hides mutation actions while preserving active-user actions', async () => {
    api.list.mockResolvedValue({ items: [
      { id: 1, status: 'active', balance: 0, deleted_at: '2026-09-01T00:00:00Z' },
      { id: 2, status: 'active', balance: 0 }
    ], total: 2, pages: 1 })
    const wrapper = mountView(); await flushPromises()
    const deleted = wrapper.get('[data-user="1"]')
    expect(deleted.text()).toContain('admin.users.deleted')
    expect(deleted.findAll('button').filter(b => b.attributes('disabled') === undefined)).toHaveLength(0)
    expect(wrapper.get('[data-user="2"]').text()).toContain('common.active')
    expect(wrapper.get('[data-user="2"]').findAll('button').length).toBeGreaterThan(1)
    wrapper.unmount()
  })
})
