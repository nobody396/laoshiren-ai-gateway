import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'

describe('API key action labels', () => {
  it('keeps setup distinct from CC Switch import', () => {
    expect(zh.keys.configureClient).toBe('一键配置')
    expect(zh.keys.importToCcSwitch).toBe('导入 CC Switch')
    expect(en.keys.configureClient).toBe('One-click Setup')
    expect(en.keys.importToCcSwitch).toBe('Import to CC Switch')
  })
})
