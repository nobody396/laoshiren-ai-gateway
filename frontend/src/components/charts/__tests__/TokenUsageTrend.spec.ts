import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import TokenUsageTrend from '../TokenUsageTrend.vue'
import type { TrendDataPoint } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: '<div data-testid="line-chart" />'
  }
}))

const point: TrendDataPoint = {
  date: '2026-07-11 12:00',
  requests: 1,
  input_tokens: 100,
  output_tokens: 20,
  cache_creation_tokens: 10,
  cache_read_tokens: 5,
  total_tokens: 135,
  cost: 0.2,
  actual_cost: 0.1
}

describe('TokenUsageTrend', () => {
  it('renders a continuous today timeline when the server returns one bucket', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [point],
        startDate: '2026-07-11',
        granularity: 'hour'
      },
      global: {
        stubs: {
          LoadingSpinner: true
        }
      }
    })

    const chart = wrapper.findComponent({ name: 'Line' })
    expect(chart.exists()).toBe(true)
    const data = chart.props('data') as { labels: string[]; datasets: Array<{ data: number[] }> }
    expect(data.labels).toHaveLength(13)
    expect(data.labels[0]).toBe('2026-07-11 00:00')
    expect(data.labels.at(-1)).toBe('2026-07-11 12:00')
    expect(data.datasets[0]?.data.slice(0, 12)).toEqual(Array(12).fill(0))
    expect(data.datasets[0]?.data[12]).toBe(100)
  })
})
