import { describe, expect, it } from 'vitest'
import { getPartnerAccessCopy, resolvePartnerAccessState } from '../partnerAccess'

describe('partnerAccess', () => {
  it('keeps an approved clear partner fully available', () => {
    const state = resolvePartnerAccessState('active', 'clear')
    expect(state).toBe('available')
    expect(getPartnerAccessCopy(state).badge).toBe('合伙人已开通')
  })

  it('describes review state in plain Chinese', () => {
    const state = resolvePartnerAccessState('active', 'review')
    const copy = getPartnerAccessCopy(state)

    expect(state).toBe('under_review')
    expect(copy.badge).toBe('待审核')
    expect(copy.banner).toContain('合伙人权限正在审核中')
    expect(copy.inviteLabel).toBe('邀请链接暂不可用')
  })

  it('describes blocked state in plain Chinese', () => {
    const state = resolvePartnerAccessState('active', 'blocked')
    const copy = getPartnerAccessCopy(state)

    expect(state).toBe('suspended')
    expect(copy.badge).toBe('已暂停')
    expect(copy.banner).toBe('你的合伙人权限已暂停。有疑问请联系客服。')
  })

  it('keeps ordinary users on the ordinary invite flow', () => {
    const state = resolvePartnerAccessState('qualified', 'clear')
    expect(state).toBe('not_partner')
    expect(getPartnerAccessCopy(state).inviteLabel).toBe('我的普通邀请链接')
  })
})
