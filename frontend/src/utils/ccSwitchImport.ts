import type { GroupPlatform } from '@/types'

export type CcsImportTarget =
  | 'claude'
  | 'codex'
  | 'grokbuild'
  | 'opencode'
  | 'openclaw'
  | 'hermes'
  | 'gemini'

export type CcsApp = CcsImportTarget

export interface CcsImportGroup {
  id?: number
  platform: GroupPlatform
  name?: string | null
  allow_messages_dispatch?: boolean
  default_mapped_model?: string
}

export interface CcsImportKey {
  key: string
  name?: string | null
  group?: CcsImportGroup | null
}

export interface BuildCcsImportDeeplinkInput {
  key: CcsImportKey
  target: CcsImportTarget
  apiBaseUrl: string
  siteName?: string | null
}

export const DEFAULT_OPENAI_MODEL = 'gpt-5.6-sol'

export const OPENAI_CODEX_MODELS = [
  { model: 'gpt-5.6-sol', displayName: 'GPT-5.6-Sol', contextWindow: 272000 },
  { model: 'gpt-5.6-terra', displayName: 'GPT-5.6-Terra', contextWindow: 272000 },
  { model: 'gpt-5.6-luna', displayName: 'GPT-5.6-Luna', contextWindow: 272000 },
  { model: 'gpt-5.6', displayName: 'GPT-5.6', contextWindow: 272000 },
  { model: 'gpt-5.5', displayName: 'GPT-5.5', contextWindow: 272000 },
  { model: 'gpt-5.4', displayName: 'GPT-5.4', contextWindow: 272000 },
  { model: 'gpt-5.4-mini', displayName: 'GPT-5.4-Mini', contextWindow: 272000 }
] as const

const DEFAULT_CLAUDE_MODELS = {
  haiku: 'claude-haiku-4-5',
  sonnet: 'claude-sonnet-5',
  opus: 'claude-opus-5'
} as const

/**
 * A group platform describes the protocol exposed by the key, while an import
 * target describes the client that will consume that protocol. Keep this
 * compatibility matrix explicit so the UI never creates a provider that looks
 * valid in CC Switch but cannot reach the group's gateway.
 */
export const getCompatibleCcsTargets = (
  platform: GroupPlatform,
  allowMessagesDispatch = false
): CcsImportTarget[] => {
  switch (platform) {
    case 'openai': {
      const targets: CcsImportTarget[] = ['codex', 'opencode', 'openclaw', 'hermes']
      if (allowMessagesDispatch) {
        targets.push('claude')
      }
      return targets
    }
    case 'anthropic':
      return ['claude']
    case 'antigravity':
      return ['claude', 'gemini']
    case 'gemini':
      return ['gemini']
    case 'gpt-image':
      return []
    case 'grok':
      return ['grokbuild']
  }
}

export const isCompatibleCcsTarget = (
  platform: GroupPlatform,
  target: CcsImportTarget,
  allowMessagesDispatch = false
): boolean => getCompatibleCcsTargets(platform, allowMessagesDispatch).includes(target)

const trimLabel = (value: string | null | undefined): string => value?.trim() || ''

const normalizeGatewayBaseUrl = (value: string): string => {
  const normalized = value.trim().replace(/\/+$/, '')
  return normalized.endsWith('/v1') ? normalized.slice(0, -3) : normalized
}

