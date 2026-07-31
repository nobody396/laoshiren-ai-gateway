import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../../client'
import {
  getInviteActivity,
  listAffiliateQualifiedCandidates,
  updateAffiliateSelfCommissionPolicy,
  updateInviteActivity
} from '../agents'

vi.mock('../../client', () => ({
  apiClient: {
    get: vi.fn(),
    put: vi.fn()
  }
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('Admin agents API', () => {
  it('updates one partner self-consumption policy with optimistic locking and a reason', async () => {
    mockClient.put.mockResolvedValue({
      data: {
        agent_id: 47,
        enabled: true,
        rate_bps: 1000,
        revision: 3,
        has_upstream: false,
        eligible: true
      }
    })

    const policy = await updateAffiliateSelfCommissionPolicy(47, {
      enabled: true,
      expected_revision: 2,
      reason: '无上级合伙人，批准本人消费返佣'
    })

    expect(mockClient.put).toHaveBeenCalledWith(
      '/admin/agents/47/self-commission-policy',
      {
        enabled: true,
        expected_revision: 2,
        reason: '无上级合伙人，批准本人消费返佣'
      }
    )
    expect(policy).toEqual(expect.objectContaining({
      agent_id: 47,
      enabled: true,
      rate_bps: 1000,
      revision: 3
    }))
  })

  it('lists users who qualified but have not applied', async () => {
    mockClient.get.mockResolvedValue({
      data: {
        items: [{
          user_id: 47,
          email: 'qualified@example.com',
          username: '',
          qualification_route: 'self_consumption',
          valid_direct_user_count: 0,
          self_consumption_micros: 530_000_000,
          direct_team_consumption_micros: 0,
          combined_consumption_micros: 530_000_000
        }]
      }
    })

    const items = await listAffiliateQualifiedCandidates(500)

    expect(mockClient.get).toHaveBeenCalledWith(
      '/admin/agents/affiliate-qualified-candidates',
      { params: { limit: 500 } }
    )
    expect(items).toEqual([
      expect.objectContaining({
        user_id: 47,
        qualification_route: 'self_consumption',
        self_consumption_micros: 530_000_000
      })
    ])
  })

  it('reads invite activity config from GET /admin/agents/rates', async () => {
    mockClient.get.mockResolvedValue({
      data: {
        consumption_rate: 0.06,
        first_recharge_invitee_rate: 0.1,
        first_recharge_referral_rate: 0.05,
        invite_activity: {
          enabled: true,
          name: '公测活动',
          start_at: '2026-05-20T00:00:00+08:00',
          end_at: '2026-05-21T00:00:00+08:00',
          registration_bonus_amount: 5,
          email_restriction_enabled: true,
          email_suffix_whitelist: ['@qq.com', '@gmail.com']
        }
      }
    })

    const activity = await getInviteActivity()

    expect(mockClient.get).toHaveBeenCalledWith('/admin/agents/rates')
    expect(activity).toEqual(expect.objectContaining({
      enabled: true,
      registration_bonus_amount: 5,
      email_restriction_enabled: true,
      email_suffix_whitelist: ['@qq.com', '@gmail.com']
    }))
    expect(activity).not.toHaveProperty('first_recharge_invitee_rate')
  })

  it('updates invite activity without dropping current global rates', async () => {
    mockClient.get.mockResolvedValue({
      data: {
        consumption_rate: 0.06,
        first_recharge_invitee_rate: 0.1,
        first_recharge_referral_rate: 0.05
      }
    })
    mockClient.put.mockResolvedValue({
      data: {
        invite_activity: {
          enabled: true,
          name: '公测活动',
          registration_bonus_amount: 5,
          email_restriction_enabled: true,
          email_suffix_whitelist: ['@qq.com']
        }
      }
    })

    await updateInviteActivity({
      enabled: true,
      name: '公测活动',
      start_at: '2026-05-20T00:00:00+08:00',
      end_at: '2026-05-21T00:00:00+08:00',
      registration_bonus_amount: 5,
      email_restriction_enabled: true,
      email_suffix_whitelist: ['@qq.com']
    })

    expect(mockClient.put).toHaveBeenCalledWith('/admin/agents/rates', {
      consumption_rate: 0.06,
      first_recharge_invitee_rate: 0.1,
      first_recharge_referral_rate: 0.05,
      invite_activity: expect.objectContaining({
        enabled: true,
        registration_bonus_amount: 5,
        email_restriction_enabled: true,
        email_suffix_whitelist: ['@qq.com']
      })
    })
    expect(mockClient.put.mock.calls[0][1].invite_activity).not.toHaveProperty('first_recharge_invitee_rate')
  })
})
