import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AffiliateOperationsView from '../AffiliateOperationsView.vue'

const api = vi.hoisted(() => ({
  completeAffiliateWithdrawal: vi.fn(),
  failAffiliateWithdrawal: vi.fn(),
  getAffiliateCommunity: vi.fn(),
  getAffiliateCommunityQRCode: vi.fn(),
  getAffiliateCommercialPolicy: vi.fn(),
  getAffiliateProgram: vi.fn(),
  getAffiliateOperationsSummary: vi.fn(),
  getAffiliatePartnerPerformance: vi.fn(),
  getAffiliateWithdrawalQRCode: vi.fn(),
  getPaymentQRCode: vi.fn(),
  listAffiliateRiskPrincipals: vi.fn(),
  listAffiliateApplications: vi.fn(),
  listAffiliateQualifiedCandidates: vi.fn(),
  listAffiliatePartnerPerformance: vi.fn(),
  listAffiliateWithdrawals: vi.fn(),
  listPendingPaymentProfiles: vi.fn(),
  reverseAffiliatePerformance: vi.fn(),
  reviewPaymentProfile: vi.fn(),
  reviewAffiliateApplication: vi.fn(),
  updateAffiliateRisk: vi.fn(),
  updateAffiliateSelfCommissionPolicy: vi.fn(),
  updateAffiliateCommunity: vi.fn(),
  updateAffiliateProgram: vi.fn(),
  uploadAffiliateCommunityQRCode: vi.fn()
}))

const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin/agents', () => ({ ...api, default: api }))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

function principal(overrides: Record<string, unknown> = {}) {
  return {
    agent_id: 8,
    email: 'partner@example.com',
    username: '合伙人甲',
    agent_status: 'active',
    risk_status: 'clear',
    risk_note: '',
    held_reward_count: 0,
    held_reward_micros: 0,
    held_cash_count: 0,
    held_cash_micros: 0,
    updated_at: '2026-07-30T08:00:00Z',
    has_upstream: false,
    self_commission_enabled: false,
    self_commission_rate_bps: 1000,
    self_commission_revision: 0,
    self_commission_eligible: true,
    ...overrides
  }
}

function mountView() {
  return mount(AffiliateOperationsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' }
      }
    }
  })
}

async function openPartnersTab(wrapper: ReturnType<typeof mountView>) {
  await flushPromises()
  const button = wrapper.findAll('button').find(item => item.text().includes('合伙人管理'))
  expect(button).toBeDefined()
  await button!.trigger('click')
}

beforeEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
  api.getAffiliateProgram.mockResolvedValue(null)
  api.getAffiliateCommercialPolicy.mockResolvedValue(null)
  api.getAffiliateCommunity.mockResolvedValue({
    enabled: false,
    title: '',
    message: '',
    qr_size: 0,
    has_qr_code: false,
    revision: 0
  })
  api.getAffiliateOperationsSummary.mockResolvedValue({
    qualified_followup: 0,
    pending_applications: 0,
    pending_payment_profiles: 0,
    processing_withdrawals: 0,
    overdue_withdrawals: 0,
    abnormal_partners: 0,
    actionable_total: 0
  })
  api.listAffiliatePartnerPerformance.mockResolvedValue([])
  api.listAffiliateQualifiedCandidates.mockResolvedValue([])
  api.listAffiliateApplications.mockResolvedValue([])
  api.listPendingPaymentProfiles.mockResolvedValue([])
  api.listAffiliateWithdrawals.mockResolvedValue([])
  api.listAffiliateRiskPrincipals.mockResolvedValue([principal()])
})

