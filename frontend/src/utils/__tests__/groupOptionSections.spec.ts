import { describe, expect, it } from 'vitest'

import { buildGroupOptionSections, sortGroupOptionsByRate } from '@/utils/groupOptionSections'
import type { SubscriptionType } from '@/types'

type TestOption = {
  label: string
  rate: number
  userRate: number | null
  subscriptionType: SubscriptionType
}

const option = (
  label: string,
  rate: number,
  subscriptionType: SubscriptionType = 'standard',
  userRate: number | null = null
): TestOption => ({ label, rate, userRate, subscriptionType })

describe('group option sections', () => {
  it('hides monthly groups when the user has no active monthly card', () => {
    const sections = buildGroupOptionSections([
      option('Claude 月卡', 1, 'subscription'),
      option('按量 1', 0.5)
    ], false)

    expect(sections.map((section) => section.id)).toEqual(['payg'])
    expect(sections[0].options.map((item) => item.label)).toEqual(['按量 1'])
  })

  it('shows monthly groups before pay-as-you-go groups when available', () => {
    const sections = buildGroupOptionSections([
      option('按量 1', 1),
      option('Claude 月卡', 1, 'subscription'),
      option('GPT 月卡', 1, 'credit')
    ], true)

    expect(sections.map((section) => section.id)).toEqual(['monthly', 'payg'])
    expect(sections[0].options.map((item) => item.label)).toEqual(['Claude 月卡', 'GPT 月卡'])
  })

  it('sorts each section by the effective user rate in ascending order', () => {
    const sections = buildGroupOptionSections([
      option('默认 0.3', 0.3),
      option('专属 0.2', 1.5, 'standard', 0.2),
      option('默认 0.25', 0.25)
    ], false)

    expect(sections[0].options.map((item) => item.label)).toEqual([
      '专属 0.2',
      '默认 0.25',
      '默认 0.3'
    ])
  })

  it('uses the label as a stable tie-breaker for equal rates', () => {
    expect(sortGroupOptionsByRate([
      option('Beta', 1),
      option('Alpha', 1)
    ]).map((item) => item.label)).toEqual(['Alpha', 'Beta'])
  })
})
