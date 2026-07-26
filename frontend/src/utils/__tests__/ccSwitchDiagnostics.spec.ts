import { describe, expect, it } from 'vitest'

import {
  buildCcsDiagnosticCommand,
  detectCcsDiagnosticPlatform
} from '@/utils/ccSwitchDiagnostics'

describe('CC Switch diagnostic platform detection', () => {
  it('detects Windows browsers', () => {
    expect(detectCcsDiagnosticPlatform(
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
      'Win32'
    )).toBe('windows')
  })

  it('detects macOS browsers', () => {
    expect(detectCcsDiagnosticPlatform(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
      'MacIntel'
    )).toBe('macos')
  })

  it('does not guess on unsupported platforms', () => {
    expect(detectCcsDiagnosticPlatform(
      'Mozilla/5.0 (X11; Linux x86_64)',
      'Linux x86_64'
    )).toBeNull()
  })
})

describe('CC Switch diagnostic commands', () => {
  it('builds a one-line PowerShell repair command', () => {
    expect(buildCcsDiagnosticCommand('windows', 'https://laoshirenai.com/')).toBe(
      'irm https://laoshirenai.com/auto-config/diagnose-cc-switch.ps1?v=1.2.0 | iex'
    )
  })

  it('builds a one-line macOS Terminal repair command', () => {
    expect(buildCcsDiagnosticCommand('macos', 'https://example.com')).toBe(
      "curl -fsSL 'https://example.com/auto-config/diagnose-cc-switch.sh?v=1.2.0' | bash"
    )
  })

  it('uses the production site when no origin is supplied', () => {
    expect(buildCcsDiagnosticCommand('windows')).toContain('https://laoshirenai.com/')
  })
})
