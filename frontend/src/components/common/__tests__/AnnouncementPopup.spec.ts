import { createPinia, setActivePinia } from 'pinia'
import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AnnouncementPopup from '../AnnouncementPopup.vue'
import { useAnnouncementStore } from '@/stores/announcements'
import type { UserAnnouncement } from '@/types'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const popup: UserAnnouncement = {
  id: 1,
  title: 'Maintenance complete',
  content: 'Service restored.',
  notify_mode: 'popup',
  created_at: '2026-07-11T00:00:00Z',
  updated_at: '2026-07-11T00:00:00Z'
}

let wrapper: VueWrapper | undefined

describe('AnnouncementPopup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.style.overflow = ''
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    document.body.style.overflow = ''
    document.body.innerHTML = ''
  })

  it('releases the landing page scroll lock when the popup closes', async () => {
    const store = useAnnouncementStore()
    wrapper = mount(AnnouncementPopup, { attachTo: document.body })

    store.popupBatch = {
      announcements: [popup],
      total: 1,
      throughAnnouncementId: popup.id
    }
    await nextTick()
    expect(document.body.style.overflow).toBe('hidden')

    store.dismissPopup()
    await nextTick()
    expect(document.body.style.overflow).toBe('')
  })

  it('shows one summary popup for a backlog instead of replaying every item', async () => {
    const store = useAnnouncementStore()
    const backlog = Array.from({ length: 10 }, (_, index) => ({
      ...popup,
      id: index + 1,
      title: `Announcement ${index + 1}`
    }))
    store.popupBatch = {
      announcements: backlog.slice(0, 3),
      total: backlog.length,
      throughAnnouncementId: 10
    }

    wrapper = mount(AnnouncementPopup, { attachTo: document.body })
    await nextTick()

    expect(document.body.querySelectorAll('[data-testid="announcement-popup"]')).toHaveLength(1)
    expect(document.body.querySelectorAll('[data-testid="announcement-popup-batch"] h3')).toHaveLength(3)
    expect(document.body.textContent).toContain('announcements.newCount')

    const buttons = Array.from(document.body.querySelectorAll('button'))
    const laterButton = buttons.find((button) => button.textContent?.includes('announcements.viewLater'))
    laterButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()

    expect(store.popupBatch).toBeNull()
    expect(document.body.querySelectorAll('[data-testid="announcement-popup"]')).toHaveLength(0)
  })
})
