import type { GroupPlatform } from '@/types'
import {
  clientAutoConfigDefaults,
  optionalCatalogClientDefaultForPlatform
} from '@/generated/modelCatalog'
import { codexClientModels, type CodexClientModel } from '@/generated/codexClientCatalog'

export type CcsImportTarget =
  | 'claude'
  | 'codex'
  | 'grokbuild'
  | 'opencode'
  | 'openclaw'
  | 'hermes'
  | 'gemini'

export type CcsApp = CcsImportTarget
export type CodexContextProfile = 'standard' | 'long'

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
  availableModels?: readonly string[]
  codexContextProfile?: CodexContextProfile
}

export const DEFAULT_OPENAI_MODEL = optionalCatalogClientDefaultForPlatform('openai')?.id ?? 'gpt-5.6-sol'

// Unknown ad-hoc aliases fall back to the historical Codex window. Every
// canonical model keeps its own matrix-derived window instead of being forced
// into one global 272K value.
export const CODEX_CONTEXT_WINDOW_TOKENS = 272000
export const CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT = 95
export const CODEX_AUTO_COMPACT_TOKEN_LIMIT = 258000
export const CODEX_LONG_CONTEXT_WINDOW_TOKENS = 1000000
export const CODEX_LONG_AUTO_COMPACT_TOKEN_LIMIT = 900000

const codexContextSettings = (profile: CodexContextProfile, nativeContext = CODEX_CONTEXT_WINDOW_TOKENS) => profile === 'long'
  ? {
      contextWindow: CODEX_LONG_CONTEXT_WINDOW_TOKENS,
      effectiveContextWindowPercent: 100,
      autoCompactTokenLimit: CODEX_LONG_AUTO_COMPACT_TOKEN_LIMIT
    }
  : {
      contextWindow: nativeContext,
      effectiveContextWindowPercent: CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT,
      autoCompactTokenLimit: Math.floor(nativeContext * CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT / 100)
    }

// One generated source drives both the downloaded Codex catalog and CC Switch
// imports so disabled models cannot remain in only one client path.
export const OPENAI_CODEX_MODELS = codexClientModels

const generatedCodexModelByID = new Map(
  OPENAI_CODEX_MODELS.map((model) => [model.model, model] as const)
)

const formatModelDisplayName = (modelID: string): string => {
  const displayToken = (token: string): string => {
    switch (token.toLowerCase()) {
      case 'gpt': return 'GPT'
      case 'codex': return 'Codex'
      case 'openai': return 'OpenAI'
      case 'daybreak': return 'Daybreak'
      case 'blue': return 'Blue'
      case 'latest': return 'Latest'
      case 'auto': return 'Auto'
      case 'review': return 'Review'
      case 'compact': return 'Compact'
      case 'sol': return 'Sol'
      case 'terra': return 'Terra'
      case 'luna': return 'Luna'
      default: return token
    }
  }

  return modelID.split('-').map(displayToken).join(' ')
}

export const resolveCodexModels = (availableModels?: readonly string[]): readonly CodexClientModel[] => {
  if (availableModels === undefined) return OPENAI_CODEX_MODELS

  const seen = new Set<string>()
  return availableModels.flatMap((rawModel) => {
    const modelID = rawModel.trim()
    if (!modelID || seen.has(modelID)) return []
    seen.add(modelID)

    const generated = generatedCodexModelByID.get(modelID)
    if (generated) return [generated]
    return [{
      model: modelID,
      displayName: formatModelDisplayName(modelID),
      contextWindow: CODEX_CONTEXT_WINDOW_TOKENS,
      reasoningLevels: [],
      defaultReasoningLevel: null,
    }]
  })
}

