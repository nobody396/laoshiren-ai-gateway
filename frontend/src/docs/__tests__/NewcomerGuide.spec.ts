import { describe, expect, it } from 'vitest'
import { docsConfig } from '@/docs/config'
import newcomerGuide from '@/docs/content/newcomer-overview.md?raw'

describe('newcomer guide documentation contract', () => {
  it('registers a dedicated beginner page without replacing the existing guide', () => {
    const items = docsConfig.flatMap(category => category.items)
    const entry = items.find(item => item.slug === 'newcomer-overview')
    expect(entry?.title).toBe('新手总览')
    expect(entry?.lastModified).toBe('2026-08-29')
    expect(items.some(item => item.slug === 'laoshirenai-guide')).toBe(true)
  })

  it('explains pay-as-you-go and monthly cards honestly', () => {
    expect(newcomerGuide).toContain('## 第二步：按量付费和月卡怎么选')
    expect(newcomerGuide).toContain('未充分使用月额度时，不一定比按量付费划算')
    expect(newcomerGuide).toContain('GPT/Codex Host')
    expect(newcomerGuide).toContain('Claude Host')
    expect(newcomerGuide).toContain('Grok Host')
  })

  it('pins the beginner journey and image-isolation boundary', () => {
    for (const contract of [
      '注册账号 → 选择按量付费或月卡 → 充值/兑换 → 创建 API Key → 选择分组 → 配置工具 → 发出第一次请求',
      'https://api.laoshirenai.com/v1/models',
      'https://api.laoshirenai.com/v1/responses',
      'https://api.laoshirenai.com/gpt-image/v1',
      '文本和生图必须分开',
    ]) expect(newcomerGuide).toContain(contract)
  })

  it('defines eight real-interface screenshot slots with redaction notes', () => {
    expect(newcomerGuide.match(/SCREENSHOT:newcomer-/g) ?? []).toHaveLength(8)
    expect(newcomerGuide).not.toContain('截图待补')
  })

  it('does not embed a real credential', () => {
    expect(newcomerGuide).not.toMatch(/Bearer sk-[A-Za-z0-9_-]{8,}/)
  })
})
