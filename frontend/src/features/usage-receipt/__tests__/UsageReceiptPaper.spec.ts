import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import UsageReceiptPaper from '../UsageReceiptPaper.vue'
import type { UsageReceiptData, UsageReceiptPreferences } from '../types'

const data: UsageReceiptData = {
  receiptNumber: 'LSR-20260813-ABC123',
  generatedAt: new Date('2026-08-13T02:15:00.000Z'),
  startDate: '2026-08-01',
  endDate: '2026-08-12',
  displayName: 'builder',
  inviteCode: 'HELLOAI',
  inviteUrl: 'https://laoshirenai.com/register?ref=HELLOAI',
  qrDataUrl: 'data:image/png;base64,QR',
  exchangeRate: 7,
  stats: {
    total_requests: 12,
    total_input_tokens: 80_000,
    total_output_tokens: 20_000,
    total_cache_tokens: 40_000,
    total_tokens: 140_000,
    total_cost: 20,
    total_actual_cost: 12.5,
    average_duration_ms: 1_250
  },
  models: [
    {
      model: 'anthropic/claude-sonnet-4-20250514',
      requests: 12,
      input_tokens: 80_000,
      output_tokens: 20_000,
      cache_creation_tokens: 0,
      cache_read_tokens: 40_000,
      total_tokens: 140_000,
      cost: 20,
      actual_cost: 12.5,
      account_cost: 10
    }
  ]
}

const preferences: UsageReceiptPreferences = {
  showDisplayName: false,
  showSavings: true,
  showModelBreakdown: true
}

describe('UsageReceiptPaper', () => {
  it('renders a branded, privacy-safe receipt with the correct totals', () => {
    const wrapper = mount(UsageReceiptPaper, {
      props: {
        data,
        preferences,
        siteName: '老实人AI',
        siteLogo: '/laoshirenai-icon.jpg'
      }
    })

    const text = wrapper.text()
    expect(wrapper.get('img.receipt-brand__logo').attributes('src')).toBe('/laoshirenai-icon.jpg')
    expect(text).toContain('老实人AI')
    expect(text).not.toContain('AI 使用小票')
    expect(text).not.toContain('AI USAGE RECEIPT')
    expect(text).not.toContain('仅展示汇总数据')
    expect(text).toContain('¥12.50')
    expect(text).toContain('140K')
    expect(text).toContain('Claude Sonnet 4')
    expect(text).toContain('¥140.00')
    expect(text).toContain('¥127.50')
    expect(text).toContain('邀请码 HELLOAI')
    expect(text).not.toContain('@builder')
    expect(text).not.toContain('builder@example.com')
    expect(text).not.toContain('sk-')
    expect(wrapper.get('.receipt-qr img').attributes('src')).toBe('data:image/png;base64,QR')
  })

  it('only includes the optional display name after the user enables it', () => {
    const wrapper = mount(UsageReceiptPaper, {
      props: {
        data,
        preferences: { ...preferences, showDisplayName: true },
        siteName: '老实人AI',
        siteLogo: '/laoshirenai-icon.jpg'
      }
    })

    expect(wrapper.text()).toContain('@builder')
  })
})
