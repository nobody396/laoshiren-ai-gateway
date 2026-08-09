import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

const listNotificationsMock = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/api/feedbacks', () => ({
  listNotifications: (...args: unknown[]) => listNotificationsMock(...args),
  markAllNotificationsRead: vi.fn(),
  markNotificationRead: vi.fn(),
}))

import UserNotificationBell from '@/components/common/UserNotificationBell.vue'

describe('UserNotificationBell', () => {
  beforeEach(() => {
    listNotificationsMock.mockReset()
    listNotificationsMock.mockResolvedValue({
      unread_count: 1,
      items: [{
        id: 1,
        title: '你反馈的问题已修复',
        body: '请进入反馈详情验证是否已经解决。',
        action_url: '/feedbacks/4',
        read_at: null,
        created_at: '2026-08-09T02:27:26Z',
      }],
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('teleports the dropdown above the sidebar and keeps it inside the viewport', async () => {
    const wrapper = mount(UserNotificationBell, {
      attachTo: document.body,
      global: {
        stubs: {
          Icon: defineComponent({ template: '<span />' }),
        },
      },
    })
    await flushPromises()

    const trigger = wrapper.get('button')
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue({
      x: 440,
      y: 32,
      top: 32,
      right: 500,
      bottom: 64,
      left: 440,
      width: 60,
      height: 32,
      toJSON: () => ({}),
    })
    await trigger.trigger('click')
    await flushPromises()

    const panel = document.body.querySelector<HTMLElement>('[data-testid="user-notification-panel"]')
    expect(panel).not.toBeNull()
    expect(panel?.className).toContain('fixed')
    expect(panel?.className).toContain('z-[120]')
    expect(panel?.style.left).toBe('116px')
    expect(panel?.style.top).toBe('72px')
    expect(panel?.style.width).toBe('384px')
    expect(panel?.textContent).toContain('你反馈的问题已修复')

    wrapper.unmount()
  })
})
