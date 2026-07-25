import type { GroupPlatform } from '@/types'

export type ClientAutoConfigTarget = 'claude' | 'codex'

export interface BuildClientAutoConfigCommandInput {
  target: ClientAutoConfigTarget
  platform: GroupPlatform
  apiKey: string
  baseUrl: string
  isWindows?: boolean
}

const shellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "'\"'\"'")}'`
}

const powerShellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "''")}'`
}

const resolveClientBaseUrl = (platform: GroupPlatform, baseUrl: string): string => {
  const normalized = baseUrl.trim().replace(/\/+$/, '')
  if (platform === 'antigravity' && !normalized.endsWith('/antigravity')) {
    return `${normalized}/antigravity`
  }
  return normalized
}

export const getClientAutoConfigTarget = (
  platform?: GroupPlatform | null
): ClientAutoConfigTarget | null => {
  switch (platform) {
    case 'openai':
      return 'codex'
    case 'anthropic':
    case 'antigravity':
      return 'claude'
    case 'gemini':
    case 'gpt-image':
    case undefined:
    case null:
      return null
  }
}

export const getClientAutoConfigName = (target: ClientAutoConfigTarget): string => {
  return target === 'claude' ? 'Claude Code' : 'Codex'
}

export const buildClientAutoConfigCommand = ({
  target,
  platform,
  apiKey,
  baseUrl,
  isWindows = typeof navigator !== 'undefined' &&
    navigator.userAgent.toLowerCase().includes('windows')
}: BuildClientAutoConfigCommandInput): string => {
  const clientBaseUrl = resolveClientBaseUrl(platform, baseUrl)

  if (isWindows) {
    const keyVariable = target === 'claude'
      ? 'LAOSHIRENAI_CLAUDE_API_KEY'
      : 'LAOSHIRENAI_CODEX_API_KEY'
    return [
      `$env:${keyVariable}=${powerShellSingleQuote(apiKey)}`,
      `$env:LAOSHIRENAI_TOOLS='${target}'`,
      `$env:LAOSHIRENAI_BASE_URL=${powerShellSingleQuote(clientBaseUrl)}`,
      'irm https://laoshirenai.com/auto-config/install.ps1 | iex'
    ].join('; ')
  }

  const keyArgument = target === 'claude' ? '--api-key' : '--codex-api-key'
  return [
    'curl -fsSL https://laoshirenai.com/auto-config/install.sh | bash -s --',
    `${keyArgument} ${shellSingleQuote(apiKey)}`,
    `--tools ${target}`,
    `--base-url ${shellSingleQuote(clientBaseUrl)}`
  ].join(' ')
}
