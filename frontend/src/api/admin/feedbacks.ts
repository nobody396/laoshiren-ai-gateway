import { apiClient } from '../client'
import type {
  BasePaginationResponse,
  CreateFeedbackReplyRequest,
  FeedbackDetail,
  FeedbackItem,
  FeedbackPriority,
  FeedbackStatus,
	FeedbackReward,
} from '@/types'

export interface AdminFeedbackListParams {
  page?: number
  pageSize?: number
  category?: string
  status?: string
  priority?: string
  search?: string
  start_time?: string
  end_time?: string
}

export interface FeedbackRewardListParams { page?: number; pageSize?: number; user_id?: number; batch_id?: string }

export async function listFeedbacks(params: AdminFeedbackListParams = {}): Promise<BasePaginationResponse<FeedbackItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackItem>>('/admin/feedbacks', {
    params: {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
      category: params.category || undefined,
      status: params.status || undefined,
      priority: params.priority || undefined,
      search: params.search || undefined,
      start_time: params.start_time || undefined,
      end_time: params.end_time || undefined,
    },
  })
  return data
}

/**
 * Agent 待核查队列也是管理员界面“新反馈”角标的唯一口径。
 * 完成核查后工单会离开该队列，因此不需要再维护一套容易漂移的已读状态。
 */
export async function listAgentQueue(params: Pick<AdminFeedbackListParams, 'page' | 'pageSize'> = {}): Promise<BasePaginationResponse<FeedbackItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackItem>>('/admin/feedbacks/agent-queue', {
    params: {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 5,
    },
  })
  return data
}

export async function getFeedback(id: number): Promise<FeedbackDetail> {
  const { data } = await apiClient.get<FeedbackDetail>(`/admin/feedbacks/${id}`)
  return data
}

export async function createReply(id: number, payload: CreateFeedbackReplyRequest): Promise<void> {
  await apiClient.post(`/admin/feedbacks/${id}/replies`, payload)
}

export async function updateStatus(id: number, status: FeedbackStatus): Promise<void> {
  await apiClient.put(`/admin/feedbacks/${id}/status`, { status })
}

export async function updatePriority(id: number, priority: FeedbackPriority): Promise<void> {
  await apiClient.put(`/admin/feedbacks/${id}/priority`, { priority })
}

export async function batchUpdateStatus(ids: number[], status: FeedbackStatus): Promise<{ updated: number }> {
  const { data } = await apiClient.put<{ updated: number }>('/admin/feedbacks/batch-status', {
    ids,
    status,
  })
  return data
}

export async function deleteFeedback(id: number): Promise<void> {
  await apiClient.delete(`/admin/feedbacks/${id}`)
}

export async function batchDeleteFeedbacks(ids: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/feedbacks/batch-delete', { ids })
  return data
}

export async function listRewards(params: FeedbackRewardListParams = {}): Promise<BasePaginationResponse<FeedbackReward>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackReward>>('/admin/feedbacks/rewards', { params: { page: params.page ?? 1, page_size: params.pageSize ?? 20, user_id: params.user_id, batch_id: params.batch_id || undefined } })
  return data
}

const adminFeedbacksAPI = {
  list: listFeedbacks,
	listAgentQueue,
  getById: getFeedback,
  createReply,
  updateStatus,
  updatePriority,
  batchUpdateStatus,
  delete: deleteFeedback,
  batchDelete: batchDeleteFeedbacks,
	listRewards,
}

export default adminFeedbacksAPI
