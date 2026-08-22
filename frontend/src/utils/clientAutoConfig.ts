import type { GroupPlatform } from '@/types'
import { clientAutoConfigVersion } from '@/generated/modelCatalog'

export type ClientAutoConfigTarget = 'claude' | 'codex' | 'grok' | 'gemini'

export interface BuildClientAutoConfigCommandInput {
  target: ClientAutoConfigTarget
  ticket: string
  isWindows?: boolean
  installCodexApp?: boolean
  grokCcSwitchCompat?: boolean
}

const shellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "'\"'\"'")}'`
}

const powerShellSingleQuote = (value: string): string => {
  return `'${value.replace(/'/g, "''")}'`
}

// Edge CDN keeps public installer paths for a long time. Version the copied
// URL so a newly deployed setup contract cannot execute a stale installer.
const SHELL_INSTALLER_URL = `https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}`
const POWERSHELL_INSTALLER_URL = `https://laoshirenai.com/auto-config/install.ps1?v=${clientAutoConfigVersion}`

export const getClientAutoConfigTarget = (
  platform?: GroupPlatform | null
): ClientAutoConfigTarget | null => {
  switch (platform) {
    case 'openai':
      return 'codex'
    case 'anthropic':
    case 'antigravity':
      return 'claude'
    case 'grok':
      return 'grok'
    case 'gemini':
      return 'gemini'
    case 'universal':
      return null
    case 'gpt-image':
    case undefined:
    case null:
      return null
  }
}

export const getClientAutoConfigName = (target: ClientAutoConfigTarget): string => {
  if (target === 'claude') return 'Claude Code'
  if (target === 'grok') return 'Grok Build'
  if (target === 'gemini') return 'Gemini CLI'
  return 'Codex'
}

export const buildClientAutoConfigCommand = ({
  target,
  ticket,
  installCodexApp = false,
  grokCcSwitchCompat = false,
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
    if (target === 'grok' && grokCcSwitchCompat) {
      parts.push("$env:LAOSHIRENAI_GROK_CC_SWITCH_COMPAT='1'")
    }
    parts.push(`irm ${POWERSHELL_INSTALLER_URL} | iex`)
    return parts.join('; ')
  }

  const environment = [
    `LAOSHIRENAI_SETUP_TOKEN=${shellSingleQuote(ticket)}`,
    `LAOSHIRENAI_TOOLS=${shellSingleQuote(target)}`
  ]
  if (target === 'codex' && installCodexApp) {
    environment.push("LAOSHIRENAI_INSTALL_CODEX_APP='1'")
  }
  if (target === 'grok' && grokCcSwitchCompat) {
    environment.push("LAOSHIRENAI_GROK_CC_SWITCH_COMPAT='1'")
  }
  return `curl -fsSL ${shellSingleQuote(SHELL_INSTALLER_URL)} | ${environment.join(' ')} bash`
}
