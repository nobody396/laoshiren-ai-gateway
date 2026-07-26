export type CcsDiagnosticPlatform = 'windows' | 'macos'

const DEFAULT_SITE_ORIGIN = 'https://laoshirenai.com'
const DIAGNOSTIC_SCRIPT_VERSION = '1.2.0'

export const detectCcsDiagnosticPlatform = (
  userAgent = typeof navigator === 'undefined' ? '' : navigator.userAgent,
  platform = typeof navigator === 'undefined' ? '' : navigator.platform
): CcsDiagnosticPlatform | null => {
  const fingerprint = `${userAgent} ${platform}`.toLowerCase()

  if (fingerprint.includes('windows') || fingerprint.includes('win32') || fingerprint.includes('win64')) {
    return 'windows'
  }

  if (fingerprint.includes('macintosh') || fingerprint.includes('mac os') || fingerprint.includes('darwin')) {
    return 'macos'
  }

  return null
}

const normalizeOrigin = (origin?: string): string => {
  const value = origin?.trim().replace(/\/+$/, '')
  return value || DEFAULT_SITE_ORIGIN
}

export const buildCcsDiagnosticCommand = (
  platform: CcsDiagnosticPlatform,
  origin?: string
): string => {
  const siteOrigin = normalizeOrigin(origin)

  if (platform === 'windows') {
    return `irm ${siteOrigin}/auto-config/diagnose-cc-switch.ps1?v=${DIAGNOSTIC_SCRIPT_VERSION} | iex`
  }

  return `curl -fsSL '${siteOrigin}/auto-config/diagnose-cc-switch.sh?v=${DIAGNOSTIC_SCRIPT_VERSION}' | bash`
}
