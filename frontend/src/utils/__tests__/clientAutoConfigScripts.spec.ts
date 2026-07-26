import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const readPublicScript = (name: string) =>
  readFileSync(resolve(process.cwd(), 'public', 'auto-config', name), 'utf8')

describe('client auto-config scripts', () => {
  it('reuses an existing Claude Code CLI on macOS and Linux', () => {
    const script = readPublicScript('install.sh')

    expect(script).toContain('SCRIPT_VERSION="0.5.0"')
    expect(script).toContain('EXISTING_CLAUDE_COMMAND="$(get_usable_client_command claude || true)"')
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('exchange_setup_ticket')
    expect(script).toContain('install_codex_app_if_requested')
    expect(script).toContain('${api_base_url}/usage')
    expect(script).toContain('余额/套餐额度不足')
  })

  it('reuses an existing Claude Code CLI on Windows', () => {
    const script = readPublicScript('install.ps1')

    expect(script).toContain("$ScriptVersion = '0.5.0'")
    expect(script).toContain("Get-UsableClientCommand -CommandName 'claude'")
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('Exchange-SetupTicket')
    expect(script).toContain('Install-CodexAppIfRequested')
    expect(script).toContain('$ApiBaseUrl/usage')
    expect(script).toContain('余额/套餐额度不足')
  })

  it('writes Claude settings without replacing unrelated JSON fields', () => {
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('ANTHROPIC_BASE_URL')
      expect(script).toContain('ANTHROPIC_AUTH_TOKEN')
      expect(script).toContain('CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY')
      expect(script).toContain('model = "gpt-5.6-sol"')
      expect(script).not.toContain('model = "gpt-5.6"')
    }
  })
})
