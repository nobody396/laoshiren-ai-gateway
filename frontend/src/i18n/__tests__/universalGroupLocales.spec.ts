import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('universal group locale keys', () => {
  it('provides route editor labels in both locales', () => {
    expect(zh.admin.groups.platforms.universal).toBe('通用分组')
    expect(zh.admin.groups.universal.routes).toContain('通用模型路由')
    expect(en.admin.groups.platforms.universal).toBe('Universal')
    expect(en.admin.groups.universal.routes).toContain('Universal model routes')
  })
})
