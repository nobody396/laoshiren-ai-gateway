import { describe, expect, it } from 'vitest'

import type { ModelStat } from '@/types'
import {
  buildInviteUrl,
  buildReceiptModelLines,
  createReceiptNumber,
  formatCompactNumber,
  formatModelName,
  formatReceiptDateRange,
  receiptImageFilename
} from '../receipt'

function model(overrides: Partial<ModelStat> & Pick<ModelStat, 'model'>): ModelStat {
  return {
    requests: 1,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: 1_000,
    cost: 0,
    actual_cost: 1,
    account_cost: 0,
    ...overrides
  }
}

describe('usage receipt helpers', () => {
  it('formats large token counts without noisy trailing zeroes', () => {
    expect(formatCompactNumber(999)).toBe('999')
    expect(formatCompactNumber(1_200)).toBe('1.2K')
    expect(formatCompactNumber(12_000_000)).toBe('12M')
    expect(formatCompactNumber(Number.NaN)).toBe('0')
  })

  it('normalizes provider-prefixed and dated model names', () => {
    expect(formatModelName('anthropic/claude-sonnet-4-20250514')).toBe('Claude Sonnet 4')
    expect(formatModelName('openai/gpt-5.4')).toBe('GPT 5.4')
    expect(formatModelName('   ')).toBe('未知模型')
  })

  it('keeps the most expensive models and aggregates the rest', () => {
    const lines = buildReceiptModelLines([
      model({ model: 'model-a', actual_cost: 4 }),
      model({ model: 'model-b', actual_cost: 2, requests: 2 }),
      model({ model: 'model-c', actual_cost: 1, requests: 3, total_tokens: 6_000 })
    ], 2)

    expect(lines).toEqual([
      { model: 'Model A', requests: 1, totalTokens: 1_000, actualCost: 4 },
      { model: 'Model B', requests: 2, totalTokens: 1_000, actualCost: 2 },
      { model: '其他 1 个模型', requests: 3, totalTokens: 6_000, actualCost: 1 }
    ])
  })

  it('builds safe invite URLs and stable receipt labels', () => {
    expect(buildInviteUrl('https://laoshirenai.com/', 'A B+C')).toBe(
      'https://laoshirenai.com/register?ref=A%20B%2BC'
    )
    expect(buildInviteUrl('https://laoshirenai.com', '')).toBe('https://laoshirenai.com/register')
    expect(formatReceiptDateRange('2026-08-01', '2026-08-01')).toBe('2026-08-01')
    expect(receiptImageFilename('2026-08-01', '2026-08-12')).toBe(
      '老实人AI-使用小票-2026-08-01-2026-08-12.png'
    )
  })

  it('uses the Beijing calendar date in the receipt number', () => {
    expect(createReceiptNumber(new Date('2026-08-12T16:30:00.000Z'))).toMatch(
      /^LSR-20260813-[0-9A-F]{6}$/
    )
  })
})
