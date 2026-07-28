import { apiClient } from '../client'

export interface CostAccountingRate {
  group_id: number
  group_name: string
  group_rate_multiplier: number
  primary_account_rate_multiplier: number
  worst_account_rate_multiplier: number
  schedulable_account_count: number
}

export interface CostAccountingMoney {
  cost_cny: number
  profit_cny: number
  margin_percent: number
}

export interface CostAccountingScenario {
  cost_per_credit_worst_account: number
  vs_shop_net_price: CostAccountingMoney
  vs_direct_price: CostAccountingMoney
}

export interface CostAccountingRealUsage {
  available: boolean
  note?: string
  observed_request_count?: number
  observed_raw_credits_consumed?: number
  observed_real_cost_cny?: number
  product_mix_percent?: Record<string, number>
  blended_cost_per_credit?: number
  projected_full_quota_cost_cny?: number
  vs_shop_net_price?: CostAccountingMoney
  vs_direct_price?: CostAccountingMoney
}

export interface CostAccountingMarginRange {
  worst_percent: number
  best_percent: number
  conservative_percent: number
  real_percent?: number
}

export type CostAccountingProduct = 'gpt' | 'claude'

export interface CostAccountingMonthlyPlan {
  id: string
  name: string
  shop_price_cny: number
  shop_fee_percent: number
  shop_net_price_cny: number
  direct_price_cny: number
  monthly_credits: number
  products: Record<CostAccountingProduct, CostAccountingRate>
  single_product_scenarios: Record<string, CostAccountingScenario>
  real_usage: CostAccountingRealUsage
  best_case_scenario: CostAccountingMoney
  margin_range: CostAccountingMarginRange
}

export interface CostAccountingPayAsYouGoGroup {
  group_id: number
  group_name: string
  product: string
  platform: string
  group_rate_multiplier: number
  primary_account_rate_multiplier: number
  worst_account_rate_multiplier: number
  schedulable_account_count: number
  topup_100_cny_scenario?: CostAccountingMoney
  real_usage: CostAccountingRealUsage
  warning?: string
}

export interface CostAccountingOverview {
  generated_at: string
  shop_channel_fee_percent: number
  usage_window_start: string
  usage_window_end: string
  pricing_source_note: string
  scope_note: string
  legacy_monthly_card_group_count: number
  legacy_monthly_card_real_usage: CostAccountingRealUsage
  monthly_cards: CostAccountingMonthlyPlan[]
  pay_as_you_go: CostAccountingPayAsYouGoGroup[]
}

export async function getOverview(): Promise<CostAccountingOverview> {
  const { data } = await apiClient.get<CostAccountingOverview>('/admin/ops/cost-accounting')
  return data
}

const costAccountingAPI = { getOverview }
export default costAccountingAPI
