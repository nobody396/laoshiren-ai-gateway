import { describe, expect, it } from 'vitest'
import { resolveGroupDisplayProtocol } from '../groupDisplayProtocol'

describe('resolveGroupDisplayProtocol', () => {
  it('uses the public protocol instead of the upstream platform identity', () => {
    expect(resolveGroupDisplayProtocol({ name: 'GLM 企业高速线路', platform: 'openai' })).toBe('chat_completions')
    expect(resolveGroupDisplayProtocol({ name: 'GPT 标准线路', platform: 'openai' })).toBe('responses')
    expect(resolveGroupDisplayProtocol({ name: 'Claude 标准线路', platform: 'anthropic' })).toBe('messages')
    expect(resolveGroupDisplayProtocol({ name: 'Gemini 分组', platform: 'gemini' })).toBe('generate_content')
    expect(resolveGroupDisplayProtocol({ name: 'GPT Image 2 生图分组', platform: 'openai' })).toBe('images')
    expect(resolveGroupDisplayProtocol({ name: 'GPT 图片生成线路', platform: 'openai' })).toBe('images')
  })

  it('recognizes an OpenAI-routed Messages group from its public contract', () => {
    expect(resolveGroupDisplayProtocol({
      name: 'Claude 兼容线路',
      platform: 'openai',
      allowMessagesDispatch: true
    })).toBe('messages')
  })
})