export const selectDefaultOpenAIModel = (
  models: readonly CodexClientModel[],
  groupDefault?: string
): string => {
  const modelIDs = new Set(models.map((model) => model.model))
  if (modelIDs.has(DEFAULT_OPENAI_MODEL)) return DEFAULT_OPENAI_MODEL

  const normalizedGroupDefault = groupDefault?.trim()
  if (normalizedGroupDefault && modelIDs.has(normalizedGroupDefault)) return normalizedGroupDefault
  return models[0]?.model ?? DEFAULT_OPENAI_MODEL
}

export const buildCodexModelCatalog = (
  models: readonly CodexClientModel[] = OPENAI_CODEX_MODELS,
  contextProfile: CodexContextProfile = 'standard'
): string => JSON.stringify({
  models: models.map((model, index) => {
    const context = codexContextSettings(contextProfile, model.contextWindow)
    return {
      slug: model.model,
      display_name: model.displayName,
      description: `${model.displayName} coding model.`,
      base_instructions: 'You are Codex, a coding agent. You and the user share the same workspace and collaborate to achieve the user\'s goals.',
      default_reasoning_level: model.defaultReasoningLevel,
      supported_reasoning_levels: model.reasoningLevels.map(effort => ({
        effort,
        description: effort === 'none'
          ? 'Disable explicit reasoning'
          : effort === 'max'
            ? 'Maximum reasoning depth'
            : `${effort[0]?.toUpperCase() ?? ''}${effort.slice(1)} reasoning depth`,
      })),
      shell_type: 'shell_command',
      visibility: 'list',
      supported_in_api: true,
      priority: index + 1,
      supports_reasoning_summaries: true,
      default_reasoning_summary: 'none',
      support_verbosity: true,
      truncation_policy: { mode: 'bytes', limit: 10000 },
      supports_parallel_tool_calls: true,
      supports_image_detail_original: false,
      context_window: context.contextWindow,
      max_context_window: context.contextWindow,
      effective_context_window_percent: context.effectiveContextWindowPercent,
      auto_compact_token_limit: context.autoCompactTokenLimit,
      experimental_supported_tools: [],
      input_modalities: ['text', 'image'],
      supports_search_tool: true,
      use_responses_lite: false,
      availability_nux: null,
      upgrade: null
    }
  })
}, null, 2)

const CATALOG_CLAUDE_DEFAULT_MODEL = clientAutoConfigDefaults.anthropic

const DEFAULT_CLAUDE_MODELS = {
  haiku: 'claude-haiku-4-5',
  sonnet: 'claude-sonnet-5',
  opus: CATALOG_CLAUDE_DEFAULT_MODEL
} as const

export interface ClaudeClientModels {
  model: string
  haiku: string
  sonnet: string
  opus: string
}

// Native Anthropic-protocol groups such as GLM expose one group-owned client
// default through default_mapped_model. That contract must win over the global
// Claude catalog default for both the main model and every Claude role slot.
export const resolveClaudeClientModels = (
  platform: GroupPlatform | null,
  groupDefault?: string
): ClaudeClientModels => {
  const nativeGroupModel = platform === 'anthropic' ? groupDefault?.trim() : ''
  if (nativeGroupModel) {
    return {
      model: nativeGroupModel,
      haiku: nativeGroupModel,
      sonnet: nativeGroupModel,
      opus: nativeGroupModel
    }
  }

  return {
    model: CATALOG_CLAUDE_DEFAULT_MODEL,
    ...DEFAULT_CLAUDE_MODELS
  }
}

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
    case 'universal':
      return ['codex', 'opencode', 'openclaw', 'hermes', 'claude']
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

