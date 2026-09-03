import { chmodSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
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
    expect(script).toContain('data.model_id')
    expect(script).toContain('data.protocol')
    expect(script).toContain('SELECTED_MODEL="$received_model"')
    expect(script).toContain('CONFIG_PROTOCOL="$GROK_API_BACKEND"')
    expect(script).toContain('SELECTED_REASONING="${LAOSHIRENAI_REASONING_EFFORT:-}"')
    expect(script).toContain("const thinkingLevel = ['low', 'medium', 'high'].includes(requestedReasoning)")
    expect(script).toContain('install_codex_app_if_requested')
    expect(script).toContain('resolve_client_update_plan')
    expect(script).toContain('检测到 ${label} 可更新')
    expect(script).toContain("item.arch === process.env.TARGET_ARCH || item.arch === 'universal'")
    expect(script).toContain("-name 'ChatGPT.app'")
    expect(script).toContain('${api_base_url}/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script).toContain('回滚方法（仅显示本次存在的备份）')
    expect(script.indexOf('ensure_node_runtime\n')).toBeLessThan(script.indexOf('exchange_setup_ticket\n'))
  })

  it('reuses an existing Claude Code CLI on Windows', () => {
    const script = readPublicScript('install.ps1')

    expect(script.startsWith('\uFEFF')).toBe(true)
    expect(script).toContain(`$ScriptVersion = '${clientAutoConfigVersion}'`)
    expect(script).toContain("Get-UsableClientCommand -CommandName 'claude'")
    expect(script).toContain('检测到现有 Claude Code CLI，跳过重复安装')
    expect(script).toContain('Exchange-SetupTicket')
    expect(script).toContain('$Data.model_id')
    expect(script).toContain('$Data.protocol')
    expect(script).toContain('$script:SelectedModel = [string]$Data.model_id')
    expect(script).toContain('$script:GrokApiBackend = $script:SelectedProtocol')
    expect(script).toContain('$SelectedReasoning = if ($env:LAOSHIRENAI_REASONING_EFFORT)')
    expect(script).toContain("$ThinkingLevel = if ($SelectedReasoning -in @('low','medium','high'))")
    expect(script).toContain('Install-CodexAppIfRequested')
    expect(script).toContain('Resolve-ClientUpdatePlan')
    expect(script).toContain('Set-AppxPackageAutoUpdateSettings')
    expect(script).toContain('Add-AppxPackage -AppInstallerFile $AppInstallerPath')
    expect(script).toContain('$ApiBaseUrl/usage')
    expect(script).toContain('余额/套餐额度不足')
    expect(script).toContain('回滚方法（仅显示本次存在的备份）')
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
      expect(script).toContain('high')
      expect(script).toContain('modelSettings')
      expect(script).toContain('ANTHROPIC_DEFAULT_OPUS_MODEL')
      expect(script).toMatch(/CatalogOpenAIDefaultModel|CATALOG_OPENAI_DEFAULT_MODEL/)
    }
  })

  it('refuses malformed JSON instead of clearing the existing client config', () => {
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    for (const target of ['claude', 'gemini'] as const) {
      const fixture = mkdtempSync(join(tmpdir(), `laoshirenai-${target}-malformed-`))
      const configPath = target === 'claude'
        ? join(fixture, '.claude', 'settings.json')
        : join(fixture, '.gemini', 'settings.json')
      const malformed = '{not-json\n'
      try {
        mkdirSync(resolve(configPath, '..'), { recursive: true })
        writeFileSync(configPath, malformed)
        expect(() => execFileSync('bash', [
          '-c',
          target === 'claude'
            ? 'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; CLAUDE_API_KEY="test-key"; write_claude_config'
            : 'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; GEMINI_API_KEY="test-key"; write_gemini_config',
          '_', installerPath,
        ], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })).toThrow()
        expect(readFileSync(configPath, 'utf8')).toBe(malformed)
      } finally {
        rmSync(fixture, { recursive: true, force: true })
      }
    }
  })

  it('writes the selected Claude model, role slots, and only a supported high effort', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-claude-selection-'))
    const settingsPath = join(fixture, '.claude', 'settings.json')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      mkdirSync(resolve(settingsPath, '..'), { recursive: true })
      writeFileSync(settingsPath, JSON.stringify({ permissions: { allow: ['keep'] }, env: { KEEP_ME: 'yes' } }))
      const writeModel = (model: string) => execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; CLAUDE_API_KEY="test-key"; CATALOG_ANTHROPIC_DEFAULT_MODEL="$2"; write_claude_config',
        '_', installerPath, model,
      ], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })
      writeModel('claude-opus-5')
      let settings = JSON.parse(readFileSync(settingsPath, 'utf8'))
      expect(settings.model).toBe('claude-opus-5')
      expect(settings.modelSettings['claude-opus-5'].effortLevel).toBe('high')
      expect(settings.env.ANTHROPIC_DEFAULT_OPUS_MODEL).toBe('claude-opus-5')
      expect(settings.env.ANTHROPIC_DEFAULT_HAIKU_MODEL).toBe('claude-opus-5')
      expect(settings.permissions).toEqual({ allow: ['keep'] })
      expect(settings.env.KEEP_ME).toBe('yes')
      writeModel('claude-haiku-4-5')
      settings = JSON.parse(readFileSync(settingsPath, 'utf8'))
      expect(settings.model).toBe('claude-haiku-4-5')
      expect(settings.modelSettings['claude-haiku-4-5']).toBeUndefined()
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('derives the Codex reasoning default from the selected model and prefers high', () => {
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('supported_reasoning_levels')
      expect(script.includes("includes('high')") || script.includes("-contains 'high'")).toBe(true)
    }
  })

  it('writes a provider-owned Codex catalog without unsupported Spark', () => {
    const catalog = JSON.parse(readPublicScript('codex-model-catalog.json'))
    const models = catalog.models.map((model: { slug: string }) => model.slug)

    expect(models).toContain('gpt-5.6-sol')
    expect(models).toContain('qwen3.7-max')
    expect(models).toContain('deepseek-v4-pro-0813')
    expect(models).toContain('minimax-m3')
    expect(models).toContain('gpt-5.6-luna')
    expect(models).toContain('gpt-5.4-mini')
    expect(models).toContain('gpt-5.3-codex-spark')
    expect(catalog.template).toBeUndefined()
    for (const model of catalog.models) {
      expect(model.base_instructions).toBeTruthy()
      expect(model.supports_reasoning_summaries).toBe(true)
      expect(model.visibility).toBe('list')
      expect(model.context_window).toBeGreaterThan(0)
      expect(model.max_context_window).toBe(model.context_window)
      expect(model.effective_context_window_percent).toBe(95)
      expect(model.auto_compact_token_limit).toBeGreaterThan(0)
    }
    for (const name of ['install.sh', 'install.ps1']) {
      const script = readPublicScript(name)
      expect(script).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
      expect(script).toContain('model_context_window = ')
      expect(script).toContain('model_auto_compact_token_limit = ')
      expect(script).toContain('model_catalog_json = "laoshirenai-model-catalog.json"')
    }
  })

  it('filters the one-click Codex catalog to the API key group on macOS and Linux', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-codex-cyber-catalog-'))
    const sourcePath = join(fixture, 'source.json')
    const authorizedPath = join(fixture, 'authorized.json')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      writeFileSync(sourcePath, readPublicScript('codex-model-catalog.json'))
      writeFileSync(authorizedPath, JSON.stringify({
        data: [
          { id: 'gpt-5.6-sol' },
          { id: 'qwen3.7-max' }
        ]
      }))
      const selected = execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; filter_codex_model_catalog "$2" "$3"',
        '_',
        installerPath,
        sourcePath,
        authorizedPath
      ], {
        env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1', LAOSHIRENAI_MODEL_ID: 'qwen3.7-max' },
        encoding: 'utf8'
      })
      const catalog = JSON.parse(readFileSync(sourcePath, 'utf8'))
      expect(selected).toBe('qwen3.7-max')
      expect(catalog.models.map((model: { slug: string }) => model.slug)).toEqual([
        'gpt-5.6-sol',
        'qwen3.7-max'
      ])
      expect(catalog.models[1].display_name).toBe('Qwen 3.7 Max')
      expect(catalog.models[1].base_instructions).toBeTruthy()
      expect(catalog.models[1].visibility).toBe('list')
      expect(catalog.models[1].context_window).toBe(1000000)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('updates only Codex-owned TOML fields and preserves MCP and other providers', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-codex-merge-'))
    const codexDir = join(fixture, '.codex')
    const configPath = join(codexDir, 'config.toml')
    const catalogPath = join(codexDir, 'laoshirenai-model-catalog.json')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    const original = 'model = "old"\nnotify = ["keep"]\n\n[mcp_servers.keep]\ncommand = "keep-me"\n\n[model_providers.other]\nbase_url = "https://other.example"\n'
    try {
      mkdirSync(codexDir, { recursive: true })
      writeFileSync(configPath, original)
      writeFileSync(catalogPath, readPublicScript('codex-model-catalog.json'))
      const runWriter = () => execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; CATALOG_OPENAI_DEFAULT_MODEL="qwen3.7-max"; write_codex_config',
        '_', installerPath,
      ], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })
      runWriter()
      const first = readFileSync(configPath, 'utf8')
      expect(first).toContain('model = "qwen3.7-max"')
      expect(first).toContain('model_reasoning_effort = "high"')
      expect(first).toContain('model_context_window = 1000000')
      expect(first).toContain('notify = ["keep"]')
      expect(first).toContain('[mcp_servers.keep]\ncommand = "keep-me"')
      expect(first).toContain('[model_providers.other]\nbase_url = "https://other.example"')
      expect(first).toContain('[model_providers.laoshirenai_responses]')
      expect(readFileSync(`${configPath}.bak`, 'utf8')).toBe(original)
      runWriter()
      expect(readFileSync(configPath, 'utf8')).toBe(first)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('uses xhigh as the Claude Code default in the manual settings template', () => {
    const modal = readUseKeyModal()
    expect(modal).toContain('resolveClaudeClientModels(props.platform, props.defaultMappedModel)')
    expect(modal).not.toContain("claudeClientDefault?.id ?? 'claude-opus-5'")
    expect(modal).toContain("effortLevel: 'xhigh'")
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
    expect(script).toContain('all|claude|codex|grok|gemini')
    expect(script).toContain("curl -fsSL https://x.ai/cli/install.sh | bash")
    expect(script).toContain('for (const profile of managedModels)')
    expect(script).toContain('`[model.${JSON.stringify(profile.id)}]`')
    expect(script).toContain('`description = ${JSON.stringify(profile.display_name)}`')
    expect(script).toContain('`api_backend = ${JSON.stringify(protocol)}`')
    expect(script).toContain('`context_window = ${Number(profile.context_window)}`')
    expect(script).toContain('fs.renameSync(temporaryPath, path)')
    expect(script).toContain('open_cc_switch_if_requested')
    expect(script).toContain('import-grok-cc-switch-provider.cjs')
    expect(script).toContain('已将 Grok 分组导入官方 CC Switch')
    expect(script).toContain('verify_api_key_readiness "Grok Build" "$GROK_API_KEY"')
    expect(script).toContain('verify_selected_model_request')
    expect(script).toContain('${SELECTED_MODEL}:generateContent')
    expect(script).toContain("['claude', 'codex', 'grok', 'gemini'].includes(data.target)")
  })

  it('installs and configures Grok Build with the native Responses model on Windows', () => {
    const script = readPublicScript('install.ps1')
    expect(script).toContain("@('all', 'claude', 'codex', 'grok', 'gemini')")
    expect(script).toContain("$DefaultGrokBuildManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/grok-build/latest.json'")
    expect(script).toContain("$DefaultGrokBuildPackagePrefix = 'https://laoshirenai.com/downloads/grok-build/'")
    expect(script).toContain('-DownloadPrefix $script:GrokBuildPackagePrefix')
    expect(script).toContain('Download-VerifiedAsset -Asset $Asset -OutputPath $TemporaryPath')
    expect(script).not.toContain("$Bases = @('https://x.ai/cli'")
    expect(script).toContain('foreach ($ModelProfile in $CatalogGrokManagedModels)')
    expect(script).toContain('$Lines.Add("[model.$(ConvertTo-TomlString $ModelProfile.Id)]")')
    expect(script).toContain('$Lines.Add("description = $(ConvertTo-TomlString $ModelProfile.DisplayName)")')
    expect(script).toContain('$Lines.Add("api_backend = $(ConvertTo-TomlString $GrokProtocol)")')
    expect(script).toContain('$Lines.Add("context_window = $($ModelProfile.ContextWindow)")')
    expect(script).toContain('[System.IO.File]::Replace($TemporaryPath, $GrokConfigPath, $ReplacementBackupPath)')
    expect(script).toContain('Open-CcSwitchIfRequested')
    expect(script).toContain('Invoke-GrokCcSwitchImporter')
    expect(script).toContain('已将 Grok 分组导入官方 CC Switch')
    expect(script).toContain("Test-ApiKeyReadiness -Label 'Grok Build' -ApiKey $script:GrokApiKey")
    expect(script).toContain('function Test-SelectedModelRequest')
    expect(script).toContain('Test-SelectedModelRequest')
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

  it('applies manual page selections to the Grok model and protocol instead of defaults', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-grok-manual-'))
    const configPath = join(fixture, '.grok', 'config.toml')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; TOOLS="grok"; BASE_URL="https://api.example.com"; GROK_API_KEY="test-owned-key"; SELECTED_MODEL="grok-custom"; SELECTED_PROTOCOL="messages"; apply_manual_selection; write_grok_config',
        '_', installerPath,
      ], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })
      const config = readFileSync(configPath, 'utf8')
      expect(config).toContain('default = "grok-custom"')
      expect(config).toContain('api_backend = "messages"')
      expect(config).not.toContain('grok-4.6')
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('rejects an unsupported Grok manual protocol', () => {
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    expect(() => execFileSync('bash', [
      '-c',
      'source "$1"; NODE_BIN="$(command -v node)"; TOOLS="grok"; SELECTED_MODEL="grok-custom"; SELECTED_PROTOCOL="generate_content"; apply_manual_selection',
      '_', installerPath,
    ], { env: { ...process.env, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })).toThrow()
  })

  it('applies manual page selections to the Claude and Gemini default models', () => {
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    const claude = execFileSync('bash', [
      '-c',
      'source "$1"; TOOLS="claude"; SELECTED_MODEL="claude-custom"; apply_manual_selection; printf %s "$CATALOG_ANTHROPIC_DEFAULT_MODEL"',
      '_', installerPath,
    ], { env: { ...process.env, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' }).toString()
    expect(claude).toBe('claude-custom')
    const gemini = execFileSync('bash', [
      '-c',
      'source "$1"; TOOLS="gemini"; SELECTED_MODEL="gemini-custom"; apply_manual_selection; printf "%s|%s" "$CATALOG_GEMINI_DEFAULT_MODEL" "$CATALOG_GEMINI_MANAGED_MODELS"',
      '_', installerPath,
    ], { env: { ...process.env, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' }).toString()
    expect(gemini).toBe('gemini-custom|gemini-custom')
  })

  it('routes ticket and manual selection through mutually exclusive branches', () => {
    const script = readPublicScript('install.sh')
    expect(script).toContain('apply_manual_selection()')
    expect(script).toContain('responses|chat_completions|messages) GROK_API_BACKEND="$SELECTED_PROTOCOL" ;;')
    const mainBody = script.slice(script.indexOf('main() {'))
    expect(mainBody).toContain('if [ -n "$SETUP_TOKEN" ]; then')
    expect(mainBody.indexOf('exchange_setup_ticket')).toBeLessThan(mainBody.indexOf('apply_manual_selection'))
    const ps1 = readPublicScript('install.ps1')
    expect(ps1).toContain('function Apply-ManualSelection')
    expect(ps1).toContain("'responses', 'chat_completions', 'messages'")
    const psMain = ps1.slice(ps1.indexOf('function Main'))
    expect(psMain).toContain('if (-not [string]::IsNullOrWhiteSpace($script:SetupToken)) {')
    expect(psMain.indexOf('Exchange-SetupTicket')).toBeLessThan(psMain.indexOf('Apply-ManualSelection'))
  })

  it('writes the exact ticket-selected Grok model and protocol instead of a family default', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-grok-selection-'))
    const configPath = join(fixture, '.grok', 'config.toml')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      mkdirSync(resolve(configPath, '..'), { recursive: true })
      execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; GROK_API_KEY="test-owned-key"; CATALOG_GROK_DEFAULT_MODEL="claude-opus-5"; GROK_API_BACKEND="messages"; CATALOG_GROK_MANAGED_MODELS_JSON=\'[{"id":"claude-opus-5","display_name":"claude-opus-5","context_window":null}]\'; write_grok_config',
        '_', installerPath,
      ], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })
      const config = readFileSync(configPath, 'utf8')
      expect(config).toContain('default = "claude-opus-5"')
      expect(config).toContain('api_backend = "messages"')
      expect(config).not.toContain('context_window = 0')
      expect(config).not.toContain('grok-4.6')
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it.each(['codex', 'grok'])('refuses malformed %s TOML without replacing the file', (target) => {
    const fixture = mkdtempSync(join(tmpdir(), `laoshirenai-${target}-toml-`))
    const dir = join(fixture, `.${target}`)
    const configPath = join(dir, 'config.toml')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    const malformed = '[broken\nvalue = true\n'
    try {
      mkdirSync(dir, { recursive: true })
      writeFileSync(configPath, malformed)
      if (target === 'codex') writeFileSync(join(dir, 'laoshirenai-model-catalog.json'), readPublicScript('codex-model-catalog.json'))
      const command = target === 'codex'
        ? 'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; CATALOG_OPENAI_DEFAULT_MODEL="qwen3.7-max"; write_codex_config'
        : 'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; GROK_API_KEY="test-key"; write_grok_config'
      expect(() => execFileSync('bash', ['-c', command, '_', installerPath], { env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })).toThrow()
      expect(readFileSync(configPath, 'utf8')).toBe(malformed)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it.each([
    ['responses', { status: 'completed', output: [{ type: 'message' }] }],
    ['chat_completions', { choices: [{ message: { content: 'CONFIG_OK' }, finish_reason: 'stop' }] }],
    ['messages', { content: [{ type: 'text', text: 'CONFIG_OK' }], stop_reason: 'end_turn' }],
    ['generate_content', { candidates: [{ content: { parts: [{ text: 'CONFIG_OK' }] }, finishReason: 'STOP' }] }],
  ])('accepts one complete minimal %s response after configuration', (protocol, response) => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-minimal-request-'))
    const binDir = join(fixture, 'bin')
    const curlPath = join(binDir, 'curl')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      mkdirSync(binDir, { recursive: true })
      writeFileSync(curlPath, `#!/usr/bin/env bash\nout=''\nwhile [ $# -gt 0 ]; do if [ "$1" = '-o' ]; then out="$2"; shift 2; else shift; fi; done\nprintf '%s' '${JSON.stringify(response)}' > "$out"\nprintf '200'\n`)
      chmodSync(curlPath, 0o755)
      execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; TOOLS="grok"; GROK_API_KEY="test-key"; SELECTED_MODEL="test-model"; SELECTED_PROTOCOL="$2"; verify_selected_model_request',
        '_', installerPath, protocol,
      ], { env: { ...process.env, HOME: fixture, PATH: `${binDir}:${process.env.PATH}`, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it.each([
    ['failed status with empty output array', { status: 'failed', output: [] }],
    ['failed status without output', { status: 'failed' }],
    ['error envelope over HTTP 200', { error: { message: 'upstream rejected' } }],
  ])('rejects a minimal responses body with %s', (_label, response) => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-minimal-fail-'))
    const binDir = join(fixture, 'bin')
    const curlPath = join(binDir, 'curl')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    try {
      mkdirSync(binDir, { recursive: true })
      writeFileSync(curlPath, `#!/usr/bin/env bash\nout=''\nwhile [ $# -gt 0 ]; do if [ "$1" = '-o' ]; then out="$2"; shift 2; else shift; fi; done\nprintf '%s' '${JSON.stringify(response)}' > "$out"\nprintf '200'\n`)
      chmodSync(curlPath, 0o755)
      expect(() => execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; TOOLS="grok"; GROK_API_KEY="test-key"; SELECTED_MODEL="test-model"; SELECTED_PROTOCOL="responses"; verify_selected_model_request',
        '_', installerPath,
      ], { env: { ...process.env, HOME: fixture, PATH: `${binDir}:${process.env.PATH}`, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1' }, stdio: 'pipe' })).toThrow()
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })

  it('configures Gemini CLI through the gateway on macOS and Linux', () => {
    const script = readPublicScript('install.sh')
    expect(script).toContain('GEMINI_ENV_PATH="${GEMINI_DIR}/.env"')
    expect(script).toContain('GEMINI_SETTINGS_PATH="${GEMINI_DIR}/settings.json"')
    expect(script).toContain('write_gemini_config')
    expect(script).toContain('configure_gemini')
    expect(script).toContain('verify_gemini_api_key')
    expect(script).toContain('verify_api_key_readiness "Gemini CLI" "$GEMINI_API_KEY"')
    expect(script).toContain('GOOGLE_GEMINI_BASE_URL')
    expect(script).toContain("GOOGLE_GENAI_USE_VERTEXAI: 'false'")
    expect(script).toContain('GEMINI_MODEL')
    expect(script).toContain("config.security.auth.selectedType = 'gemini-api-key'")
    expect(script).toContain('config.model.name = model')
    expect(script).toContain('thinkingConfig: { thinkingLevel }')
    expect(script).toContain("CATALOG_GEMINI_DEFAULT_MODEL='gemini-3.7-flash'")
    expect(script).toContain("CATALOG_GEMINI_MANAGED_MODELS='gemini-3.1-pro gemini-3.7-flash gemini-3.7-flash-high gemini-3.8-flash'")
    expect(script).toContain('npm_install_with_fallback "@google/gemini-cli@latest"')
    expect(script).toContain('LAOSHIRENAI_GEMINI_API_KEY')
  })

  it('configures Gemini CLI through the gateway on Windows', () => {
    const script = readPublicScript('install.ps1')
    expect(script).toContain("$GeminiEnvPath = Join-Path $GeminiDir '.env'")
    expect(script).toContain("$GeminiSettingsPath = Join-Path $GeminiDir 'settings.json'")
    expect(script).toContain('function Write-GeminiConfig')
    expect(script).toContain('Configure-Gemini')
    expect(script).toContain('Test-GeminiApiKey')
    expect(script).toContain("Test-ApiKeyReadiness -Label 'Gemini CLI' -ApiKey $script:GeminiApiKey")
    expect(script).toContain('GOOGLE_GEMINI_BASE_URL')
    expect(script).toContain("GOOGLE_GENAI_USE_VERTEXAI = 'false'")
    expect(script).toContain("GEMINI_MODEL = $CatalogGeminiDefaultModel")
    expect(script).toContain("-NotePropertyName selectedType -NotePropertyValue 'gemini-api-key' -Force")
    expect(script).toContain('thinkingConfig = [pscustomobject]@{ thinkingLevel = $ThinkingLevel }')
    expect(script).toContain("$CatalogGeminiDefaultModel = 'gemini-3.7-flash'")
    expect(script).toContain("$CatalogGeminiManagedModels = @('gemini-3.1-pro', 'gemini-3.7-flash', 'gemini-3.7-flash-high', 'gemini-3.8-flash')")
    expect(script).toContain("Install-NpmPackageWithFallback -PackageName '@google/gemini-cli@latest'")
    expect(script).toContain('$Data.target -notin @(\'claude\', \'codex\', \'grok\', \'gemini\')')
    expect(script).toContain('$env:LAOSHIRENAI_GEMINI_API_KEY')
  })

  it('writes Gemini CLI .env and settings.json idempotently while preserving unrelated config', () => {
    const fixture = mkdtempSync(join(tmpdir(), 'laoshirenai-gemini-config-'))
    const geminiDir = join(fixture, '.gemini')
    const envPath = join(geminiDir, '.env')
    const settingsPath = join(geminiDir, 'settings.json')
    const installerPath = resolve(process.cwd(), 'public', 'auto-config', 'install.sh')
    const originalEnv = 'GEMINI_API_KEY=old-key\nFOO_BAR=keep\nGOOGLE_GENAI_USE_VERTEXAI=true\n'
    const originalSettings = JSON.stringify({
      security: { auth: { selectedType: 'oauth-personal', other: 1 } },
      model: { name: 'old-model', extra: true },
      modelConfigs: {
        overrides: [
          { match: { model: 'user-model' }, generateContentConfig: { temperature: 0.1 } },
          { match: { model: 'gemini-3.7-flash' }, generateContentConfig: { thinkingConfig: { thinkingLevel: 'LOW' } } }
        ]
      },
      theme: 'dark'
    })
    try {
      mkdirSync(geminiDir, { recursive: true })
      writeFileSync(envPath, originalEnv)
      writeFileSync(settingsPath, originalSettings)
      const runWriter = () => execFileSync('bash', [
        '-c',
        'source "$1"; NODE_BIN="$(command -v node)"; BASE_URL="https://api.example.com"; GEMINI_API_KEY="test-owned-key"; write_gemini_config',
        '_',
        installerPath
      ], {
        env: { ...process.env, HOME: fixture, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1', LAOSHIRENAI_REASONING_EFFORT: 'low' },
        stdio: 'pipe'
      })

      runWriter()
      const firstEnv = readFileSync(envPath, 'utf8')
      expect(firstEnv).toContain('GEMINI_API_KEY=test-owned-key')
      expect(firstEnv).toContain('FOO_BAR=keep')
      expect(firstEnv).toContain('GOOGLE_GEMINI_BASE_URL=https://api.example.com')
      expect(firstEnv).toContain('GOOGLE_GENAI_USE_VERTEXAI=false')
      expect(firstEnv).toContain('GEMINI_MODEL=gemini-3.7-flash')
      expect(firstEnv).not.toContain('old-key')
      expect(statSync(envPath).mode & 0o777).toBe(0o600)
      expect(readFileSync(`${envPath}.bak`, 'utf8')).toBe(originalEnv)

      const firstSettings = JSON.parse(readFileSync(settingsPath, 'utf8')) as {
        security: { auth: { selectedType: string; other: number } }
        model: { name: string; extra: boolean }
        modelConfigs: { overrides: Array<{ match: { model: string }; generateContentConfig: Record<string, unknown> }> }
        theme: string
      }
      expect(firstSettings.security.auth.selectedType).toBe('gemini-api-key')
      expect(firstSettings.security.auth.other).toBe(1)
      expect(firstSettings.model.name).toBe('gemini-3.7-flash')
      expect(firstSettings.model.extra).toBe(true)
      expect(firstSettings.theme).toBe('dark')
      const overrideModels = firstSettings.modelConfigs.overrides.map((entry) => entry.match.model)
      expect(overrideModels).toEqual([
        'user-model',
        'gemini-3.1-pro',
        'gemini-3.7-flash',
        'gemini-3.7-flash-high',
        'gemini-3.8-flash',
      ])
      for (const entry of firstSettings.modelConfigs.overrides.slice(1)) {
        expect(entry.generateContentConfig).toEqual({ thinkingConfig: { thinkingLevel: 'LOW' } })
      }
      expect(readFileSync(`${settingsPath}.bak`, 'utf8')).toBe(originalSettings)

      runWriter()
      expect(readFileSync(envPath, 'utf8')).toBe(firstEnv)
      expect(readFileSync(settingsPath, 'utf8')).toBe(JSON.stringify(firstSettings, null, 2) + '\n')
      expect(readFileSync(`${envPath}.bak`, 'utf8')).toBe(originalEnv)
      expect(readFileSync(`${settingsPath}.bak`, 'utf8')).toBe(originalSettings)
      expect(readdirSync(geminiDir).some(name => name.includes('.tmp.'))).toBe(false)
    } finally {
      rmSync(fixture, { recursive: true, force: true })
    }
  })
})
