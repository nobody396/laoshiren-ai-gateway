import type { GroupPlatform } from '@/types'

interface GroupDisplayModelInput {
  name: string
  platform: GroupPlatform
  defaultMappedModel?: string | null
  models?: string[]
}

export const resolveGroupDisplayModel = (input: GroupDisplayModelInput): string => {
  const explicit = input.defaultMappedModel?.trim() || input.models?.find(Boolean)
  if (explicit) return explicit

  const name = input.name.toLowerCase()
  if (/(?:gpt[\s_-]*image|图片生成|生图)/i.test(name)) return 'gpt-image-2'
  if (/(?:glm|智谱)/i.test(name)) return 'glm-5.3'
  if (/(?:deepseek)/i.test(name)) return 'deepseek-v4-pro-0813'
  if (/(?:qwen|千问)/i.test(name)) return 'qwen3.8-max'
  if (/(?:kimi|月之暗面)/i.test(name)) return 'kimi-k3'
  if (/(?:minimax)/i.test(name)) return 'minimax-m3'
  if (/(?:claude|anthropic)/i.test(name)) return 'claude-opus-5'
  if (/(?:gemini)/i.test(name)) return 'gemini-3.7-flash'
  if (/(?:grok)/i.test(name)) return 'grok-4.6'
  if (/(?:gpt|codex|openai)/i.test(name)) return 'gpt-5.6-sol'

  switch (input.platform) {
    case 'anthropic': return 'claude-opus-5'
    case 'gemini':
    case 'antigravity': return 'gemini-3.7-flash'
    case 'grok': return 'grok-4.6'
    case 'gpt-image': return 'gpt-image-2'
    default: return 'gpt-5.6-sol'
  }
}
