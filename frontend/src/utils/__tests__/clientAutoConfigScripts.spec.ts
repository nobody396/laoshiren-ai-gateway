import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const readPublicScript = (name: string) =>
  readFileSync(resolve(process.cwd(), 'public', 'auto-config', name), 'utf8')

const readUseKeyModal = () =>
  readFileSync(resolve(process.cwd(), 'src', 'components', 'keys', 'UseKeyModal.vue'), 'utf8')

describe('client auto-config scripts', () => {
  it('reuses an existing Claude Code CLI on macOS and Linux', () => {
    const script = readPublicScript('install.sh')

    expect(script).toContain('SCRIPT_VERSION="0.5.2"')
    expect(script).toContain('EXISTING_CLAUDE_COMMAND="$(get_usable_client_command claude || true)"')
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('exchange_setup_ticket')
    expect(script).toContain('install_codex_app_if_requested')
    expect(script).toContain("item.arch === process.env.TARGET_ARCH || item.arch === 'universal'")
    expect(script).toContain("-name 'ChatGPT.app'")
    expect(script).toContain('${api_base_url}/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script.indexOf('exchange_setup_ticket\n')).toBeLessThan(script.indexOf('ensure_node_runtime\n'))
  })

  it('reuses an existing Claude Code CLI on Windows', () => {
    const script = readPublicScript('install.ps1')

    expect(script).toContain("$ScriptVersion = '0.5.2'")
    expect(script).toContain("Get-UsableClientCommand -CommandName 'claude'")
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('Exchange-SetupTicket')
    expect(script).toContain('Install-CodexAppIfRequested')
    expect(script).toContain('Add-AppxPackage -AppInstallerFile $AppInstallerPath')
    expect(script).toContain('Set-AppxPackageAutoUpdateSettings')
    expect(script).toContain('$ApiBaseUrl/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script.indexOf('Exchange-SetupTicket\n')).toBeLessThan(script.indexOf('Resolve-ClientInstallPlan\n'))
  })

  it('writes Claude settings without replacing unrelated JSON fields', () => {
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('ANTHROPIC_BASE_URL')
      expect(script).toContain('ANTHROPIC_AUTH_TOKEN')
      expect(script).toContain('CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY')
      expect(script).toContain('claude-opus-5')
      expect(script).toContain('effortLevel')
      expect(script).toContain('xhigh')
      expect(script).toContain('model = "gpt-5.6-sol"')
      expect(script).not.toContain('model = "gpt-5.6"')
    }
  })

  it('uses xhigh as the explicit Codex reasoning default on every platform', () => {
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('model_reasoning_effort = "xhigh"')
      expect(script).not.toContain('model_reasoning_effort = "high"')
    }
  })

  it('uses xhigh as the Claude Code default in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain('"model": "claude-opus-5"')
    expect(modal).toContain('"effortLevel": "xhigh"')
  })
})
