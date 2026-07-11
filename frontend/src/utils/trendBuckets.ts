import type { TrendDataPoint } from '@/types'

export type TrendGranularity = 'day' | 'hour'

const MAX_BUCKETS = 1000
const DAY_MS = 24 * 60 * 60 * 1000
const HOUR_MS = 60 * 60 * 1000

function parseBucket(label: string, granularity: TrendGranularity): number | null {
  const pattern = granularity === 'hour'
    ? /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):00$/
    : /^(\d{4})-(\d{2})-(\d{2})$/
  const match = pattern.exec(label)
  if (!match) return null

  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hour = granularity === 'hour' ? Number(match[4]) : 0
  const value = Date.UTC(year, month - 1, day, hour)
  const date = new Date(value)

  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day ||
    date.getUTCHours() !== hour
  ) {
    return null
  }
  return value
}

function formatBucket(value: number, granularity: TrendGranularity): string {
  const date = new Date(value)
  const base = [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, '0'),
    String(date.getUTCDate()).padStart(2, '0')
  ].join('-')
  if (granularity === 'day') return base
  return `${base} ${String(date.getUTCHours()).padStart(2, '0')}:00`
}

/**
 * Expands sparse server trend labels from the selected range start through the
 * latest observed bucket. This makes a one-bucket "today" response render as a
 * real time series without inventing future buckets.
 */
export function buildContinuousBucketLabels(
  startDate: string,
  granularity: TrendGranularity,
  observedLabels: string[]
): string[] {
  const observed = [...new Set(observedLabels)].sort()
  if (observed.length === 0) return []

  const parsedObserved = observed.map((label) => parseBucket(label, granularity))
  if (parsedObserved.some((value) => value === null)) return observed

  const startLabel = granularity === 'hour' ? `${startDate} 00:00` : startDate
  const start = parseBucket(startLabel, granularity)
  const end = parsedObserved[parsedObserved.length - 1] ?? null
  if (start === null || end === null || end < start) return observed

  const step = granularity === 'hour' ? HOUR_MS : DAY_MS
  const bucketCount = Math.floor((end - start) / step) + 1
  if (bucketCount <= 0 || bucketCount > MAX_BUCKETS) return observed

  return Array.from({ length: bucketCount }, (_, index) =>
    formatBucket(start + index * step, granularity)
  )
}

export function fillUsageTrendBuckets(
  points: TrendDataPoint[],
  startDate: string | undefined,
  granularity: TrendGranularity | undefined
): TrendDataPoint[] {
  if (!startDate || !granularity || points.length === 0) return points

  const byDate = new Map(points.map((point) => [point.date, point]))
  return buildContinuousBucketLabels(
    startDate,
    granularity,
    points.map((point) => point.date)
  ).map((date) => byDate.get(date) ?? {
    date,
    requests: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: 0,
    cost: 0,
    actual_cost: 0
  })
}
