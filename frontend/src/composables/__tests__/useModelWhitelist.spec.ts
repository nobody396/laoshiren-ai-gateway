import { describe, expect, it, vi } from 'vitest'

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

import { buildModelMappingObject, getModelsByPlatform } from '../useModelWhitelist'

describe('useModelWhitelist', () => {
  it('openai 模型列表默认只填充当前官方主力模型', () => {
    const models = getModelsByPlatform('openai')

    expect(models).toContain('gpt-5.5')
    expect(models).toContain('gpt-5.5-pro')
    expect(models).toContain('gpt-5.4')
    expect(models).toContain('gpt-5.1-codex')
    expect(models).not.toContain('gpt-3.5-turbo')
    expect(models).not.toContain('gpt-4')
  })

  it('anthropic 模型列表默认不再包含 Claude 2 和 Claude 3 历史模型', () => {
    const models = getModelsByPlatform('anthropic')

    expect(models).toContain('claude-opus-4-8')
    expect(models).toContain('claude-opus-latest')
    expect(models).toContain('claude-opus-4-7')
    expect(models).toContain('claude-sonnet-4-6')
    expect(models).toContain('claude-haiku-4-5-20251001')
    expect(models).not.toContain('claude-2.1')
    expect(models).not.toContain('claude-3-haiku-20240307')
  })

  it('antigravity 模型列表包含图片模型兼容项', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models).toContain('gemini-3-pro-image')
  })

  it('gemini 模型列表包含原生生图模型', () => {
    const models = getModelsByPlatform('gemini')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.0-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
  })

  it('antigravity 模型列表会把新的 Gemini 图片模型排在前面', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash-lite'))
  })

  it('whitelist 模式会忽略通配符条目', () => {
    const mapping = buildModelMappingObject('whitelist', ['claude-*', 'gemini-3.1-flash-image'], [])
    expect(mapping).toEqual({
      'gemini-3.1-flash-image': 'gemini-3.1-flash-image'
    })
  })

  it('whitelist 模式会保留 GPT-5.4 官方模型的精确映射', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4'], [])

    expect(mapping).toEqual({
      'gpt-5.4': 'gpt-5.4'
    })
  })
})
