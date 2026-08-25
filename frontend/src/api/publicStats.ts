/**
 * Public landing stats API (no auth required)
 * 落地页计数器数据来源:token 处理量与累计赔付金额。
 * 显示倍率(×10)由服务端统一应用,前端直接使用返回值,
 * 保证页面与 API 数值永远一致(见 backend public_stats_service.go)。
 */

import { apiClient } from './client'

export interface PublicStats {
  /** 累计处理 token 数(已含服务端显示倍率) */
  tokens_total: number
  /** 累计请求数 */
  requests_total: number
  /** 累计赔付金额(元,赔付 + 赠送合并口径) */
  compensation_cny: number
  /** 数据更新时间(ISO 字符串) */
  updated_at: string
}

/**
 * 获取落地页公开统计数据(无需登录)。
 */
export async function getPublicStats(): Promise<PublicStats> {
  const { data } = await apiClient.get<PublicStats>('/public/stats')
  return data
}
