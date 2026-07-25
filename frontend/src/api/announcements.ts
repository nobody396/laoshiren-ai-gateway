/**
 * User Announcements API endpoints
 */

import { apiClient } from './client'
import type { UserAnnouncement } from '@/types'

export async function list(unreadOnly: boolean = false): Promise<UserAnnouncement[]> {
  const { data } = await apiClient.get<UserAnnouncement[]>('/announcements', {
    params: unreadOnly ? { unread_only: 1 } : {}
  })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/read`)
  return data
}

export async function getPopupState(): Promise<{ last_prompted_announcement_id: number }> {
  const { data } = await apiClient.get<{ last_prompted_announcement_id: number }>(
    '/announcements/popup-state'
  )
  return data
}

export async function markPopupBatchPrompted(
  throughAnnouncementId: number
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/announcements/popup-prompted', {
    through_announcement_id: throughAnnouncementId
  })
  return data
}

const announcementsAPI = {
  list,
  markRead,
  getPopupState,
  markPopupBatchPrompted
}

export default announcementsAPI
