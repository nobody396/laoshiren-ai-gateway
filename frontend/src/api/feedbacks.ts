import { apiClient } from './client'
import type {
  BasePaginationResponse,
  CreateFeedbackReplyRequest,
  CreateFeedbackRequest,
  FeedbackDetail,
  FeedbackItem,
  UpdateFeedbackRequest,
	UserNotification,
} from '@/types'

export interface FeedbackListParams {
  page?: number
  pageSize?: number
  status?: string
}

export async function listFeedbacks(params: FeedbackListParams = {}): Promise<BasePaginationResponse<FeedbackItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackItem>>('/feedbacks', {
    params: {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
      status: params.status || undefined,
    },
  })
  return data
}

export async function getFeedback(id: number): Promise<FeedbackDetail> {
  const { data } = await apiClient.get<FeedbackDetail>(`/feedbacks/${id}`)
  return data
}

export async function createFeedback(payload: CreateFeedbackRequest): Promise<FeedbackItem> {
  const { data } = await apiClient.post<FeedbackItem>('/feedbacks', payload)
  return data
}

export async function updateFeedback(id: number, payload: UpdateFeedbackRequest): Promise<FeedbackItem> {
  const { data } = await apiClient.put<FeedbackItem>(`/feedbacks/${id}`, payload)
  return data
}

export async function createFeedbackReply(id: number, payload: CreateFeedbackReplyRequest): Promise<void> {
  await apiClient.post(`/feedbacks/${id}/replies`, payload)
}

export async function verifyFeedback(id: number, resolved: boolean, note = ''): Promise<void> {
  await apiClient.post(`/feedbacks/${id}/verification`, { resolved, note })
}

export async function listNotifications(limit = 30): Promise<{ items: UserNotification[]; unread_count: number }> {
  const { data } = await apiClient.get<{ items: UserNotification[]; unread_count: number }>('/notifications', { params: { limit } })
  return data
}

export async function markNotificationRead(id: number): Promise<void> { await apiClient.post(`/notifications/${id}/read`) }
export async function markAllNotificationsRead(): Promise<void> { await apiClient.post('/notifications/read-all') }

export async function uploadFeedbackImage(file: File): Promise<string> {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await apiClient.post<{ url: string }>('/feedbacks/upload-image', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
  return data.url
}

const feedbacksAPI = {
  list: listFeedbacks,
  getById: getFeedback,
  create: createFeedback,
  update: updateFeedback,
  createReply: createFeedbackReply,
	verify: verifyFeedback,
  uploadImage: uploadFeedbackImage,
}

export default feedbacksAPI
