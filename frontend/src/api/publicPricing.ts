/**
 * Public model pricing API (no auth required)
 * 独立模型价格页的数据来源：全部 active 分组 × 模型 × 实付价（元/1M tokens）。
 */

import { apiClient } from './client'

export interface PublicModelPrice {
  model: string
  /** 元/1M tokens，价格未知时为 null */
  input_price: number | null
  output_price: number | null
  cache_read_price: number | null
}

export interface PublicPricingGroup {
  group_id: number
  name: string
  platform: string
  rate_multiplier: number
  is_exclusive: boolean
  subscription_type: string
  models: PublicModelPrice[]
}

export interface PublicModelPricingCatalog {
  updated_at: string
  currency: string
  unit: string
  groups: PublicPricingGroup[]
}

/**
 * 获取公开模型价格目录（无需登录）。
 */
export async function getPublicModelPricing(): Promise<PublicModelPricingCatalog> {
  const { data } = await apiClient.get<PublicModelPricingCatalog>('/public/model-pricing')
  return data
}
