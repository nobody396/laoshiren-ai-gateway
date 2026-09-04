import { execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { describe, expect, it } from 'vitest'
const script = resolve('public/auto-config/install.sh')
const os = process.platform === 'darwin' ? 'macos' : 'linux'
function fixture() {
 const home = mkdtempSync(join(tmpdir(), 'setup-plan-'))
 return { home, close: () => rmSync(home, { recursive: true, force: true }), run: (body: string, plan: unknown) => execFileSync('bash', ['-c', `source "$1"; NODE_BIN="$2"; SETUP_PLAN_JSON="$FIXTURE_PLAN"; ${body}`, 'fixture', script, process.execPath], { encoding: 'utf8', env: { HOME: home, PATH: process.env.PATH, LAOSHIRENAI_INSTALLER_SOURCE_ONLY: '1', FIXTURE_PLAN: JSON.stringify(plan) } }) }
}
const plan = (target: string, models: Array<{ id: string; protocol: string }>) => ({ target, os, available: true, client_version_key: 'cli:1.0.13', models, default_model: models[0].id })
describe('setup plan installer execution', () => {
 it('imports every Grok plan model, preserves unrelated entries, prunes old owned aliases and is idempotent', () => {
  const f = fixture()
  try {
   mkdirSync(join(f.home, '.grok'))
   writeFileSync(join(f.home, '.grok/config.toml'), '[model."user-model"]\nmodel="keep"\n[model."laoshirenai/retired"]\nmodel="retired"\n')
   const p = plan('grok', [{ id: 'gpt-5.4', protocol: 'responses' }, { id: 'claude-opus-5', protocol: 'messages' }])
   const command = 'TOOLS=grok; GROK_API_KEY=fixture-key; CATALOG_GROK_DEFAULT_MODEL=gpt-5.4; apply_setup_plan; write_grok_config'
   f.run(command, p)
   const first = readFileSync(join(f.home, '.grok/config.toml'), 'utf8')
   expect(first).toContain('[model."laoshirenai/gpt-5.4"]')
   expect(first).toContain('[model."laoshirenai/claude-opus-5"]')
   expect(first).toContain('model="keep"')
   expect(first).not.toContain('retired')
   expect(first).toContain('api_backend = "messages"')
   expect(first).toContain('base_url = "https://api.laoshirenai.com"')
   f.run(command, p)
   expect(readFileSync(join(f.home, '.grok/config.toml'), 'utf8')).toBe(first)
  } finally { f.close() }
 })
 it('rejects wrong OS, protocol, empty plan and command-shaped model before writing', () => {
  const f = fixture()
  try {
   const p = plan('codex', [{ id: 'gpt-5.4', protocol: 'responses' }])
   for (const bad of [{ ...p, os: 'windows' }, { ...p, models: [] }, { ...p, models: [{ id: '$(touch unsafe)', protocol: 'responses' }] }, { ...p, models: [{ id: 'gpt-5.4', protocol: 'messages' }] }]) {
    expect(() => f.run('TOOLS=codex; apply_setup_plan', bad)).toThrow()
   }
  } finally { f.close() }
 })
 it('does not query latest or update an already installed plan client', () => {
  const f = fixture()
  try {
   const output = f.run('TOOLS=codex; apply_setup_plan; check_client_update() { echo UNEXPECTED_UPDATE; }; resolve_client_update_plan; printf "version=%s" "$SETUP_CLIENT_VERSION"', plan('codex', [{ id: 'gpt-5.4', protocol: 'responses' }]))
   expect(output).toContain('version=1.0.13'); expect(output).not.toContain('UNEXPECTED_UPDATE')
  } finally { f.close() }
 })
 it('configures Gemini gateway auth and only the planned model overrides', () => {
  const f = fixture()
  try {
   f.run('TOOLS=gemini; GEMINI_API_KEY=fixture-key; CATALOG_GEMINI_DEFAULT_MODEL=gemini-3.7-flash; apply_setup_plan; write_gemini_config', plan('gemini', [{ id: 'gemini-3.7-flash', protocol: 'generate_content' }]))
   const config = JSON.parse(readFileSync(join(f.home, '.gemini/settings.json'), 'utf8'))
   expect(config.security.auth.selectedType).toBe('gateway')
   expect(config.modelConfigs.overrides).toHaveLength(1)
  } finally { f.close() }
 })
})
