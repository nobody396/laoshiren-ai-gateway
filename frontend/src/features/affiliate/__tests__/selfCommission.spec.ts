import { describe, expect, it } from 'vitest'
import type { AffiliateRiskPrincipal } from '@/api/admin/agents'
import type { CommissionRecord } from '@/api/agent'
import {
  getCommissionDisplayType,
  getSelfCommissionPresentation,
  isSelfConsumptionCommission
} from '../selfCommission'

function principal(overrides: Partial<AffiliateRiskPrincipal> = {}): AffiliateRiskPrincipal {
  return {
    agent_id: 8,
    email: 'partner@example.com',
    username: 'partner',
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

function commission(overrides: Partial<CommissionRecord> = {}): CommissionRecord {
  return {
    id: 1,
    beneficiary_id: 8,
    user_id: 8,
    amount: 1,
    source_amount: 10,
    type: 'consumption_commission',
    created_at: '2026-07-30T08:00:00Z',
    ...overrides
  }
}

describe('self commission presentation', () => {
  it('shows an enabled fixed-rate policy as applying only to future paid purchases', () => {
    const result = getSelfCommissionPresentation(principal({
      self_commission_enabled: true,
      self_commission_effective_at: '2026-07-30T08:00:00Z',
      self_commission_revision: 2
    }))

    expect(result.label).toBe('已开启 · 10%')
    expect(result.detail).toContain('开启后新购买')
    expect(result.tone).toBe('enabled')
  })

  it('explains in Chinese why a partner with an upstream cannot enable it', () => {
    const result = getSelfCommissionPresentation(principal({
      has_upstream: true,
      self_commission_eligible: false,
      self_commission_block_reason: 'has_upstream'
    }))

    expect(result.label).toBe('不可开启')
    expect(result.detail).toBe('已有上级，不能开启本人消费返佣')
    expect(result.canEnable).toBe(false)
  })
})

describe('self commission record labels', () => {
  it('derives self consumption only for consumption rows paid to the same user', () => {
    const item = commission()
    expect(isSelfConsumptionCommission(item)).toBe(true)
    expect(getCommissionDisplayType(item)).toBe('self_consumption_commission')
  })

  it('does not relabel ordinary invitee rewards where beneficiary and user match', () => {
    const item = commission({ type: 'first_recharge_invitee_bonus' })
    expect(isSelfConsumptionCommission(item)).toBe(false)
    expect(getCommissionDisplayType(item)).toBe('first_recharge_invitee_bonus')
  })

  it('keeps downstream consumption rows labeled as ordinary long-term commission', () => {
    const item = commission({ user_id: 19 })
    expect(isSelfConsumptionCommission(item)).toBe(false)
    expect(getCommissionDisplayType(item)).toBe('consumption_commission')
  })
})