describe('AffiliateOperationsView self-consumption policy controls', () => {
  it('requires a reason, saves one partner, and renders the refreshed audit details', async () => {
    api.updateAffiliateSelfCommissionPolicy.mockResolvedValue({
      agent_id: 8,
      enabled: true,
      rate_bps: 1000,
      effective_at: '2026-07-30T09:00:00Z',
      revision: 1,
      updated_by: 3,
      reason: '批准无上级合伙人本人返佣',
      updated_at: '2026-07-30T09:00:00Z',
      has_upstream: false,
      eligible: true
    })
    api.listAffiliateRiskPrincipals
      .mockResolvedValueOnce([principal()])
      .mockResolvedValueOnce([principal({
        self_commission_enabled: true,
        self_commission_effective_at: '2026-07-30T09:00:00Z',
        self_commission_revision: 1,
        self_commission_reason: '批准无上级合伙人本人返佣',
        self_commission_updated_by: 3,
        self_commission_updated_by_username: '运营员',
        self_commission_updated_by_email: 'ops@example.com',
        self_commission_updated_at: '2026-07-30T09:00:00Z'
      })])

    const wrapper = mountView()
    await openPartnersTab(wrapper)

    const openButton = wrapper.findAll('button').find(item => item.text() === '开启')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')

    const submit = wrapper.get('[role="dialog"] button.btn-primary')
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('[role="dialog"] textarea').setValue('批准无上级合伙人本人返佣')
    await wrapper.get('[role="dialog"] form').trigger('submit')
    await flushPromises()

    expect(api.updateAffiliateSelfCommissionPolicy).toHaveBeenCalledWith(8, {
      enabled: true,
      expected_revision: 0,
      reason: '批准无上级合伙人本人返佣'
    })
    expect(wrapper.text()).toContain('已开启 · 10%')
    expect(wrapper.text()).toContain('最近原因：批准无上级合伙人本人返佣')
    expect(wrapper.text()).toContain('运营员（ops@example.com）')
    expect(showSuccess).toHaveBeenCalledWith('本人消费返佣已开启')
  })

  it('shows a Chinese revision-conflict message and refreshes the partner row', async () => {
    api.updateAffiliateSelfCommissionPolicy.mockRejectedValue({
      status: 409,
      code: 'AFFILIATE_SELF_COMMISSION_POLICY_REVISION_CONFLICT',
      message: 'self-commission policy changed; reload and retry'
    })
    api.listAffiliateRiskPrincipals
      .mockResolvedValueOnce([principal()])
      .mockResolvedValueOnce([principal({ self_commission_revision: 2 })])

    const wrapper = mountView()
    await openPartnersTab(wrapper)
    const openButton = wrapper.findAll('button').find(item => item.text() === '开启')
    await openButton!.trigger('click')
    await wrapper.get('[role="dialog"] textarea').setValue('批准开通')
    await wrapper.get('[role="dialog"] form').trigger('submit')
    await flushPromises()

    expect(api.listAffiliateRiskPrincipals).toHaveBeenCalledTimes(2)
    expect(showError).toHaveBeenCalledWith('设置已被其他管理员修改，页面已刷新，请重新确认后再操作。')
    expect(showError).not.toHaveBeenCalledWith(expect.stringContaining('self-commission policy changed'))
  })
})

describe('AffiliateOperationsView actionable queues and performance', () => {
  it('shows only non-zero queue badges and does not present partner totals as pending work', async () => {
    api.getAffiliateOperationsSummary.mockResolvedValue({
      qualified_followup: 3,
      pending_applications: 2,
      pending_payment_profiles: 0,
      processing_withdrawals: 1,
      overdue_withdrawals: 1,
      abnormal_partners: 0,
      actionable_total: 3
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('待处理 3')
    expect(wrapper.text()).toContain('已达标待申请 3')
    expect(wrapper.text()).toContain('合伙人申请 2')
    expect(wrapper.text()).toContain('提现打款（逾期 1） 1')
    const partnerTab = wrapper.findAll('button').find(item => item.text() === '合伙人管理')
    expect(partnerTab).toBeDefined()
  })

  it('opens a period-filterable partner performance detail with direct-user metrics', async () => {
    api.listAffiliatePartnerPerformance.mockResolvedValue([{
      agent_id: 8,
      email: 'partner@example.com',
      username: '合伙人甲',
      activated_at: '2026-07-01T00:00:00Z',
      direct_user_count: 2,
      paid_direct_user_count: 1,
      self_recharge_micros: 100000000,
      direct_team_recharge_micros: 200000000,
      self_consumption_micros: 50000000,
      direct_team_consumption_micros: 120000000,
      recent_30d_consumption_micros: 170000000,
      lifetime_earned_micros: 12000000,
      available_commission_micros: 7000000,
      processing_withdrawal_micros: 2000000,
      paid_commission_micros: 3000000
    }])
    api.getAffiliatePartnerPerformance.mockResolvedValue({
      summary: (await api.listAffiliatePartnerPerformance())[0],
      period_start: '2026-07-01T00:00:00Z',
      period_end: '2026-07-31T00:00:00Z',
      direct_users: [{
        user_id: 9, email: 'user@example.com', username: '用户乙', joined_at: '2026-07-02T00:00:00Z',
        recharge_micros: 200000000, consumption_micros: 120000000, generated_commission_micros: 12000000
      }],
      commission_ledger: [{
        id: 92,
        consumer_user_id: 9,
        entry_type: 'earned',
        posting_status: 'posted',
        amount_micros: 901,
        source_amount_micros: 18025,
        customer_rebate_rate_bps: 500,
        agent_commission_rate_bps: 500,
        source_type: 'confirmed_consumption',
        occurred_at: '2026-07-02T00:00:00Z'
      }],
      withdrawals: []
    })
    const wrapper = mountView()
    await openPartnersTab(wrapper)
    const detailButton = wrapper.findAll('button').find(item => item.text() === '查看明细')
    expect(detailButton).toBeDefined()
    await detailButton!.trigger('click')
    await flushPromises()

    expect(api.getAffiliatePartnerPerformance).toHaveBeenCalledWith(8, undefined)
    const detail = document.body.querySelector('[data-testid="affiliate-performance-detail"]')
    expect(detail).not.toBeNull()
    expect(detail?.classList.contains('z-[60]')).toBe(true)
    expect(detail?.textContent).toContain('直属用户业绩')
    expect(detail?.textContent).toContain('用户乙')
    expect(detail?.textContent).toContain('¥200')
    expect(detail?.textContent).toContain('¥0.000901')
    expect(detail?.textContent).toContain('对应消费 ¥0.018025 · 用户返利 5% · 合伙人 5%')
    wrapper.unmount()
  })
})
