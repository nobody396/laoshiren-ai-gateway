'use strict'
/* eslint-disable @typescript-eslint/no-var-requires, no-empty */

const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { DatabaseSync } = require('node:sqlite')

const APP_TYPE = 'grokbuild'
const DEFAULT_PROVIDER_ID = 'laoshirenai-grok-group'
const DEFAULT_PROVIDER_NAME = 'Grok 分组'

class MissingDatabaseError extends Error {}

function expandHome(value, homeDir) {
  if (value === '~') return homeDir
  if (value.startsWith('~/') || value.startsWith('~\\')) {
    return path.join(homeDir, value.slice(2))
  }
  return value
}

function readJsonObject(filePath, fallback = {}) {
  if (!fs.existsSync(filePath)) return fallback
  const value = JSON.parse(fs.readFileSync(filePath, 'utf8'))
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`JSON root must be an object: ${filePath}`)
  }
  return value
}

function resolveHomeDir() {
  return process.env.CC_SWITCH_TEST_HOME || os.homedir()
}

function resolveStoreCandidates(homeDir) {
  if (process.env.CC_SWITCH_APP_PATHS_STORE) {
    return [process.env.CC_SWITCH_APP_PATHS_STORE]
  }
  if (process.platform === 'darwin') {
    return [path.join(homeDir, 'Library', 'Application Support', 'com.ccswitch.desktop', 'app_paths.json')]
  }
  if (process.platform === 'win32') {
    const appData = process.env.APPDATA || path.join(homeDir, 'AppData', 'Roaming')
    return [path.join(appData, 'com.ccswitch.desktop', 'app_paths.json')]
  }
  const dataHome = process.env.XDG_DATA_HOME || path.join(homeDir, '.local', 'share')
  return [path.join(dataHome, 'com.ccswitch.desktop', 'app_paths.json')]
}

function resolveDatabasePath() {
  if (process.env.CC_SWITCH_DB) return path.resolve(expandHome(process.env.CC_SWITCH_DB, resolveHomeDir()))

  const homeDir = resolveHomeDir()
  for (const storePath of resolveStoreCandidates(homeDir)) {
    if (!fs.existsSync(storePath)) continue
    const store = readJsonObject(storePath)
    const override = typeof store.app_config_dir_override === 'string'
      ? store.app_config_dir_override.trim()
      : ''
    if (!override) continue
    const overrideDir = path.resolve(expandHome(override, homeDir))
    if (fs.existsSync(overrideDir)) return path.join(overrideDir, 'cc-switch.db')
  }

  const defaultPath = path.join(homeDir, '.cc-switch', 'cc-switch.db')
  if (process.platform === 'win32' && !fs.existsSync(defaultPath) && process.env.HOME) {
    const legacyPath = path.join(process.env.HOME, '.cc-switch', 'cc-switch.db')
    if (fs.existsSync(legacyPath)) return legacyPath
  }
  return defaultPath
}

function resolveSettingsPath() {
  if (process.env.CC_SWITCH_SETTINGS_PATH) {
    return path.resolve(expandHome(process.env.CC_SWITCH_SETTINGS_PATH, resolveHomeDir()))
  }
  // CC Switch 3.19.2 keeps device-local current-provider state in the default home.
  return path.join(resolveHomeDir(), '.cc-switch', 'settings.json')
}

function tomlString(value) {
  return JSON.stringify(String(value))
}

function buildProviderConfig() {
  const defaultModel = (process.env.GROK_PROVIDER_DEFAULT_MODEL || '').trim()
  const baseUrl = (process.env.GROK_PROVIDER_BASE_URL || '').trim().replace(/\/+$/, '')
  const apiKey = process.env.GROK_PROVIDER_API_KEY || ''
  const models = JSON.parse(process.env.GROK_PROVIDER_MODELS_JSON || '[]')

  if (!defaultModel || !baseUrl || !apiKey || !Array.isArray(models) || models.length < 2) {
    throw new Error('Grok Provider import input is incomplete')
  }
  if (!models.some((model) => model && model.id === defaultModel)) {
    throw new Error('Grok Provider default model is missing from the model catalog')
  }

  const seen = new Set()
  const lines = ['[models]', `default = ${tomlString(defaultModel)}`, '']
  for (const model of models) {
    const id = typeof model?.id === 'string' ? model.id.trim() : ''
    const displayName = typeof model?.display_name === 'string' ? model.display_name.trim() : ''
    const contextWindow = Number(model?.context_window)
    if (!id || !displayName || !Number.isSafeInteger(contextWindow) || contextWindow <= 0 || seen.has(id)) {
      throw new Error('Grok Provider model catalog is invalid')
    }
    seen.add(id)
    lines.push(
      `[model.${tomlString(id)}]`,
      `model = ${tomlString(id)}`,
      `base_url = ${tomlString(baseUrl)}`,
      `name = ${tomlString(displayName)}`,
      `description = ${tomlString(displayName)}`,
      `api_key = ${tomlString(apiKey)}`,
      'api_backend = "responses"',
      `context_window = ${contextWindow}`,
      ''
    )
  }
  return `${lines.join('\n')}\n`
}

function assertProviderSchema(db) {
  const required = new Set([
    'id', 'app_type', 'name', 'settings_config', 'website_url', 'category',
    'created_at', 'sort_index', 'notes', 'meta', 'is_current', 'in_failover_queue'
  ])
  for (const row of db.prepare('PRAGMA table_info(providers)').all()) required.delete(row.name)
  if (required.size) throw new Error(`Unsupported CC Switch providers schema: ${[...required].join(', ')}`)
}

