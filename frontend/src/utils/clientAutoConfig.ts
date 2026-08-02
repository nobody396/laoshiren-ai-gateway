import type { GroupPlatform } from '@/types'

export type ClientAutoConfigTarget = 'claude' | 'codex'

export interface BuildClientAutoConfigCommandInput {
  target: ClientAutoConfigTarget
  ticket: string
  isWindows?: boolean
}

const shellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "'\"'\"'")}'`
}

const powerShellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "''")}'`
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
  ticket,
  isWindows = typeof navigator !== 'undefined' &&
    navigator.userAgent.toLowerCase().includes('windows')
}: BuildClientAutoConfigCommandInput): string => {
  if (isWindows) {
    return [
      `$env:LAOSHIRENAI_SETUP_TOKEN=${powerShellSingleQuote(ticket)}`,
      `$env:LAOSHIRENAI_TOOLS='${target}'`,
      'irm https://laoshirenai.com/auto-config/install.ps1 | iex'
    ].join('; ')
  }

  return [
    'curl -fsSL https://laoshirenai.com/auto-config/install.sh |',
    `LAOSHIRENAI_SETUP_TOKEN=${shellSingleQuote(ticket)}`,
    `LAOSHIRENAI_TOOLS=${shellSingleQuote(target)}`,
    'bash'
  ].join(' ')
}
