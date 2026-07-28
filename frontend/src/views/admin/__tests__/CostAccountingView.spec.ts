import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CostAccountingView from '../CostAccountingView.vue'

const { getOverview, summary, showError } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  summary: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    costAccounting: { getOverview },
    financeTransactions: { summary }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

const usage = {
  available: true,
  observed_request_count: 10,
  observed_raw_credits_consumed: 20,
  observed_real_cost_cny: 12.5,
  product_mix_percent: { gpt: 60, claude: 40 },
  blended_cost_per_credit: 0.625,
  projected_full_quota_cost_cny: 281.25,
  vs_shop_net_price: { cost_cny: 281.25, profit_cny: 37.88, margin_percent: 11.87 },
  vs_direct_price: { cost_cny: 281.25, profit_cny: 37.75, margin_percent: 11.83 }
}

function plan(id: 'plus' | 'pro' | 'max', name: string) {
  const product = (groupId: number, groupName: string) => ({
    group_id: groupId,
    group_name: groupName,
    group_rate_multiplier: 2,
    primary_account_rate_multiplier: 0.5,
    worst_account_rate_multiplier: 0.7,
    schedulable_account_count: 2
  })
  const scenario = {
    cost_per_credit_worst_account: 0.35,
    vs_shop_net_price: { cost_cny: 157.5, profit_cny: 151.93, margin_percent: 49.1 },
    vs_direct_price: { cost_cny: 157.5, profit_cny: 161.5, margin_percent: 50.63 }
  }
  return {
    id,
    name,
    shop_price_cny: id === 'plus' ? 259 : id === 'pro' ? 729 : 1549,
    shop_fee_percent: 3,
    shop_net_price_cny: id === 'plus' ? 251.23 : id === 'pro' ? 707.13 : 1502.53,
    direct_price_cny: id === 'plus' ? 249 : id === 'pro' ? 699 : 1499,
    monthly_credits: id === 'plus' ? 220 : id === 'pro' ? 650 : 1400,
    products: {
      gpt: product(id === 'plus' ? 7 : id === 'pro' ? 8 : 9, `${name} GPT`),
      claude: product(id === 'plus' ? 11 : id === 'pro' ? 12 : 13, `${name} Claude`)
    },
    single_product_scenarios: {
      all_gpt: scenario,
      all_claude: scenario
    },
    real_usage: usage,
    best_case_scenario: { cost_cny: 112.5, profit_cny: 206.5, margin_percent: 64.73 },
    margin_range: {
      worst_percent: 50,
      best_percent: 64.73,
      conservative_percent: 50,
      real_percent: 11.83
    }
  }
}

const overview = {
  generated_at: '2026-07-25T12:00:00+08:00',
  shop_channel_fee_percent: 3,
  usage_window_start: '2026-07-01T00:00:00+08:00',
  usage_window_end: '2026-07-25T12:00:00+08:00',
  pricing_source_note: 'test',
  scope_note: '当前月卡只核算在售 Plus/Pro/Max。',
  legacy_monthly_card_group_count: 9,
  legacy_monthly_card_real_usage: { ...usage, observed_real_cost_cny: 2 },
  monthly_cards: [plan('plus', 'Plus'), plan('pro', 'Pro'), plan('max', 'Max')],
  pay_as_you_go: [
    {
      group_id: 5,
      group_name: 'MAX 20X 分组',
      product: 'claude',
      platform: 'anthropic',
      group_rate_multiplier: 1.6,
      primary_account_rate_multiplier: 0.4,
      worst_account_rate_multiplier: 0.6,
      schedulable_account_count: 2,
      topup_100_cny_scenario: { cost_cny: 37.5, profit_cny: 59.5, margin_percent: 61.34 },
      real_usage: { ...usage, observed_real_cost_cny: 5 }
    },
    {
      group_id: 21,
      group_name: 'Claude AWS Bedrock 分组',
      product: 'claude',
      platform: 'anthropic',
      group_rate_multiplier: 3,
      primary_account_rate_multiplier: 1,
      worst_account_rate_multiplier: 1,
      schedulable_account_count: 1,
      topup_100_cny_scenario: { cost_cny: 33.33, profit_cny: 63.67, margin_percent: 65.64 },
      real_usage: { ...usage, observed_real_cost_cny: 3 }
    }
  ]
}

const financeSummary = {
  range_from: '2026-07-01T00:00:00+08:00',
  range_to: '2026-07-25T12:00:00+08:00',
  total_income_fen: 100000,
  total_expense_fen: 40000,
  net_profit_fen: 60000,
  margin_percent: 60,
  by_category: [
    { type: 'expense', category: 'upstream_topup', total_fen: 20000, tx_count: 2 }
  ],
  by_payment_channel: [],
  monthly_series: []
}

const mountView = () =>
  mount(CostAccountingView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
        Icon: true
      }
    }
  })

beforeEach(() => {
  getOverview.mockReset()
  summary.mockReset()
  showError.mockReset()
  getOverview.mockResolvedValue(overview)
  summary.mockResolvedValue(financeSummary)
})

describe('admin CostAccountingView', () => {
  it('loads cost data and links it to the same Beijing usage window in the finance ledger', async () => {
    mountView()
    await flushPromises()

    expect(getOverview).toHaveBeenCalledTimes(1)
    expect(summary).toHaveBeenCalledWith(
      Math.floor(new Date(overview.usage_window_start).getTime() / 1000),
      Math.floor(new Date(overview.usage_window_end).getTime() / 1000),
      'month'
    )
  })

  it('renders the current Plus/Pro/Max catalog and every pay-as-you-go group returned by the backend', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="plan-plus"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="plan-pro"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="plan-max"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test^="paygo-"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('Claude AWS Bedrock 分组')
  })

  it('shows cash top-ups separately from observed upstream consumption', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('其中上游充值')
    expect(wrapper.text()).toContain('¥200.00')
    expect(wrapper.text()).toContain('本月实际上游成本')
    expect(wrapper.text()).toContain('¥47.50')
  })
})
