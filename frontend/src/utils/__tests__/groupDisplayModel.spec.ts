import { describe, expect, it } from 'vitest'
import { resolveGroupDisplayModel } from '../groupDisplayModel'

describe('resolveGroupDisplayModel', () => {
  it('uses the real model family identity rather than a protocol icon', () => {
    expect(resolveGroupDisplayModel({ name: 'GLM 企业高速线路', platform: 'openai' })).toBe('glm-5.3')
    expect(resolveGroupDisplayModel({ name: 'GPT 标准线路', platform: 'openai' })).toBe('gpt-5.6-sol')
    expect(resolveGroupDisplayModel({ name: 'Claude 标准线路', platform: 'anthropic' })).toBe('claude-opus-5')
    expect(resolveGroupDisplayModel({ name: 'Gemini 标准线路', platform: 'gemini' })).toBe('gemini-3.7-flash')
  })

  it('prefers a concrete mapped or catalog model when available', () => {
    expect(resolveGroupDisplayModel({ name: '模型线路', platform: 'openai', defaultMappedModel: 'glm-5.3' })).toBe('glm-5.3')
    expect(resolveGroupDisplayModel({ name: '模型线路', platform: 'openai', models: ['qwen3.8-max'] })).toBe('qwen3.8-max')
  })
})
