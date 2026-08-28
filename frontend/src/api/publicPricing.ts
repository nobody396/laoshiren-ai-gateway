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
  cache_write_price: number | null
  cache_read_price: number | null
  long_context?: {
    input_threshold: number
    input_multiplier: number
    output_multiplier: number
  }
  context_intervals?: Array<{
    min_tokens: number
    max_tokens?: number
    input_price: number | null
    output_price: number | null
    cache_write_price: number | null
    cache_read_price: number | null
  }>
  time_pricing?: {
    timezone: string
    weekdays_only?: boolean
    periods: Array<{ start_time: string; end_time: string; multiplier: number }>
  }
  disabled?: boolean
}

export interface PublicImageGenerationPricing {
  mode: 'fixed_per_image' | 'token'
  price_per_image?: number
  text_input_price?: number
  text_cached_input_price?: number
  image_input_price?: number
  image_cached_input_price?: number
  image_output_price?: number
}

export interface PublicPricingGroup {
  group_id: number
  name: string
  description?: string
  platform: string
  rate_multiplier: number
  is_exclusive: boolean
  subscription_type: string
  // New responses always return an array. Keep null in the runtime contract so
  // the public page remains compatible with an older cached response.
  models: PublicModelPrice[] | null
  image_generation?: PublicImageGenerationPricing
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
