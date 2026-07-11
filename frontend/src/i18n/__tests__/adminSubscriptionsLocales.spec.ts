import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const requiredKeys = [
  'allPlatforms',
  'usageGuide',
  'guideTitle',
  'guideIntro',
  'guideCreateGroupTitle',
  'guideCreateGroupBody',
  'guideAssignTitle',
  'guideAssignBody',
  'guideOperateTitle',
  'guideOperateBody'
] as const

describe('admin subscription locale keys', () => {
  it.each([
    ['en', en.admin.subscriptions],
    ['zh', zh.admin.subscriptions]
  ])('defines every rendered guide key for %s', (_locale, messages) => {
    for (const key of requiredKeys) {
      expect(messages[key]).toBeTruthy()
      expect(messages[key]).not.toBe(`admin.subscriptions.${key}`)
    }
  })
})