function timestampForPath(date = new Date()) {
  return `${date.toISOString().replace(/[-:.]/g, '')}-${process.pid}`
}

function writeJsonAtomic(filePath, value) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true, mode: 0o700 })
  const temporaryPath = `${filePath}.tmp.${process.pid}.${Date.now()}`
  try {
    fs.writeFileSync(temporaryPath, `${JSON.stringify(value, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 })
    try { fs.chmodSync(temporaryPath, 0o600) } catch {}
    fs.renameSync(temporaryPath, filePath)
    try { fs.chmodSync(filePath, 0o600) } catch {}
  } finally {
    try { fs.unlinkSync(temporaryPath) } catch {}
  }
}

function restoreBackup(dbPath, dbBackupPath, settingsPath, settingsBackupPath, settingsExisted) {
  for (const suffix of ['-wal', '-shm']) {
    try { fs.unlinkSync(`${dbPath}${suffix}`) } catch {}
  }
  fs.copyFileSync(dbBackupPath, dbPath)
  if (settingsExisted) {
    fs.copyFileSync(settingsBackupPath, settingsPath)
  } else {
    try { fs.unlinkSync(settingsPath) } catch {}
  }
}

function importProvider() {
  const dbPath = resolveDatabasePath()
  const settingsPath = resolveSettingsPath()
  if (!fs.existsSync(dbPath)) throw new MissingDatabaseError(`CC Switch database not found: ${dbPath}`)

  const providerId = (process.env.GROK_PROVIDER_ID || DEFAULT_PROVIDER_ID).trim() || DEFAULT_PROVIDER_ID
  const providerName = (process.env.GROK_PROVIDER_NAME || DEFAULT_PROVIDER_NAME).trim() || DEFAULT_PROVIDER_NAME
  const config = buildProviderConfig()
  const settingsConfig = JSON.stringify({ config })
  const settings = readJsonObject(settingsPath)

  let db = new DatabaseSync(dbPath)
  assertProviderSchema(db)
  const existing = db.prepare(
    'SELECT name, settings_config, website_url, is_current FROM providers WHERE id = ? AND app_type = ?'
  ).get(providerId, APP_TYPE)
  const otherCurrent = db.prepare(
    'SELECT COUNT(*) AS count FROM providers WHERE app_type = ? AND id <> ? AND is_current = 1'
  ).get(APP_TYPE, providerId)
  const unchanged = existing &&
    existing.name === providerName &&
    existing.settings_config === settingsConfig &&
    existing.website_url === 'https://laoshirenai.com' &&
    Number(existing.is_current) === 1 &&
    Number(otherCurrent.count) === 0 &&
    settings.currentProviderGrokbuild === providerId

  if (unchanged) {
    db.close()
    return { status: 'unchanged', providerId, providerName, dbPath }
  }

  db.exec('PRAGMA wal_checkpoint(FULL)')
  db.close()
  db = null

  const backupDir = path.join(path.dirname(dbPath), 'backups', 'laoshirenai-grok', timestampForPath())
  fs.mkdirSync(backupDir, { recursive: true, mode: 0o700 })
  const dbBackupPath = path.join(backupDir, 'cc-switch.db')
  const settingsBackupPath = path.join(backupDir, 'settings.json')
  const settingsExisted = fs.existsSync(settingsPath)
  fs.copyFileSync(dbPath, dbBackupPath)
  if (settingsExisted) fs.copyFileSync(settingsPath, settingsBackupPath)

  try {
    db = new DatabaseSync(dbPath)
    db.exec('BEGIN IMMEDIATE')
    try {
      db.prepare('UPDATE providers SET is_current = 0 WHERE app_type = ?').run(APP_TYPE)
      db.prepare(`
        INSERT INTO providers (
          id, app_type, name, settings_config, website_url, category, created_at,
          sort_index, notes, meta, is_current, in_failover_queue
        ) VALUES (?, ?, ?, ?, ?, NULL, ?,
          (SELECT COALESCE(MAX(sort_index), -1) + 1 FROM providers WHERE app_type = ?),
          ?, '{}', 1, 0)
        ON CONFLICT(id, app_type) DO UPDATE SET
          name = excluded.name,
          settings_config = excluded.settings_config,
          website_url = excluded.website_url,
          category = NULL,
          notes = excluded.notes,
          is_current = 1,
          in_failover_queue = 0
      `).run(
        providerId,
        APP_TYPE,
        providerName,
        settingsConfig,
        'https://laoshirenai.com',
        Date.now(),
        APP_TYPE,
        '老实人 AI 一键配置，可随时在 CC Switch 中切换。'
      )
      db.exec('COMMIT')
    } catch (error) {
      try { db.exec('ROLLBACK') } catch {}
      throw error
    } finally {
      db.close()
      db = null
    }

    settings.currentProviderGrokbuild = providerId
    writeJsonAtomic(settingsPath, settings)
  } catch (error) {
    try { if (db) db.close() } catch {}
    restoreBackup(dbPath, dbBackupPath, settingsPath, settingsBackupPath, settingsExisted)
    throw error
  }

  return { status: 'imported', providerId, providerName, dbPath, backupDir }
}

if (process.argv.includes('--print-db-path')) {
  process.stdout.write(`${resolveDatabasePath()}\n`)
} else {
  try {
    const result = importProvider()
    process.stdout.write(`${JSON.stringify(result)}\n`)
  } catch (error) {
    if (error instanceof MissingDatabaseError) {
      process.stderr.write(`${error.message}\n`)
      process.exitCode = 2
    } else {
      process.stderr.write(`CC Switch Provider import failed: ${error.message}\n`)
      process.exitCode = 1
    }
  }
}