const buildCodexImportConfig = (
  endpoint: string,
  providerName: string,
  models: readonly CodexClientModel[],
  defaultModel: string,
  contextProfile: CodexContextProfile
): string => {
  const selectedModel = models.find((model) => model.model === defaultModel) ?? models[0]
  const context = codexContextSettings(contextProfile, selectedModel?.contextWindow)
  const safeEndpoint = escapeTomlString(endpoint)
  const safeProviderName = escapeTomlString(providerName)
  const safeModel = escapeTomlString(defaultModel)
  const config = `model_provider = "custom"
model = "${safeModel}"
model_reasoning_effort = "high"
model_context_window = ${context.contextWindow}
model_auto_compact_token_limit = ${context.autoCompactTokenLimit}
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
      models: models.map((model) => ({
        ...model,
        contextWindow: codexContextSettings(contextProfile, model.contextWindow).contextWindow,
        // CC Switch's Codex model catalog follows the same visibility contract
        // as the downloaded catalog file; without it imported models can be
        // hidden from the client model selector.
        visibility: 'list'
      }))
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
  siteName?: string | null,
  codexContextProfile: CodexContextProfile = 'standard'
): string => {
  const parts = [
    trimLabel(siteName) || 'sub2api',
    appLabelForTarget(target),
    trimLabel(key.group?.name),
    trimLabel(key.name),
    target === 'codex' ? (codexContextProfile === 'long' ? '1M' : '原生上下文') : ''
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
  siteName,
  availableModels,
  codexContextProfile = 'standard'
}: BuildCcsImportDeeplinkInput): string => {
  const platform = key.group?.platform || 'anthropic'
  const allowMessagesDispatch = key.group?.allow_messages_dispatch === true
  if (!isCompatibleCcsTarget(platform, target, allowMessagesDispatch)) {
    throw new Error(`CC Switch target "${target}" is not compatible with platform "${platform}"`)
  }
  if (target === 'grokbuild') {
    throw new Error(
      'CC Switch 3.19.2 cannot preserve Grok Build multi-model deeplinks; use the one-click compatibility setup'
    )
  }

  const gatewayBaseUrl = normalizeGatewayBaseUrl(apiBaseUrl)
  const app: CcsApp = target
  const endpoint = endpointForTarget(gatewayBaseUrl, platform, target)
  const providerName = buildProviderName(key, target, siteName, codexContextProfile)
  const providerNotes = buildProviderNotes(key, endpoint)
  const codexModels = resolveCodexModels(availableModels)
  if (platform === 'openai' && availableModels !== undefined && codexModels.length === 0) {
    throw new Error('CC Switch import requires at least one active model for the selected OpenAI group')
  }
  const defaultOpenAIModel = selectDefaultOpenAIModel(
    codexModels,
    key.group?.default_mapped_model
  )
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

  if (
    target === 'codex' ||
    target === 'opencode' ||
    target === 'openclaw' ||
    target === 'hermes'
  ) {
    params.set('model', defaultOpenAIModel)
  }

  if (target === 'codex') {
    params.set('configFormat', 'json')
    params.set(
      'config',
      encodeBase64Utf8(buildCodexImportConfig(
        endpoint,
        providerName,
        codexModels,
        defaultOpenAIModel,
        codexContextProfile
      ))
    )
  }

  if (target === 'claude') {
    const claudeModels = resolveClaudeClientModels(
      platform,
      key.group?.default_mapped_model
    )
    params.set('model', claudeModels.model)
    params.set('configFormat', 'json')
    params.set(
      'config',
      encodeBase64Utf8(buildClaudeImportConfig(FABLE_ENABLED_CLAUDE_GROUP_IDS.has(key.group?.id ?? -1)))
    )
    const groupModel = key.group?.default_mapped_model?.trim()
    if (platform === 'anthropic' && groupModel) {
      params.set('haikuModel', claudeModels.haiku)
      params.set('sonnetModel', claudeModels.sonnet)
      params.set('opusModel', claudeModels.opus)
    } else if (platform === 'anthropic' || platform === 'openai') {
      params.set('haikuModel', DEFAULT_CLAUDE_MODELS.haiku)
      params.set('sonnetModel', DEFAULT_CLAUDE_MODELS.sonnet)
      params.set('opusModel', DEFAULT_CLAUDE_MODELS.opus)
    }
  }

  return `ccswitch://v1/import?${params.toString()}`
}
