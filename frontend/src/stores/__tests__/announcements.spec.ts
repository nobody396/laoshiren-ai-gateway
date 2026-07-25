import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAnnouncementStore } from '@/stores/announcements'
import type { UserAnnouncement } from '@/types'

const mockList = vi.fn()
const mockGetPopupState = vi.fn()
const mockMarkPopupBatchPrompted = vi.fn()
const mockMarkRead = vi.fn()

vi.mock('@/api', () => ({
  announcementsAPI: {
    list: (...args: unknown[]) => mockList(...args),
    getPopupState: (...args: unknown[]) => mockGetPopupState(...args),
    markPopupBatchPrompted: (...args: unknown[]) => mockMarkPopupBatchPrompted(...args),
    markRead: (...args: unknown[]) => mockMarkRead(...args)
  }
}))

function announcement(id: number, overrides: Partial<UserAnnouncement> = {}): UserAnnouncement {
  return {
    id,
    title: `Announcement ${id}`,
    content: `Content ${id}`,
    notify_mode: 'popup',
    created_at: `2026-07-25T${String(id % 24).padStart(2, '0')}:00:00Z`,
    updated_at: `2026-07-25T${String(id % 24).padStart(2, '0')}:00:00Z`,
    ...overrides
  }
}

describe('useAnnouncementStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockGetPopupState.mockResolvedValue({ last_prompted_announcement_id: 0 })
    mockMarkPopupBatchPrompted.mockResolvedValue({ message: 'ok' })
    mockMarkRead.mockResolvedValue({ message: 'ok' })
  })

  it('collapses a long unread popup backlog into one three-item preview', async () => {
    mockList.mockResolvedValue(
      Array.from({ length: 10 }, (_, index) => announcement(10 - index))
    )
    const store = useAnnouncementStore()

    await store.fetchAnnouncements(true)

    expect(store.popupBatch).toEqual({
      announcements: expect.arrayContaining([
        expect.objectContaining({ id: 10 }),
        expect.objectContaining({ id: 9 }),
        expect.objectContaining({ id: 8 })
      ]),
      total: 10,
      throughAnnouncementId: 10
    })
    expect(store.popupBatch?.announcements).toHaveLength(3)
    expect(mockMarkPopupBatchPrompted).toHaveBeenCalledTimes(1)
    expect(mockMarkPopupBatchPrompted).toHaveBeenCalledWith(10)

    store.dismissPopup()
    await store.fetchAnnouncements(true)

    expect(store.popupBatch).toBeNull()
    expect(mockMarkPopupBatchPrompted).toHaveBeenCalledTimes(1)
  })

  it('does not replay announcements already covered by the server popup cursor', async () => {
    mockList.mockResolvedValue([announcement(21), announcement(20), announcement(19)])
    mockGetPopupState.mockResolvedValue({ last_prompted_announcement_id: 21 })
    const store = useAnnouncementStore()

    await store.fetchAnnouncements(true)

    expect(store.popupBatch).toBeNull()
    expect(mockMarkPopupBatchPrompted).not.toHaveBeenCalled()
    expect(store.unreadCount).toBe(3)
  })

  it('keeps an exact unread count and makes the full backlog available in the center', async () => {
    mockList.mockResolvedValue(
      Array.from({ length: 25 }, (_, index) => announcement(25 - index, {
        notify_mode: 'silent'
      }))
    )
    const store = useAnnouncementStore()

    await store.fetchAnnouncements(true)

    expect(store.announcements).toHaveLength(25)
    expect(store.unreadCount).toBe(25)
  })

  it('closing a batch does not mark its announcements as read', async () => {
    mockList.mockResolvedValue([announcement(2), announcement(1)])
    const store = useAnnouncementStore()
    await store.fetchAnnouncements(true)

    store.dismissPopup()

    expect(store.unreadCount).toBe(2)
    expect(mockMarkRead).not.toHaveBeenCalled()
  })

  it('still loads announcements when popup cursor persistence is unavailable', async () => {
    mockList.mockResolvedValue([announcement(3), announcement(2)])
    mockGetPopupState.mockRejectedValue(new Error('temporary state failure'))
    const store = useAnnouncementStore()

    await store.fetchAnnouncements(true)

    expect(store.unreadCount).toBe(2)
    expect(store.popupBatch?.total).toBe(2)
  })
})
