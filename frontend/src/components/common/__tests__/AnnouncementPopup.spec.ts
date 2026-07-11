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

    store.currentPopup = popup
    await nextTick()
    expect(document.body.style.overflow).toBe('hidden')

    store.currentPopup = null
    await nextTick()
    expect(document.body.style.overflow).toBe('')
  })
})