const appendPath = (baseUrl: string, path: string): string => {
  return `${baseUrl.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
}

const encodeBase64Utf8 = (value: string): string => {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary)
}

const escapeTomlString = (value: string): string => {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, '\\n').replace(/\r/g, '\\r')
}

const buildCodexImportConfig = (endpoint: string, providerName: string): string => {
  const safeEndpoint = escapeTomlString(endpoint)
  const safeProviderName = escapeTomlString(providerName)
  const safeModel = escapeTomlString(DEFAULT_OPENAI_MODEL)
  const config = `model_provider = "custom"
model = "${safeModel}"
model_reasoning_effort = "high"
disable_response_storage = true

[model_providers.custom]
name = "${safeProviderName}"
base_url = "${safeEndpoint}"
wire_api = "responses"
requires_openai_auth = true
`

  return JSON.stringify({
    config,
    modelCatalog: {
      models: OPENAI_CODEX_MODELS
    }
  })
}

const FABLE_ENABLED_CLAUDE_GROUP_IDS = new Set([5, 15])

const buildClaudeImportConfig = (enableFable: boolean): string => {
  const env: Record<string, string> = {
    CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1',
    CLAUDE_CODE_EFFORT_LEVEL: 'high'
  }
  if (enableFable) {
    env.ANTHROPIC_DEFAULT_FABLE_MODEL = 'claude-fable-5'
  }

  return JSON.stringify({
    env
  })
}

const appLabelForTarget = (target: CcsImportTarget): string => {
  switch (target) {
    case 'claude':
      return 'Claude Code'
    case 'codex':
      return 'Codex'
    case 'grokbuild':
      return 'Grok Build'
    case 'opencode':
      return 'OpenCode'
    case 'openclaw':
      return 'OpenClaw'
    case 'hermes':
      return 'Hermes'
    case 'gemini':
      return 'Gemini'
  }
}

const endpointForTarget = (
  gatewayBaseUrl: string,
  platform: GroupPlatform,
  target: CcsImportTarget
): string => {
  if (platform === 'antigravity') {
    return appendPath(gatewayBaseUrl, 'antigravity')
  }
  if (
    target === 'codex' ||
    target === 'grokbuild' ||
    target === 'opencode' ||
    target === 'openclaw' ||
    target === 'hermes'
  ) {
    return appendPath(gatewayBaseUrl, 'v1')
  }
  return gatewayBaseUrl
}

const buildProviderName = (
  key: CcsImportKey,
  target: CcsImportTarget,
  siteName?: string | null
): string => {
  const parts = [
    trimLabel(siteName) || 'sub2api',
    appLabelForTarget(target),
    trimLabel(key.group?.name),
    trimLabel(key.name)
  ].filter((part, index, values) => part && (index < 3 || part !== values[2]))

  const name = parts.join(' - ')
  return name.length > 96 ? `${name.slice(0, 93)}...` : name
}

const buildProviderNotes = (
  key: CcsImportKey,
  endpoint: string
): string => {
  return [
    key.group?.name ? `Group: ${key.group.name}` : '',
    key.name ? `API Key: ${key.name}` : '',
    `Endpoint: ${endpoint}`
  ].filter(Boolean).join('\n')
}

export const buildCcsImportDeeplink = ({
  key,
  target,
  apiBaseUrl,
  siteName
}: BuildCcsImportDeeplinkInput): string => {
  const platform = key.group?.platform || 'anthropic'
  const allowMessagesDispatch = key.group?.allow_messages_dispatch === true
  if (!isCompatibleCcsTarget(platform, target, allowMessagesDispatch)) {
    throw new Error(`CC Switch target "${target}" is not compatible with platform "${platform}"`)
  }

  const gatewayBaseUrl = normalizeGatewayBaseUrl(apiBaseUrl)
  const app: CcsApp = target
  const endpoint = endpointForTarget(gatewayBaseUrl, platform, target)
  const providerName = buildProviderName(key, target, siteName)
  const providerNotes = buildProviderNotes(key, endpoint)
  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

  const params = new URLSearchParams({
    resource: 'provider',
    app,
    name: providerName,
    homepage: gatewayBaseUrl,
    endpoint,
    apiKey: key.key,
    notes: providerNotes,
    usageEnabled: 'true',
    usageScript: btoa(usageScript),
    usageBaseUrl: gatewayBaseUrl,
    usageAutoInterval: '30'
  })

  if (target === 'grokbuild') {
    params.set('model', 'grok-4.5')
  } else if (
    target === 'codex' ||
    target === 'opencode' ||
    target === 'openclaw' ||
    target === 'hermes'
  ) {
    params.set('model', DEFAULT_OPENAI_MODEL)
  }

  if (target === 'codex') {
    params.set('configFormat', 'json')
    params.set('config', encodeBase64Utf8(buildCodexImportConfig(endpoint, providerName)))
  }

  if (target === 'claude') {
    params.set('model', 'claude-opus-5')
    params.set('configFormat', 'json')
    params.set(
      'config',
      encodeBase64Utf8(buildClaudeImportConfig(FABLE_ENABLED_CLAUDE_GROUP_IDS.has(key.group?.id ?? -1)))
    )
    const groupModel = key.group?.default_mapped_model?.trim()
    if (platform === 'anthropic' && groupModel) {
      params.set('haikuModel', groupModel)
      params.set('sonnetModel', groupModel)
      params.set('opusModel', groupModel)
    } else if (platform === 'anthropic' || platform === 'openai') {
      params.set('haikuModel', DEFAULT_CLAUDE_MODELS.haiku)
      params.set('sonnetModel', DEFAULT_CLAUDE_MODELS.sonnet)
      params.set('opusModel', DEFAULT_CLAUDE_MODELS.opus)
    }
  }

  return `ccswitch://v1/import?${params.toString()}`
}
