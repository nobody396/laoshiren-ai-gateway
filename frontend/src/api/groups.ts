/**
 * User Groups API endpoints (non-admin)
 * Handles group-related operations for regular users
 */

import { apiClient } from './client'
import type { Group } from '@/types'

export interface GroupCacheStats {
  hit_rate: number
  hit_rate_pct: number
  has_data: boolean
}

export interface GroupCacheStatsResponse {
  window_days: number
  groups: Record<number, GroupCacheStats>
}

/**
 * Get available groups that the current user can bind to API keys
 * This returns groups based on user's permissions:
 * - Standard groups: public (non-exclusive) or explicitly allowed
 * - Subscription groups: user has active subscription
 * @returns List of available groups
 */
export async function getAvailable(): Promise<Group[]> {
  const { data } = await apiClient.get<Group[]>('/groups/available')
  return data
}

/**
 * Get current user's custom group rate multipliers
 * @returns Map of group_id to custom rate_multiplier
 */
export async function getUserGroupRates(): Promise<Record<number, number>> {
  const { data } = await apiClient.get<Record<number, number> | null>('/groups/rates')
  return data || {}
}

/**
 * Get cache hit-rate stats for groups available to the current user.
 * This is display-only; failures should not block key creation.
 */
export async function getCacheStats(windowDays = 7): Promise<GroupCacheStatsResponse> {
  const { data } = await apiClient.get<GroupCacheStatsResponse>('/groups/cache-stats', {
    params: { window_days: windowDays }
  })
  return data
}

export const userGroupsAPI = {
  getAvailable,
  getUserGroupRates,
  getCacheStats
}

export default userGroupsAPI
