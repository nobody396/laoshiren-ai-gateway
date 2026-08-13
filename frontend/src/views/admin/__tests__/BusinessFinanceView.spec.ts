import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BusinessFinanceView from '../BusinessFinanceView.vue'

const { financeSummary, costOverview, summary, getOverview, replace, route } = vi.hoisted(() => ({
  financeSummary: {
    total_income_fen: 337613,
    total_expense_fen: 185661,
    net_profit_fen: 151952,
    margin_percent: 45.01,
    by_category: [
      { type: 'expense', category: 'upstream_topup', total_fen: 100000, tx_count: 2 }
    ],
    by_payment_channel: [],
    monthly_series: [],
    range_from: '2026-08-01T00:00:00+08:00',
    range_to: '2026-08-13T10:00:00+08:00'
  },
  costOverview: {
    generated_at: '2026-08-13T10:00:00+08:00',
    shop_channel_fee_percent: 3,
    usage_window_start: '2026-08-01T00:00:00+08:00',
    usage_window_end: '2026-08-13T10:00:00+08:00',
    pricing_source_note: '',
    scope_note: '',
    legacy_monthly_card_group_count: 1,
    legacy_monthly_card_real_usage: { available: true, observed_request_count: 10, observed_real_cost_cny: 20 },
    monthly_cards: [{ real_usage: { observed_request_count: 20, observed_real_cost_cny: 30 } }],
    pay_as_you_go: [{ real_usage: { observed_request_count: 40, observed_real_cost_cny: 50 } }]
  },
  summary: vi.fn(),
  getOverview: vi.fn(),
  replace: vi.fn(),
  route: { query: {} as Record<string, string> }
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    financeTransactions: { summary },
    costAccounting: { getOverview }
  }
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => route,
    useRouter: () => ({ replace })
  }
})

const mountView = () => mount(BusinessFinanceView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Icon: true,
      RouterLink: { props: ['to'], template: '<a><slot /></a>' },
      FinanceTransactionsView: { template: '<div data-test="ledger-stub">ledger</div>' },
      CostAccountingView: { template: '<div data-test="cost-stub">cost</div>' }
    }
  }
})

beforeEach(() => {
  route.query = {}
  summary.mockReset().mockResolvedValue(financeSummary)
  getOverview.mockReset().mockResolvedValue(costOverview)
  replace.mockReset()
})

describe('admin BusinessFinanceView', () => {
  it('combines cash and usage cost into a conclusion-first overview', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(summary).toHaveBeenCalledWith(undefined, undefined, 'month')
    expect(getOverview).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('经营财务中心')
    expect(wrapper.text()).toContain('¥3,376.13')
    expect(wrapper.text()).toContain('¥1,519.52')
    expect(wrapper.text()).toContain('¥100.00')
    expect(wrapper.text()).toContain('70 次真实计费请求')
  })

  it('switches modules without leaving the unified page', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="business-tab-ledger"]').trigger('click')
    expect(wrapper.find('[data-test="business-ledger"]').exists()).toBe(true)
    expect(replace).toHaveBeenLastCalledWith({ query: { tab: 'ledger' } })

    await wrapper.get('[data-test="business-tab-cost"]').trigger('click')
    expect(wrapper.find('[data-test="business-cost"]').exists()).toBe(true)
    expect(replace).toHaveBeenLastCalledWith({ query: { tab: 'cost' } })
  })

  it('keeps the overview usable when one data source fails', async () => {
    getOverview.mockRejectedValueOnce(new Error('cost unavailable'))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('部分数据暂不可用：用量成本')
    expect(wrapper.text()).toContain('¥3,376.13')
  })
})
