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

    expect(script).toContain('SCRIPT_VERSION="0.7.6"')
    expect(script).toContain('EXISTING_CLAUDE_COMMAND="$(get_usable_client_command claude || true)"')
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('exchange_setup_ticket')
    expect(script).toContain('install_codex_app_if_requested')
    expect(script).toContain('resolve_client_update_plan')
    expect(script).toContain('检测到 ${label} 可更新')
    expect(script).toContain("item.arch === process.env.TARGET_ARCH || item.arch === 'universal'")
    expect(script).toContain("-name 'ChatGPT.app'")
    expect(script).toContain('${api_base_url}/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script.indexOf('ensure_node_runtime\n')).toBeLessThan(script.indexOf('exchange_setup_ticket\n'))
  })

  it('reuses an existing Claude Code CLI on Windows', () => {
    const script = readPublicScript('install.ps1')

    expect(script.startsWith('\uFEFF')).toBe(true)
    expect(script).toContain("$ScriptVersion = '0.7.6'")
    expect(script).toContain("Get-UsableClientCommand -CommandName 'claude'")
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('Exchange-SetupTicket')
    expect(script).toContain('Install-CodexAppIfRequested')
    expect(script).toContain('Resolve-ClientUpdatePlan')
    expect(script).toContain('Set-AppxPackageAutoUpdateSettings')
    expect(script).toContain('Add-AppxPackage -AppInstallerFile $AppInstallerPath')
    expect(script).toContain('$ApiBaseUrl/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script).toContain('Resolve-SystemNpmCmd')
    expect(script).toContain("Get-Command npm.cmd -CommandType Application")
    expect(script).toContain('$script:NpmCmd = Resolve-SystemNpmCmd -NodeCommand $NodeCommand')
    expect(script).toContain('throw "npm.cmd 执行失败，退出码: $LASTEXITCODE"')
    expect(script).toContain('https://laoshirenai.com/api/v1/public-downloads/git-for-windows/latest.json')
    expect(script).toContain('https://laoshirenai.com/downloads/git-for-windows/')
    expect(script).toContain('https://laoshirenai.com/api/v1/public-downloads/grok-build/latest.json')
    expect(script).toContain('https://laoshirenai.com/downloads/grok-build/')
    expect(script).toContain('Download-VerifiedAsset -Asset $Asset')
    expect(script).not.toContain("$Bases = @('https://x.ai/cli'")
    expect(script).toContain('SHASUMS256.txt')
    expect(script).toContain('Download-VerifiedFileWithFallback -OutputPath $ZipPath')
    expect(script).not.toContain('$script:NpmCmd = (Get-Command npm).Source')
    expect(script).toContain('function Remove-ManagedPowerShellShims')
    expect(script).toContain("Get-ChildItem -LiteralPath $Dir -Filter '*.ps1'")
    expect(script).toContain("[IO.Path]::ChangeExtension($_.FullName, '.cmd')")
    expect(script).toContain('Install-RequestedClients\n  Remove-ManagedPowerShellShims\n  Install-CodexAppIfRequested')
    expect(script.indexOf('Exchange-SetupTicket\n')).toBeLessThan(script.indexOf('Resolve-ClientInstallPlan\n'))
  })

  it('keeps every customer-facing Windows helper parseable by PowerShell 5.1', () => {
    for (const name of [
      'install.ps1',
      'diagnose-cc-switch.ps1',
      'save-openai-official-provider.ps1'
    ]) {
      expect(readPublicScript(name).startsWith('\uFEFF')).toBe(true)
    }
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

  it('writes a provider-owned Codex catalog without unsupported Spark', () => {
    const catalog = JSON.parse(readPublicScript('codex-model-catalog.json'))
    const models = catalog.models.map((model: { slug: string }) => model.slug)

    expect(models).toEqual([
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna',
      'gpt-5.6',
      'gpt-5.5',
      'gpt-5.4',
      'gpt-5.4-mini'
    ])
    expect(models).not.toContain('gpt-5.3-codex-spark')
    expect(catalog.template).toBeUndefined()
    for (const model of catalog.models) {
      expect(model.base_instructions).toBeTruthy()
      expect(model.supports_reasoning_summaries).toBe(true)
      expect(model.visibility).toBe('list')
      expect(model.context_window).toBe(250000)
      expect(model.max_context_window).toBe(250000)
      expect(model.auto_compact_token_limit).toBe(225000)
    }
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
      expect(script).toContain('model_context_window = 250000')
      expect(script).toContain('model_auto_compact_token_limit = 225000')
      expect(script).toContain('gpt-5.3-codex-spark')
    }
  })

  it('uses xhigh as the Claude Code default in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain('"model": "claude-opus-5"')
    expect(modal).toContain('"effortLevel": "xhigh"')
  })

  it('shows the supported Codex catalog in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
    expect(modal).toContain('buildCodexModelCatalog()')
    expect(modal).not.toContain("'gpt-5.3-codex-spark'")
  })

  it('installs and configures Grok Build with the native Responses model on macOS and Linux', () => {
    const script = readPublicScript('install.sh')
    expect(script).toContain('all|claude|codex|grok')
    expect(script).toContain("curl -fsSL https://x.ai/cli/install.sh | bash")
    expect(script).toContain("'[model.\"grok-4.6\"]'")
    expect(script).toContain("'description = \"Grok 4.6\"'")
    expect(script).toContain("'api_backend = \"responses\"'")
    expect(script).toContain("'context_window = 500000'")
    expect(script).toContain('verify_api_key_readiness "Grok Build" "$GROK_API_KEY"')
    expect(script).toContain("['claude', 'codex', 'grok'].includes(data.target)")
  })

  it('installs and configures Grok Build with the native Responses model on Windows', () => {
    const script = readPublicScript('install.ps1')
    expect(script).toContain("@('all', 'claude', 'codex', 'grok')")
    expect(script).toContain("$DefaultGrokBuildManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/grok-build/latest.json'")
    expect(script).toContain("$DefaultGrokBuildPackagePrefix = 'https://laoshirenai.com/downloads/grok-build/'")
    expect(script).toContain('-DownloadPrefix $script:GrokBuildPackagePrefix')
    expect(script).toContain('Download-VerifiedAsset -Asset $Asset -OutputPath $TemporaryPath')
    expect(script).not.toContain("$Bases = @('https://x.ai/cli'")
    expect(script).toContain("$Lines.Add('[model.\"grok-4.6\"]')")
    expect(script).toContain("$Lines.Add('description = \"Grok 4.6\"')")
    expect(script).toContain("$Lines.Add('api_backend = \"responses\"')")
    expect(script).toContain("$Lines.Add('context_window = 500000')")
    expect(script).toContain("Test-ApiKeyReadiness -Label 'Grok Build' -ApiKey $script:GrokApiKey")
  })
})
