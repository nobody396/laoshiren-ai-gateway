import type { GroupPlatform } from '@/types'
import { clientAutoConfigVersion } from '@/generated/modelCatalog'
import { powershellInstallerSha256, shellInstallerSha256 } from '@/generated/installerIntegrity'

export type ClientAutoConfigTarget = 'claude' | 'codex' | 'grok' | 'gemini'

export interface BuildClientAutoConfigCommandInput {
  target: ClientAutoConfigTarget
  ticket: string
  isWindows?: boolean
  installCodexApp?: boolean
  grokCcSwitchCompat?: boolean
  installMissing?: boolean
}

export interface BuildClientManualConfigCommandInput {
  target: ClientAutoConfigTarget
  apiKey: string
  baseUrl: string
  modelId: string
  protocol: 'responses' | 'chat_completions' | 'messages' | 'generate_content'
  reasoningEffort?: string
  isWindows?: boolean
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
  installMissing = false,
  isWindows = typeof navigator !== 'undefined' &&
    navigator.userAgent.toLowerCase().includes('windows')
}: BuildClientAutoConfigCommandInput): string => {
  if (isWindows) {
    const parts = [
      `$env:LAOSHIRENAI_SETUP_TOKEN=${powerShellSingleQuote(ticket)}`,
      `$env:LAOSHIRENAI_TOOLS='${target}'`,
    ]
    if (!installMissing) parts.push("$env:LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'")
    if (target === 'codex' && installCodexApp) {
      parts.push("$env:LAOSHIRENAI_INSTALL_CODEX_APP='1'")
    }
    if (target === 'grok' && grokCcSwitchCompat) {
      parts.push("$env:LAOSHIRENAI_GROK_CC_SWITCH_COMPAT='1'")
    }
    parts.push(
      `$u=${powerShellSingleQuote(POWERSHELL_INSTALLER_URL)}`,
      '$p=[IO.Path]::GetTempFileName()',
      'irm $u -OutFile $p',
      `if((Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.ToLowerInvariant() -ne '${powershellInstallerSha256}'){Remove-Item -LiteralPath $p -Force; throw '安装器完整性校验失败'}`,
      '$s=[IO.File]::ReadAllText($p)',
      'Remove-Item -LiteralPath $p -Force',
      'Invoke-Expression $s'
    )
    return parts.join('; ')
  }

  const environment = [
    `LAOSHIRENAI_SETUP_TOKEN=${shellSingleQuote(ticket)}`,
    `LAOSHIRENAI_TOOLS=${shellSingleQuote(target)}`,
  ]
  if (!installMissing) environment.push("LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'")
  if (target === 'codex' && installCodexApp) {
    environment.push("LAOSHIRENAI_INSTALL_CODEX_APP='1'")
  }
  if (target === 'grok' && grokCcSwitchCompat) {
    environment.push("LAOSHIRENAI_GROK_CC_SWITCH_COMPAT='1'")
  }
  return [
    'f="$(mktemp)"',
    'trap \'rm -f "$f"\' EXIT',
    `curl -fsSL ${shellSingleQuote(SHELL_INSTALLER_URL)} -o "$f"`,
    `[ "$(shasum -a 256 "$f" | awk '{print $1}')" = '${shellInstallerSha256}' ] || { echo '安装器完整性校验失败' >&2; exit 1; }`,
    `${environment.join(' ')} bash "$f"`,
  ].join('; ')
}

/**
 * Manual-page equivalent of the tested installer flow. Every value comes from
 * the visible form instead of a setup ticket; the installer digest is still
 * verified, client installation is skipped, and only owned fields are merged.
 */
export const buildClientManualConfigCommand = ({
  target,
  apiKey,
  baseUrl,
  modelId,
  protocol,
  reasoningEffort = '',
  isWindows = false,
}: BuildClientManualConfigCommandInput): string => {
  const keyVariable: Record<ClientAutoConfigTarget, string> = {
    claude: 'LAOSHIRENAI_CLAUDE_API_KEY',
    codex: 'LAOSHIRENAI_CODEX_API_KEY',
    grok: 'LAOSHIRENAI_GROK_API_KEY',
    gemini: 'LAOSHIRENAI_GEMINI_API_KEY',
  }
  if (isWindows) {
    return [
      `$env:${keyVariable[target]}=${powerShellSingleQuote(apiKey)}`,
      `$env:LAOSHIRENAI_BASE_URL=${powerShellSingleQuote(baseUrl)}`,
      `$env:LAOSHIRENAI_MODEL_ID=${powerShellSingleQuote(modelId)}`,
      `$env:LAOSHIRENAI_PROTOCOL=${powerShellSingleQuote(protocol)}`,
      ...(reasoningEffort ? [`$env:LAOSHIRENAI_REASONING_EFFORT=${powerShellSingleQuote(reasoningEffort)}`] : []),
      `$env:LAOSHIRENAI_TOOLS=${powerShellSingleQuote(target)}`,
      "$env:LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'",
      `$u=${powerShellSingleQuote(POWERSHELL_INSTALLER_URL)}`,
      '$p=[IO.Path]::GetTempFileName()',
      'irm $u -OutFile $p',
      `if((Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.ToLowerInvariant() -ne '${powershellInstallerSha256}'){Remove-Item -LiteralPath $p -Force; throw '安装器完整性校验失败'}`,
      '$s=[IO.File]::ReadAllText($p)',
      'Remove-Item -LiteralPath $p -Force',
      'Invoke-Expression $s',
    ].join('; ')
  }
  const environment = [
    `${keyVariable[target]}=${shellSingleQuote(apiKey)}`,
    `LAOSHIRENAI_BASE_URL=${shellSingleQuote(baseUrl)}`,
    `LAOSHIRENAI_MODEL_ID=${shellSingleQuote(modelId)}`,
    `LAOSHIRENAI_PROTOCOL=${shellSingleQuote(protocol)}`,
    ...(reasoningEffort ? [`LAOSHIRENAI_REASONING_EFFORT=${shellSingleQuote(reasoningEffort)}`] : []),
    `LAOSHIRENAI_TOOLS=${shellSingleQuote(target)}`,
    "LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'",
  ]
  return [
    'f="$(mktemp)"',
    'trap \'rm -f "$f"\' EXIT',
    `curl -fsSL ${shellSingleQuote(SHELL_INSTALLER_URL)} -o "$f"`,
    `[ "$(shasum -a 256 "$f" | awk '{print $1}')" = '${shellInstallerSha256}' ] || { echo '安装器完整性校验失败' >&2; exit 1; }`,
    `${environment.join(' ')} bash "$f"`,
  ].join('; ')
}
