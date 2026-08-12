import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const readPublicScript = (name: string) =>
  readFileSync(resolve(process.cwd(), 'public', 'auto-config', name), 'utf8')

describe('CC Switch public diagnostic scripts', () => {
  it('repairs the Windows protocol for the current user without admin access', () => {
    const script = readPublicScript('diagnose-cc-switch.ps1')

    expect(script).toContain("$ScriptVersion = '1.2.1'")
    expect(script).toContain("HKCU:\\Software\\Classes\\ccswitch")
    expect(script).toContain('Programs\\CC Switch\\cc-switch.exe')
    expect(script).toContain("(Join-Path $env:USERPROFILE 'Downloads')")
    expect(script).toContain("[Environment]::GetFolderPath('Desktop')")
    expect(script).toContain("$MinimumVersion = [Version]'3.16.5'")
  })

  it('re-registers the official macOS bundle and supports apps opened from Downloads', () => {
    const script = readPublicScript('diagnose-cc-switch.sh')

    expect(script).toContain('SCRIPT_VERSION="1.2.1"')
    expect(script).toContain('BUNDLE_ID="com.ccswitch.desktop"')
    expect(script).toContain('/usr/bin/open "$APP_PATH" --args --register-protocol')
    expect(script).toContain('$HOME/Applications/CC Switch.app')
  })

  it('skips incomplete Windows uninstall entries under strict mode', () => {
    const script = readPublicScript('diagnose-cc-switch.ps1')

    expect(script).toContain('function Get-OptionalPropertyValue')
    expect(script).toContain("Get-OptionalPropertyValue -InputObject $_ -Name 'DisplayName'")
    expect(script).toContain("Get-OptionalPropertyValue -InputObject $_ -Name 'InstallLocation'")
    expect(script).toContain("Get-OptionalPropertyValue -InputObject $_ -Name 'DisplayIcon'")
    expect(script).not.toContain("$_.DisplayName -like 'CC Switch*'")
  })

  it('automatically installs the verified same-site Windows cache with official fallback', () => {
    const script = readPublicScript('diagnose-cc-switch.ps1')

    expect(script).toContain(
      'https://laoshirenai.com/api/v1/public-downloads/cc-switch/latest.json'
    )
    expect(script).toContain('return Get-MirrorCcSwitchAsset')
    expect(script).toContain(
      'https://api.github.com/repos/farion1231/cc-switch/releases/latest'
    )
    expect(script).toContain("return 'Windows-arm64.msi'")
    expect(script).toContain("return 'Windows.msi'")
    expect(script).toContain('Get-FileHash -LiteralPath $TemporaryMsi -Algorithm SHA256')
    expect(script).toContain("Start-Process -FilePath 'msiexec.exe'")
    expect(script).toContain("'/passive', '/norestart'")
    expect(script).toContain('if ($InstalledVersion -lt $LatestAsset.Version)')
    expect(script).toContain('Wait-For-CcSwitchExit')
    expect(script).not.toContain('Stop-Process')
  })

  it('automatically installs the signed and notarized same-site macOS cache with official fallback', () => {
    const script = readPublicScript('diagnose-cc-switch.sh')

    expect(script).toContain(
      'https://laoshirenai.com/api/v1/public-downloads/cc-switch/latest.json'
    )
    expect(script).toContain('fetch_latest_macos_mirror')
    expect(script).toContain(
      'https://api.github.com/repos/farion1231/cc-switch/releases/latest'
    )
    expect(script).toContain('CC-Switch-v${LATEST_VERSION}-macOS.tar.gz')
    expect(script).toContain('/usr/bin/shasum -a 256 "$archive"')
    expect(script).toContain('/usr/bin/codesign --verify --deep --strict "$app_path"')
    expect(script).toContain('OFFICIAL_TEAM_ID="R8UR22V2F9"')
    expect(script).toContain('/usr/sbin/spctl --assess --type execute "$app_path"')
    expect(script).toContain('"$brew_path" upgrade --cask cc-switch')
    expect(script).toContain('version_is_older "$VERSION" "$LATEST_VERSION"')
    expect(script).not.toContain('/usr/bin/killall')
  })

  it('opens only the official release page when verified automatic installation fails', () => {
    for (const name of ['diagnose-cc-switch.ps1', 'diagnose-cc-switch.sh']) {
      const script = readPublicScript(name)

      expect(script).toContain('https://github.com/farion1231/cc-switch/releases/latest')
      expect(script).toContain('自动安装')
    }
  })
})
