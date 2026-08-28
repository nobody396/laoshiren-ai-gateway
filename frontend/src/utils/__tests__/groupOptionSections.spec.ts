import { describe, expect, it } from 'vitest'

import {
  buildGroupOptionFamilies,
  buildGroupOptionSections,
  classifyGroupOptionFamily,
  sortGroupOptionsByRate,
  type GroupOptionFamilyId
} from '@/utils/groupOptionSections'
import type { GroupPlatform, SubscriptionType } from '@/types'

type TestOption = {
  label: string
  rate: number
  userRate: number | null
  subscriptionType: SubscriptionType
  platform: GroupPlatform
  familyKey: GroupOptionFamilyId
}

const option = (
  label: string,
  rate: number,
  subscriptionType: SubscriptionType = 'standard',
  userRate: number | null = null,
  platform: GroupPlatform = 'openai',
  familyKey: GroupOptionFamilyId = 'openai'
): TestOption => ({ label, rate, userRate, subscriptionType, platform, familyKey })

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

    expect(sections.map((section) => section.id)).toEqual(['payg', 'monthly'])
    expect(sections[1].options.map((item) => item.label)).toEqual(['Claude 月卡', 'GPT 月卡'])
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

  it('classifies text providers before domestic models and image generation', () => {
    expect(classifyGroupOptionFamily({ label: 'CodeX Pro 20X 分组', platform: 'openai' })).toBe('openai')
    expect(classifyGroupOptionFamily({ label: 'Claude 官转分组', platform: 'anthropic' })).toBe('claude')
    expect(classifyGroupOptionFamily({ label: 'Grok 分组', platform: 'grok' })).toBe('grok')
    expect(classifyGroupOptionFamily({ label: 'Gemini 分组', platform: 'gemini' })).toBe('gemini')
    expect(classifyGroupOptionFamily({ label: 'GLM（阿里云）', platform: 'openai' })).toBe('domestic')
    expect(classifyGroupOptionFamily({ label: 'GPT Image 2 生图分组', platform: 'openai' })).toBe('image')
  })

  it('orders families and sorts prices from low to high inside each family', () => {
    const families = buildGroupOptionFamilies([
      option('国产 0.95', 0.95, 'standard', null, 'openai', 'domestic'),
      option('OpenAI 1.0', 1, 'standard', null, 'openai', 'openai'),
      option('OpenAI 0.35', 0.35, 'standard', null, 'openai', 'openai'),
      option('Claude 2.4', 2.4, 'standard', null, 'anthropic', 'claude'),
      option('生图 4.0', 4, 'standard', null, 'openai', 'image')
    ])

    expect(families.map((family) => family.id)).toEqual(['openai', 'claude', 'domestic', 'image'])
    expect(families[0].options.map((item) => item.label)).toEqual(['OpenAI 0.35', 'OpenAI 1.0'])
  })
})
