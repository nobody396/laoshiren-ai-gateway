import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { announcementsAPI } from '@/api'
import type { UserAnnouncement } from '@/types'

const THROTTLE_MS = 20 * 60 * 1000 // 20 minutes
const POPUP_PREVIEW_LIMIT = 3

export interface AnnouncementPopupBatch {
  announcements: UserAnnouncement[]
  total: number
  throughAnnouncementId: number
}

export const useAnnouncementStore = defineStore('announcements', () => {
  const allAnnouncements = ref<UserAnnouncement[]>([])
  const loading = ref(false)
  const lastFetchTime = ref(0)
  const popupBatch = ref<AnnouncementPopupBatch | null>(null)
  const lastPromptedAnnouncementId = ref(0)
  const isCenterOpen = ref(false)

  const announcements = computed(() => allAnnouncements.value)
  const unreadCount = computed(() =>
    allAnnouncements.value.filter((announcement) => !announcement.read_at).length
  )
  const currentPopup = computed(() => popupBatch.value?.announcements[0] ?? null)

  async function fetchAnnouncements(force = false) {
    const now = Date.now()
    if (!force && lastFetchTime.value > 0 && now - lastFetchTime.value < THROTTLE_MS) {
      return
    }

    lastFetchTime.value = now

    try {
      loading.value = true
      const all = await announcementsAPI.list(false)
      allAnnouncements.value = all
      try {
        const popupState = await announcementsAPI.getPopupState()
        lastPromptedAnnouncementId.value = Math.max(
          lastPromptedAnnouncementId.value,
          popupState.last_prompted_announcement_id
        )
      } catch (err: any) {
        // Announcement listing remains useful even if delivery-state persistence is unavailable.
        console.error('Failed to fetch announcement popup state:', err)
      }
      preparePopupBatch()
    } catch (err: any) {
      lastFetchTime.value = 0
      console.error('Failed to fetch announcements:', err)
    } finally {
      loading.value = false
    }
  }

  function preparePopupBatch() {
    if (popupBatch.value) return

    const candidates = allAnnouncements.value.filter(
      (announcement) =>
        announcement.notify_mode === 'popup' &&
        !announcement.read_at &&
        announcement.id > lastPromptedAnnouncementId.value
    )
    if (candidates.length === 0) return

    const throughAnnouncementId = Math.max(...candidates.map((announcement) => announcement.id))
    popupBatch.value = {
      announcements: candidates.slice(0, POPUP_PREVIEW_LIMIT),
      total: candidates.length,
      throughAnnouncementId
    }

    // Optimistically advance the in-session cursor so refreshes cannot replay the batch.
    // The server persists the same monotonic cursor for future logins and other devices.
    lastPromptedAnnouncementId.value = throughAnnouncementId
    void announcementsAPI.markPopupBatchPrompted(throughAnnouncementId).catch((err: any) => {
      console.error('Failed to persist announcement popup state:', err)
    })
  }

  function dismissPopup() {
    popupBatch.value = null
  }

  async function acknowledgeCurrentPopup() {
    const announcement = currentPopup.value
    if (!announcement) return
    await markAsRead(announcement.id)
    dismissPopup()
  }

  async function markAsRead(id: number) {
    try {
      await announcementsAPI.markRead(id)
      const announcement = allAnnouncements.value.find((item) => item.id === id)
      if (announcement) {
        announcement.read_at = new Date().toISOString()
      }
    } catch (err: any) {
      console.error('Failed to mark announcement as read:', err)
      throw err
    }
  }

  async function markAllAsRead() {
    const unread = allAnnouncements.value.filter((announcement) => !announcement.read_at)
    if (unread.length === 0) return

    try {
      loading.value = true
      await Promise.all(unread.map((announcement) => announcementsAPI.markRead(announcement.id)))
      allAnnouncements.value.forEach((announcement) => {
        if (!announcement.read_at) {
          announcement.read_at = new Date().toISOString()
        }
      })
    } catch (err: any) {
      console.error('Failed to mark all as read:', err)
      throw err
    } finally {
      loading.value = false
    }
  }

  function openCenter() {
    isCenterOpen.value = true
  }

  function closeCenter() {
    isCenterOpen.value = false
  }

  function reset() {
    allAnnouncements.value = []
    lastFetchTime.value = 0
    popupBatch.value = null
    lastPromptedAnnouncementId.value = 0
    isCenterOpen.value = false
    loading.value = false
  }

  return {
    announcements,
    loading,
    popupBatch,
    currentPopup,
    isCenterOpen,
    unreadCount,
    fetchAnnouncements,
    dismissPopup,
    acknowledgeCurrentPopup,
    markAsRead,
    markAllAsRead,
    openCenter,
    closeCenter,
    reset,
  }
})
