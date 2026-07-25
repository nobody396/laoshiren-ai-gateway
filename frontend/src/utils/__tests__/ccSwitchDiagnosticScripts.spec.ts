import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const readPublicScript = (name: string) =>
  readFileSync(resolve(process.cwd(), 'public', 'auto-config', name), 'utf8')

describe('CC Switch public diagnostic scripts', () => {
  it('repairs the Windows protocol for the current user without admin access', () => {
    const script = readPublicScript('diagnose-cc-switch.ps1')

    expect(script).toContain("$ScriptVersion = '1.0.0'")
    expect(script).toContain("HKCU:\\Software\\Classes\\ccswitch")
    expect(script).toContain('Programs\\CC Switch\\cc-switch.exe')
    expect(script).toContain("(Join-Path $env:USERPROFILE 'Downloads')")
    expect(script).toContain("[Environment]::GetFolderPath('Desktop')")
    expect(script).toContain("$MinimumVersion = [Version]'3.16.5'")
  })

  it('re-registers the official macOS bundle and supports apps opened from Downloads', () => {
    const script = readPublicScript('diagnose-cc-switch.sh')

    expect(script).toContain('SCRIPT_VERSION="1.0.0"')
    expect(script).toContain('BUNDLE_ID="com.ccswitch.desktop"')
    expect(script).toContain('/usr/bin/open -a "CC Switch" --args --register-protocol')
    expect(script).toContain('$HOME/Applications/CC Switch.app')
  })

  it('uses only the official CC Switch release page for upgrade guidance', () => {
    for (const name of ['diagnose-cc-switch.ps1', 'diagnose-cc-switch.sh']) {
      expect(readPublicScript(name)).toContain(
        'https://github.com/farion1231/cc-switch/releases/latest'
      )
    }
  })
})
