import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { listAgentQueue } from '@/api/admin/feedbacks'
import { useAdminFeedbackInboxStore } from '@/stores/adminFeedbackInbox'

vi.mock('@/api/admin/feedbacks', () => ({
  listAgentQueue: vi.fn(),
}))

describe('useAdminFeedbackInboxStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(listAgentQueue).mockReset()
  })

  it('uses the unreviewed agent queue total as the pending badge count', async () => {
    vi.mocked(listAgentQueue).mockResolvedValue({
      items: [{ id: 11, title: '新反馈' }] as any,
      total: 3,
      page: 1,
      page_size: 5,
      pages: 1,
    })
    const store = useAdminFeedbackInboxStore()

    await store.refresh()

    expect(listAgentQueue).toHaveBeenCalledWith({ page: 1, pageSize: 5 })
    expect(store.pendingCount).toBe(3)
    expect(store.items).toHaveLength(1)
    expect(store.loaded).toBe(true)
  })

  it('keeps the last successful count when polling temporarily fails', async () => {
    vi.mocked(listAgentQueue)
      .mockResolvedValueOnce({ items: [], total: 2, page: 1, page_size: 5, pages: 1 })
      .mockRejectedValueOnce(new Error('network unavailable'))
    const store = useAdminFeedbackInboxStore()

    await store.refresh()
    await store.refresh()

    expect(store.pendingCount).toBe(2)
    expect(store.loaded).toBe(true)
  })
})
