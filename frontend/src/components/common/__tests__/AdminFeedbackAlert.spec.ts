import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent } from 'vue'

const listAgentQueueMock = vi.fn()

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params?.count ? `${key}:${params.count}` : key,
    }),
  }
})

vi.mock('@/api/admin/feedbacks', () => ({
  listAgentQueue: (...args: unknown[]) => listAgentQueueMock(...args),
}))

import AdminFeedbackAlert from '@/components/common/AdminFeedbackAlert.vue'

describe('AdminFeedbackAlert', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listAgentQueueMock.mockReset()
    listAgentQueueMock.mockResolvedValue({ items: [], total: 2, page: 1, page_size: 5, pages: 1 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows a direct admin entry when feedback awaits agent review', async () => {
    const wrapper = mount(AdminFeedbackAlert, {
      global: {
        plugins: [createPinia()],
        stubs: {
          RouterLink: defineComponent({
            props: ['to'],
            template: '<a :href="to"><slot /></a>',
          }),
          Icon: defineComponent({ template: '<span />' }),
        },
      },
    })
    await flushPromises()

    const alert = wrapper.get('[data-testid="admin-feedback-alert"]')
    expect(alert.attributes('href')).toBe('/admin/feedbacks')
    expect(alert.text()).toContain('2')
    expect(alert.attributes('aria-label')).toBe('feedback.admin.pendingInbox:2')

    wrapper.unmount()
  })
})
