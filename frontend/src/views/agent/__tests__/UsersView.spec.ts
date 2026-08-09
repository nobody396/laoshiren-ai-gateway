import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsersView from '../UsersView.vue'

const getAgentInvitedUsers = vi.hoisted(() => vi.fn())

vi.mock('@/api/agent', () => ({
  getAgentInvitedUsers
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountView() {
  return mount(UsersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' }
      }
    }
  })
}

describe('agent UsersView direct invitee details', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getAgentInvitedUsers.mockReset()
    getAgentInvitedUsers.mockResolvedValue({
      items: [
        {
          user_id: 120,
          email: 'direct.invitee@example.com',
          username: 'Direct Invitee',
          joined_at: '2026-08-09T10:59:41Z',
          total_recharge: 20,
          total_consumption: 7.5,
          total_commission: 0.38
        }
      ],
      pagination: { page: 1, page_size: 20, total: 1, total_pages: 1 }
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the full email without exposing the internal user ID', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Direct Invitee')
    expect(wrapper.text()).toContain('direct.invitee@example.com')
    expect(wrapper.text()).not.toContain('#120')

    wrapper.unmount()
  })
})
