import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AgentsView from '../AgentsView.vue'

const {
  getRates,
  updateInviteActivity,
  getSettlementSettings,
  getLevelRules,
  listSettlementCandidates,
  listUsers,
  listCommissions,
  listSettlements
} = vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn()
  })

  return {
    getRates: vi.fn(),
    updateInviteActivity: vi.fn(),
    getSettlementSettings: vi.fn(),
    getLevelRules: vi.fn(),
    listSettlementCandidates: vi.fn(),
    listUsers: vi.fn(),
    listCommissions: vi.fn(),
    listSettlements: vi.fn()
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    agents: {
      getRates,
      updateInviteActivity,
      getSettlementSettings,
      getLevelRules,
      listSettlementCandidates,
      listUsers,
      listCommissions,
      listSettlements
    }
  }
}))

const showSuccess = vi.fn()
const showError = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('@/utils/authError', () => ({
  buildAuthErrorMessage: (_error: unknown, options: { fallback: string }) => options.fallback
}))

vi.mock('@/utils/imagePreview', () => ({
  imageBlobToDataURL: vi.fn()
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

function mountAgentsView() {
  return mount(AgentsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue'],
          template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
        }
      }
    }
  })
}

describe('admin AgentsView invite activity config', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getRates.mockResolvedValue({
      consumption_rate: 0.06,
      first_recharge_invitee_rate: 0.1,
      first_recharge_referral_rate: 0.05,
      invite_activity: {
        enabled: true,
        name: '公测活动',
        start_at: '2026-05-20T00:00:00+08:00',
        end_at: '2026-05-21T00:00:00+08:00',
        registration_bonus_amount: 5,
        email_restriction_enabled: true,
        email_suffix_whitelist: ['@qq.com', '@gmail.com']
      }
    })
    updateInviteActivity.mockResolvedValue({
      enabled: true,
      name: '公测活动',
      registration_bonus_amount: 5,
      email_restriction_enabled: true,
      email_suffix_whitelist: ['@qq.com', '@gmail.com']
    })
    getSettlementSettings.mockResolvedValue({ minimum_amount: 50 })
    getLevelRules.mockResolvedValue([])
    listSettlementCandidates.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
    listCommissions.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
    listSettlements.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
  })

  it('loads and saves public beta invite activity values', async () => {
    const wrapper = mountAgentsView()
    await flushPromises()

    expect((wrapper.get('[data-test="invite-activity-registration-bonus"]').element as HTMLInputElement).value).toBe('5')
    expect(wrapper.find('[data-test="invite-activity-first-recharge-rate"]').exists()).toBe(false)
    expect((wrapper.get('[data-test="invite-activity-email-restriction-enabled"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-test="invite-activity-email-whitelist"]').element as HTMLTextAreaElement).value).toContain('@qq.com')

    await wrapper.get('[data-test="invite-activity-save"]').trigger('click')
    await flushPromises()

    expect(updateInviteActivity).toHaveBeenCalledWith(expect.objectContaining({
      enabled: true,
      name: '公测活动',
      registration_bonus_amount: 5,
      email_restriction_enabled: true,
      email_suffix_whitelist: ['@qq.com', '@gmail.com']
    }))
    expect(updateInviteActivity.mock.calls[0][0]).not.toHaveProperty('first_recharge_invitee_rate')
    expect(showSuccess).toHaveBeenCalledWith('admin.agents.inviteActivityUpdated')
  })
})
