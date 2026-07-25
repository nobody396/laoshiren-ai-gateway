import { apiClient } from '../client'
import type {
  AdminChangelogEntry,
  BasePaginationResponse,
  CreateChangelogRequest,
  UpdateChangelogRequest
} from '@/types'

export async function list(
  page = 1,
  pageSize = 20,
  filters?: { status?: string; category?: string; search?: string }
): Promise<BasePaginationResponse<AdminChangelogEntry>> {
  const { data } = await apiClient.get<BasePaginationResponse<AdminChangelogEntry>>('/admin/changelog', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getById(id: number): Promise<AdminChangelogEntry> {
  const { data } = await apiClient.get<AdminChangelogEntry>(`/admin/changelog/${id}`)
  return data
}

export async function create(request: CreateChangelogRequest): Promise<AdminChangelogEntry> {
  const { data } = await apiClient.post<AdminChangelogEntry>('/admin/changelog', request)
  return data
}

export async function update(id: number, request: UpdateChangelogRequest): Promise<AdminChangelogEntry> {
  const { data } = await apiClient.put<AdminChangelogEntry>(`/admin/changelog/${id}`, request)
  return data
}

export async function deleteEntry(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/changelog/${id}`)
  return data
}

const changelogAdminAPI = { list, getById, create, update, delete: deleteEntry }

export default changelogAdminAPI
