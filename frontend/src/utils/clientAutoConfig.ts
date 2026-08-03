import type { GroupPlatform } from '@/types'

export type ClientAutoConfigTarget = 'claude' | 'codex'

export interface BuildClientAutoConfigCommandInput {
  target: ClientAutoConfigTarget
  ticket: string
  isWindows?: boolean
  installCodexApp?: boolean
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
  installCodexApp = false,
  isWindows = typeof navigator !== 'undefined' &&
    navigator.userAgent.toLowerCase().includes('windows')
}: BuildClientAutoConfigCommandInput): string => {
  if (isWindows) {
    const parts = [
      `$env:LAOSHIRENAI_SETUP_TOKEN=${powerShellSingleQuote(ticket)}`,
      `$env:LAOSHIRENAI_TOOLS='${target}'`
    ]
    if (target === 'codex' && installCodexApp) {
      parts.push("$env:LAOSHIRENAI_INSTALL_CODEX_APP='1'")
    }
    parts.push('irm https://laoshirenai.com/auto-config/install.ps1 | iex')
    return parts.join('; ')
  }

  const environment = [
    `LAOSHIRENAI_SETUP_TOKEN=${shellSingleQuote(ticket)}`,
    `LAOSHIRENAI_TOOLS=${shellSingleQuote(target)}`
  ]
  if (target === 'codex' && installCodexApp) {
    environment.push("LAOSHIRENAI_INSTALL_CODEX_APP='1'")
  }
  return `curl -fsSL https://laoshirenai.com/auto-config/install.sh | ${environment.join(' ')} bash`
}
