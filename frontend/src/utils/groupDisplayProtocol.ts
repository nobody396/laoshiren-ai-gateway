import type { GroupPlatform } from '@/types'

export type GroupDisplayProtocol =
  | 'responses'
  | 'chat_completions'
  | 'messages'
  | 'generate_content'
  | 'images'

interface GroupProtocolInput {
  name: string
  platform: GroupPlatform
  defaultMappedModel?: string | null
  allowMessagesDispatch?: boolean
}

const IMAGE_GROUP = /(?:gpt[\s_-]*image|image[\s_-]*\d|生图|图像生成|图片生成)/i
const CHAT_MODEL = /(?:glm|kimi|minimax|grok|智谱|月之暗面)/i
const MESSAGES_MODEL = /(?:claude|anthropic)/i

/**
 * Resolve the primary user-facing protocol for a group.
 *
 * Platform is only the internal upstream adapter. The key selector must not
 * present that adapter as though it were the protocol the user configures.
 */
export const resolveGroupDisplayProtocol = (input: GroupProtocolInput): GroupDisplayProtocol => {
  const identity = `${input.name} ${input.defaultMappedModel ?? ''}`

  if (input.platform === 'gpt-image' || IMAGE_GROUP.test(identity)) return 'images'
  if (input.platform === 'anthropic') return 'messages'
  if (input.platform === 'gemini' || input.platform === 'antigravity') return 'generate_content'
  if (input.platform === 'grok') return 'chat_completions'

  if (MESSAGES_MODEL.test(identity) && input.allowMessagesDispatch) return 'messages'
  if (CHAT_MODEL.test(identity)) return 'chat_completions'

  // GPT/Codex/Qwen/DeepSeek and generic OpenAI-compatible groups use the
  // Responses entry as the primary key-creation contract.
  return 'responses'
}

export const groupDisplayProtocolLabel = (protocol: GroupDisplayProtocol): string => {
  switch (protocol) {
    case 'responses': return 'Responses'
    case 'chat_completions': return 'Chat Completions'
    case 'messages': return 'Messages'
    case 'generate_content': return 'GenerateContent'
    case 'images': return 'Images'
  }
}
