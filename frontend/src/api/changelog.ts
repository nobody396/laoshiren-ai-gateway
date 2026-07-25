import { apiClient } from './client'
import type { BasePaginationResponse, PublicChangelogEntry } from '@/types'

export async function list(
  page = 1,
  pageSize = 12,
  filters?: { category?: string; search?: string }
): Promise<BasePaginationResponse<PublicChangelogEntry>> {
  const { data } = await apiClient.get<BasePaginationResponse<PublicChangelogEntry>>('/changelog', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getBySlug(slug: string): Promise<PublicChangelogEntry> {
  const { data } = await apiClient.get<PublicChangelogEntry>(`/changelog/${encodeURIComponent(slug)}`)
  return data
}

export async function latest(): Promise<PublicChangelogEntry | null> {
  const { data } = await apiClient.get<PublicChangelogEntry | null>('/changelog/latest')
  return data
}

export const changelogAPI = { list, getBySlug, latest }

export default changelogAPI
