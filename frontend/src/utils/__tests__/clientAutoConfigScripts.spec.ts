import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { createRequire } from 'node:module'

import { describe, expect, it } from 'vitest'
import { clientAutoConfigVersion } from '@/generated/modelCatalog'

const readPublicScript = (name: string) =>
  readFileSync(resolve(process.cwd(), 'public', 'auto-config', name), 'utf8')

const readUseKeyModal = () =>
  readFileSync(resolve(process.cwd(), 'src', 'components', 'keys', 'UseKeyModal.vue'), 'utf8')

const nodeRequire = createRequire(import.meta.url)
const { DatabaseSync } = nodeRequire('node:sqlite') as {
  DatabaseSync: new (path: string) => {
    close: () => void
    exec: (sql: string) => void
    prepare: (sql: string) => {
      all: (...params: unknown[]) => Array<Record<string, unknown>>
      get: (...params: unknown[]) => Record<string, unknown> | undefined
      run: (...params: unknown[]) => unknown
    }
  }
}

describe('client auto-config scripts', () => {
  it('reuses an existing Claude Code CLI on macOS and Linux', () => {
    const script = readPublicScript('install.sh')

    expect(script).toContain(`SCRIPT_VERSION='${clientAutoConfigVersion}'`)
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
    expect(script).toContain(`$ScriptVersion = '${clientAutoConfigVersion}'`)
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
      expect(script).toMatch(/CatalogOpenAIDefaultModel|CATALOG_OPENAI_DEFAULT_MODEL/)
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
      'gpt-5.6',
      'gpt-5.5',
      'gpt-5.4'
    ])
    expect(models).not.toContain('gpt-5.6-luna')
    expect(models).not.toContain('gpt-5.4-mini')
    expect(models).not.toContain('gpt-5.3-codex-spark')
    expect(catalog.template).toBeUndefined()
    for (const model of catalog.models) {
      expect(model.base_instructions).toBeTruthy()
      expect(model.supports_reasoning_summaries).toBe(true)
      expect(model.visibility).toBe('list')
      expect(model.context_window).toBe(272000)
      expect(model.max_context_window).toBe(272000)
      expect(model.effective_context_window_percent).toBe(95)
      expect(model.auto_compact_token_limit).toBe(258000)
    }
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
      expect(script).toMatch(/model_context_window = (?:\$CatalogOpenAIContextWindow|\$\{CATALOG_OPENAI_CONTEXT_WINDOW\})/)
      expect(script).toMatch(/model_auto_compact_token_limit = (?:\$CatalogOpenAIAutoCompactTokenLimit|\$\{CATALOG_OPENAI_AUTO_COMPACT_TOKEN_LIMIT\})/)
      expect(script).toContain('gpt-5.3-codex-spark')
    }
  })

  it('uses xhigh as the Claude Code default in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain("claudeClientDefault?.id ?? 'claude-opus-5'")
    expect(modal).toContain('"effortLevel": "xhigh"')
  })

  it('shows the supported Codex catalog in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
    // The modal resolves the key's group models before building the catalog,
    // so enterprise groups see every routed model instead of the static list.
    // codex-auto-review (the auto-review feature target) is filtered out, and
    // the config default keeps legacy groups on gpt-5.5.
    expect(modal).toContain('resolveCodexModels(')
    expect(modal).toContain('ONE_CLICK_CODEX_EXCLUDED_MODELS')
    expect(modal).toContain("ONE_CLICK_CODEX_PREFERRED_DEFAULT = 'gpt-5.6-sol'")
    expect(modal).toContain('buildCodexModelCatalog(codexCatalogModels.value)')
    expect(modal).not.toContain("'gpt-5.3-codex-spark'")
  })

  it('installs and configures Grok Build with the native Responses model on macOS and Linux', () => {
    const script = readPublicScript('install.sh')
    expect(script).toContain('all|claude|codex|grok')
    expect(script).toContain("curl -fsSL https://x.ai/cli/install.sh | bash")
    expect(script).toContain('for (const profile of managedModels)')
    expect(script).toContain('`[model.${JSON.stringify(profile.id)}]`')
    expect(script).toContain('`description = ${JSON.stringify(profile.display_name)}`')
    expect(script).toContain("'api_backend = \"responses\"'")
    expect(script).toContain('`context_window = ${Number(profile.context_window)}`')
    expect(script).toContain('fs.renameSync(temporaryPath, path)')
    expect(script).toContain('open_cc_switch_if_requested')
    expect(script).toContain('import-grok-cc-switch-provider.cjs')
    expect(script).toContain('已将 Grok 分组导入官方 CC Switch')
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
    expect(script).toContain('foreach ($ModelProfile in $CatalogGrokManagedModels)')
    expect(script).toContain('$Lines.Add("[model.$(ConvertTo-TomlString $ModelProfile.Id)]")')
    expect(script).toContain('$Lines.Add("description = $(ConvertTo-TomlString $ModelProfile.DisplayName)")')
    expect(script).toContain("$Lines.Add('api_backend = \"responses\"')")
    expect(script).toContain('$Lines.Add("context_window = $($ModelProfile.ContextWindow)")')
    expect(script).toContain('[System.IO.File]::Replace($TemporaryPath, $GrokConfigPath, $ReplacementBackupPath)')
    expect(script).toContain('Open-CcSwitchIfRequested')
    expect(script).toContain('Invoke-GrokCcSwitchImporter')
    expect(script).toContain('已将 Grok 分组导入官方 CC Switch')
    expect(script).toContain("Test-ApiKeyReadiness -Label 'Grok Build' -ApiKey $script:GrokApiKey")
  })

  it('imports one neutral dual-model Grok Provider without changing other CC Switch providers', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-cc-switch-grok-'))
    const ccSwitchDir = join(fixture, '.cc-switch')
    const dbPath = join(ccSwitchDir, 'cc-switch.db')
    const settingsPath = join(ccSwitchDir, 'settings.json')
    const importerPath = resolve(process.cwd(), 'public', 'auto-config', 'import-grok-cc-switch-provider.cjs')
    const unrelatedConfig = JSON.stringify({ config: 'unrelated-secret-sentinel' })
    try {
      mkdirSync(ccSwitchDir, { recursive: true })
      writeFileSync(settingsPath, JSON.stringify({ currentProviderGrokbuild: 'existing-grok', keep: true }))
      const db = new DatabaseSync(dbPath)
      db.exec(`
        CREATE TABLE providers (
          id TEXT NOT NULL,
          app_type TEXT NOT NULL,
          name TEXT NOT NULL,
          settings_config TEXT NOT NULL,
          website_url TEXT,
          category TEXT,
          created_at INTEGER,
          sort_index INTEGER,
          notes TEXT,
          icon TEXT,
          icon_color TEXT,
          meta TEXT NOT NULL DEFAULT '{}',
          is_current BOOLEAN NOT NULL DEFAULT 0,
          in_failover_queue BOOLEAN NOT NULL DEFAULT 0,
          cost_multiplier TEXT NOT NULL DEFAULT '1.0',
          limit_daily_usd TEXT,
          limit_monthly_usd TEXT,
          provider_type TEXT,
          PRIMARY KEY (id, app_type)
        );
      `)
      db.prepare(`
        INSERT INTO providers (id, app_type, name, settings_config, created_at, sort_index, meta, is_current)
        VALUES (?, ?, ?, ?, ?, ?, '{}', ?)
      `).run('existing-grok', 'grokbuild', 'Existing Grok', unrelatedConfig, 1, 0, 1)
      db.prepare(`
        INSERT INTO providers (id, app_type, name, settings_config, created_at, sort_index, meta, is_current)
        VALUES (?, ?, ?, ?, ?, ?, '{}', ?)
      `).run('existing-claude', 'claude', 'Existing Claude', unrelatedConfig, 2, 0, 1)
      db.close()

      const env = {
        ...process.env,
        CC_SWITCH_DB: dbPath,
        CC_SWITCH_SETTINGS_PATH: settingsPath,
        GROK_PROVIDER_DEFAULT_MODEL: 'grok-4.6',
        GROK_PROVIDER_BASE_URL: 'https://api.example.com/v1',
        GROK_PROVIDER_API_KEY: 'test-owned-key',
        GROK_PROVIDER_MODELS_JSON: JSON.stringify([
          { id: 'grok-4.5', display_name: 'Grok 4.5', context_window: 500000 },
          { id: 'grok-4.6', display_name: 'Grok 4.6', context_window: 500000 }
        ])
      }
      const runImporter = () => JSON.parse(execFileSync(process.execPath, [
        '--no-warnings', importerPath
      ], { env, encoding: 'utf8' })) as { status: string }

      expect(runImporter().status).toBe('imported')
      const verify = new DatabaseSync(dbPath)
      const rows = verify.prepare(`
        SELECT id, app_type, name, settings_config, is_current
        FROM providers ORDER BY app_type, id
      `).all()
      verify.close()

      expect(rows).toHaveLength(3)
      expect(rows.find(row => row.id === 'existing-grok')?.settings_config).toBe(unrelatedConfig)
      expect(rows.find(row => row.id === 'existing-claude')?.settings_config).toBe(unrelatedConfig)
      expect(Number(rows.find(row => row.id === 'existing-grok')?.is_current)).toBe(0)
      expect(Number(rows.find(row => row.id === 'existing-claude')?.is_current)).toBe(1)
      const imported = rows.find(row => row.id === 'laoshirenai-grok-group')
      expect(imported?.name).toBe('Grok 分组')
      expect(imported?.name).not.toContain('4.6')
      expect(Number(imported?.is_current)).toBe(1)
      const config = JSON.parse(String(imported?.settings_config)).config as string
      expect(config).toContain('default = "grok-4.6"')
      expect(config.match(/\[model\."grok-4\.5"\]/g)).toHaveLength(1)
      expect(config.match(/\[model\."grok-4\.6"\]/g)).toHaveLength(1)
      expect(config).toContain('name = "Grok 4.5"')
      expect(config).toContain('description = "Grok 4.6"')
      expect(config).not.toContain('unrelated-secret-sentinel')

      const settings = JSON.parse(readFileSync(settingsPath, 'utf8'))
      expect(settings).toEqual({ currentProviderGrokbuild: 'laoshirenai-grok-group', keep: true })
      expect(runImporter().status).toBe('unchanged')
      expect(readdirSync(join(ccSwitchDir, 'backups', 'laoshirenai-grok'))).toHaveLength(1)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('resolves the CC Switch app-config directory override used by 3.19.2', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-cc-switch-path-'))
    const overrideDir = join(fixture, 'custom-cc-switch')
    const storePath = join(fixture, 'app_paths.json')
    const importerPath = resolve(process.cwd(), 'public', 'auto-config', 'import-grok-cc-switch-provider.cjs')
    try {
      mkdirSync(overrideDir, { recursive: true })
      writeFileSync(storePath, JSON.stringify({ app_config_dir_override: overrideDir }))
      const resolvedPath = execFileSync(process.execPath, [
        '--no-warnings', importerPath, '--print-db-path'
      ], {
        env: {
          ...process.env,
          CC_SWITCH_DB: '',
          CC_SWITCH_TEST_HOME: fixture,
          CC_SWITCH_APP_PATHS_STORE: storePath
        },
        encoding: 'utf8'
      }).trim()
      expect(resolvedPath).toBe(join(overrideDir, 'cc-switch.db'))
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('atomically writes both Grok models while preserving unrelated config on macOS and Linux', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-grok-config-'))
    const grokDir = join(fixture, '.grok')
    const configPath = join(grokDir, 'config.toml')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    const original = `[models]\ndefault = "unrelated"\n\n[preferences]\ntheme = "dark"\n\n[model."unrelated"]\nmodel = "unrelated"\nbase_url = "https://unrelated.example/v1"\napi_key = "keep-me"\n\n[model."grok-4.5"]\nname = "stale"\n`
    try {
      mkdirSync(grokDir, { recursive: true })
      writeFileSync(configPath, original)
      const runWriter = () => execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; GROK_API_KEY="test-owned-key"; write_grok_config',
        '_',
        installerPath
      ], {
        env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' },
        stdio: 'pipe'
      })

      runWriter()
      const first = readFileSync(configPath, 'utf8')
      expect(first).toContain('default = "grok-4.6"')
      expect(first).toContain('[preferences]\ntheme = "dark"')
      expect(first).toContain('[model."unrelated"]')
      expect(first).toContain('api_key = "keep-me"')
      expect(first.match(/\[model\."grok-4\.5"\]/g)).toHaveLength(1)
      expect(first.match(/\[model\."grok-4\.6"\]/g)).toHaveLength(1)
      expect(first).toContain('name = "Grok 4.5"')
      expect(first).toContain('description = "Grok 4.5"')
      expect(first).toContain('name = "Grok 4.6"')
      expect(first).toContain('description = "Grok 4.6"')
      expect(first).not.toContain('老实人AI')
      expect(statSync(configPath).mode & 0o777).toBe(0o600)
      expect(readFileSync(`${configPath}.bak`, 'utf8')).toBe(original)
      expect(readdirSync(grokDir).some(name => name.includes('.tmp.'))).toBe(false)

      runWriter()
      expect(readFileSync(configPath, 'utf8')).toBe(first)
      expect(readFileSync(`${configPath}.bak`, 'utf8')).toBe(original)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })
})
