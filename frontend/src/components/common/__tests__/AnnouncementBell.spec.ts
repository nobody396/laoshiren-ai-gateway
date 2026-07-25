import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AnnouncementBell from '../AnnouncementBell.vue'
import { useAnnouncementStore } from '@/stores/announcements'
import type { UserAnnouncement } from '@/types'

const mockList = vi.fn()
const mockGetPopupState = vi.fn()

vi.mock('@/api', () => ({
  announcementsAPI: {
    list: (...args: unknown[]) => mockList(...args),
    getPopupState: (...args: unknown[]) => mockGetPopupState(...args),
    markPopupBatchPrompted: vi.fn(),
    markRead: vi.fn()
  }
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

function silentAnnouncement(id: number): UserAnnouncement {
  return {
    id,
    title: `Announcement ${id}`,
    content: `Content ${id}`,
    notify_mode: 'silent',
    created_at: '2026-07-25T00:00:00Z',
    updated_at: '2026-07-25T00:00:00Z'
  }
}

describe('AnnouncementBell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockGetPopupState.mockResolvedValue({ last_prompted_announcement_id: 0 })
  })

  it('renders the exact unread count in the top-right badge', async () => {
    mockList.mockResolvedValue(
      Array.from({ length: 25 }, (_, index) => silentAnnouncement(index + 1))
    )
    const store = useAnnouncementStore()
    await store.fetchAnnouncements(true)

    const wrapper = mount(AnnouncementBell, {
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="announcement-unread-badge"]').text()).toBe('25')
  })

  it('caps a large unread badge at 99+', async () => {
    mockList.mockResolvedValue(
      Array.from({ length: 120 }, (_, index) => silentAnnouncement(index + 1))
    )
    const store = useAnnouncementStore()
    await store.fetchAnnouncements(true)

    const wrapper = mount(AnnouncementBell, {
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="announcement-unread-badge"]').text()).toBe('99+')
  })
})
