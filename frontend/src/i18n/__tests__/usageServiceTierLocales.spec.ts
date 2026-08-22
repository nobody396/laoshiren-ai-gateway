import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('usage service tier locale keys', () => {
  it('contains zh labels for service tier tooltip', () => {
    expect(zh.usage.serviceTier).toBe('服务档位')
    expect(zh.usage.serviceTierPriority).toBe('Fast')
    expect(zh.usage.serviceTierFlex).toBe('Flex')
    expect(zh.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains en labels for service tier tooltip', () => {
    expect(en.usage.serviceTier).toBe('Service tier')
    expect(en.usage.serviceTierPriority).toBe('Fast')
    expect(en.usage.serviceTierFlex).toBe('Flex')
    expect(en.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains channel multiplier labels in both locales', () => {
    expect(zh.admin.channels.form.fastMultiplier).toBe('Fast / Priority 倍率')
    expect(zh.admin.channels.form.flexMultiplier).toBe('Flex 倍率')
    expect(en.admin.channels.form.fastMultiplier).toBe('Fast / Priority Multiplier')
    expect(en.admin.channels.form.flexMultiplier).toBe('Flex Multiplier')
  })
})
