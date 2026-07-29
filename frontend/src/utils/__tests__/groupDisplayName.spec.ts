import { describe, expect, it } from 'vitest'
import { publicGroupDisplayName } from '../groupDisplayName'

describe('publicGroupDisplayName', () => {
  it('removes internal monthly-card version markers', () => {
    expect(publicGroupDisplayName('GPT Pro V3 月卡组')).toBe('GPT Pro 月卡组')
    expect(publicGroupDisplayName('Claude Max V12 月卡组')).toBe('Claude Max 月卡组')
  })

  it('does not change ordinary or non-monthly group names', () => {
    expect(publicGroupDisplayName('GPT Plus 月卡组')).toBe('GPT Plus 月卡组')
    expect(publicGroupDisplayName('Codex V3 测试分组')).toBe('Codex V3 测试分组')
  })
})
