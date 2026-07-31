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
  getAffiliateWithdrawalQRCode: vi.fn(),
  getPaymentQRCode: vi.fn(),
  listAffiliateRiskPrincipals: vi.fn(),
  listAffiliateApplications: vi.fn(),
  listAffiliateQualifiedCandidates: vi.fn(),
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
