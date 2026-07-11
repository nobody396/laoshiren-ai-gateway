import { describe, expect, it } from 'vitest'
import { buildContinuousBucketLabels, fillUsageTrendBuckets } from '../trendBuckets'
import type { TrendDataPoint } from '@/types'

const point = (date: string, inputTokens = 10): TrendDataPoint => ({
  date,
  requests: 1,
  input_tokens: inputTokens,
  output_tokens: 2,
  cache_creation_tokens: 3,
  cache_read_tokens: 4,
  total_tokens: inputTokens + 9,
  cost: 0.2,
  actual_cost: 0.1
})

describe('trendBuckets', () => {
  it('fills hourly buckets from the selected day start through the latest observation', () => {
    const labels = buildContinuousBucketLabels(
      '2026-07-11',
      'hour',
      ['2026-07-11 12:00']
    )

    expect(labels).toHaveLength(13)
    expect(labels[0]).toBe('2026-07-11 00:00')
    expect(labels.at(-1)).toBe('2026-07-11 12:00')
  })

  it('preserves real values while inserting zero-valued gaps', () => {
    const filled = fillUsageTrendBuckets(
      [point('2026-07-11 02:00', 25)],
      '2026-07-11',
      'hour'
    )

    expect(filled.map((item) => item.date)).toEqual([
      '2026-07-11 00:00',
      '2026-07-11 01:00',
      '2026-07-11 02:00'
    ])
    expect(filled[0]?.total_tokens).toBe(0)
    expect(filled[2]?.input_tokens).toBe(25)
    expect(filled[2]?.actual_cost).toBe(0.1)
  })

  it('falls back to observed labels when the server format is unknown', () => {
    expect(buildContinuousBucketLabels('2026-07-11', 'hour', ['custom-bucket'])).toEqual([
      'custom-bucket'
    ])
  })
})
