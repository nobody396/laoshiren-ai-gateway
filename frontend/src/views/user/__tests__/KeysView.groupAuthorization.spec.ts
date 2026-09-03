import { shallowMount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import KeysView from '../KeysView.vue'

const m = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), groups: vi.fn(), error: vi.fn(), replace: vi.fn(),
}))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ replace: m.replace }) }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: m.error, showSuccess: vi.fn(), cachedPublicSettings: { team_enabled: true } }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ hasActiveSubscriptions: true }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('@/api', () => ({
  keysAPI: { list: m.list, create: m.create, update: m.update },
  userGroupsAPI: { getAvailable: m.groups, getUserGroupRates: vi.fn().mockResolvedValue({}) },
  usageAPI: { getDashboardApiKeysUsage: vi.fn().mockResolvedValue({ stats: {} }) },
  authAPI: { getPublicSettings: vi.fn().mockResolvedValue({}) }, resourcesAPI: {},
}))
const personal = [
  { id: 6, name: 'GPT', platform: 'openai', subscription_type: 'standard', rate_multiplier: 1 },
  { id: 5, name: 'Claude subscription', platform: 'anthropic', subscription_type: 'subscription', rate_multiplier: 1 },
]
const wrappers: ReturnType<typeof shallowMount>[] = []
async function page() {
  const wrapper = shallowMount(KeysView)
  wrappers.push(wrapper)
  await flushPromises()
  // Integration-test the actual page state/actions; component DOM interactions
  // are separately covered by KeyGroupMultiSelect and the browser fixture.
  return wrapper.vm as unknown as {
    showCreateModal: boolean; selectedGroupIds: number[]; keyGroupMode: string;
    formData: { name: string }; groups: { id: number }[];
    groupsLoadedScope: string | null;
    handleSubmit(): Promise<void>; editKey(key: unknown): void;
    setScope(scope: 'personal' | 'team'): Promise<void>; loadGroups(): Promise<void>;
  }
}
beforeEach(() => {
  vi.clearAllMocks()
  m.list.mockResolvedValue({ items: [], total: 0, pages: 0 })
  m.groups.mockResolvedValue(personal)
  m.create.mockResolvedValue({ id: 1 })
  m.update.mockResolvedValue({ id: 1 })
  m.replace.mockResolvedValue(undefined)
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.restoreAllMocks() })

describe('KeysView group authorization integration', () => {
  it('opens with a current-scope snapshot and submits ordered multi-group authorization', async () => {
    const vm = await page()
    vm.showCreateModal = true
    await nextTick()
    expect(vm.selectedGroupIds).toEqual([5, 6])
    vm.selectedGroupIds = [6, 5]
    vm.formData.name = 'one key'
    await vm.handleSubmit()
    expect(m.create).toHaveBeenCalledTimes(1)
    expect(m.create.mock.calls[0][8]).toBe('personal')
    expect(m.create.mock.calls[0][9]).toEqual([6, 5])
  })
  it('edits multi-group and legacy keys without mixing group_id and group_ids', async () => {
    const vm = await page()
    const key = { id: 1, name: 'key', group_id: null, group_ids: [6, 5], status: 'active', ip_whitelist: [], ip_blacklist: [], quota: 0 }
    vm.editKey(key)
    await vm.handleSubmit()
    expect(m.update.mock.calls[0][1].group_ids).toEqual([6, 5])
    expect(m.update.mock.calls[0][1]).not.toHaveProperty('group_id')
    vm.editKey({ ...key, group_ids: undefined, group_id: 6 })
    await vm.handleSubmit()
    expect(m.update.mock.calls[1][1].group_id).toBe(6)
    expect(m.update.mock.calls[1][1]).not.toHaveProperty('group_ids')
  })
  it('fails closed after scope loading fails, then permits a successful retry', async () => {
    const vm = await page()
    m.groups.mockRejectedValueOnce(new Error('offline'))
    await vm.setScope('team')
    vm.showCreateModal = true
    await nextTick()
    expect(vm.groupsLoadedScope).toBeNull()
    expect(vm.selectedGroupIds).toEqual([])
    await vm.handleSubmit()
    expect(m.create).not.toHaveBeenCalled()
    m.groups.mockResolvedValueOnce([{ ...personal[0], id: 57 }])
    await vm.loadGroups()
    expect(vm.selectedGroupIds).toEqual([57])
    await vm.handleSubmit()
    expect(m.create.mock.calls[0][8]).toBe('team')
    expect(m.create.mock.calls[0][9]).toEqual([57])
  })
  it('ignores a late response from a previous scope', async () => {
    const vm = await page()
    let resolveTeam!: (groups: unknown[]) => void
    m.groups.mockImplementationOnce(() => new Promise(resolve => { resolveTeam = resolve }))
    const first = vm.setScope('team')
    await flushPromises()
    await vm.setScope('personal')
    resolveTeam([{ ...personal[0], id: 99 }])
    await first
    expect(vm.groups.map(group => group.id)).toEqual([6, 5])
    expect(vm.groupsLoadedScope).toBe('personal')
  })
})
