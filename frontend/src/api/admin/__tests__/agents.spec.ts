import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../../client'
import { getInviteActivity, updateInviteActivity } from '../agents'

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
          registration_bonus_amount: 5
        }
      }
    })

    const activity = await getInviteActivity()

    expect(mockClient.get).toHaveBeenCalledWith('/admin/agents/rates')
    expect(activity).toEqual(expect.objectContaining({
      enabled: true,
      registration_bonus_amount: 5
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
          registration_bonus_amount: 5
        }
      }
    })

    await updateInviteActivity({
      enabled: true,
      name: '公测活动',
      start_at: '2026-05-20T00:00:00+08:00',
      end_at: '2026-05-21T00:00:00+08:00',
      registration_bonus_amount: 5
    })

    expect(mockClient.put).toHaveBeenCalledWith('/admin/agents/rates', {
      consumption_rate: 0.06,
      first_recharge_invitee_rate: 0.1,
      first_recharge_referral_rate: 0.05,
      invite_activity: expect.objectContaining({
        enabled: true,
        registration_bonus_amount: 5
      })
    })
    expect(mockClient.put.mock.calls[0][1].invite_activity).not.toHaveProperty('first_recharge_invitee_rate')
  })
})
