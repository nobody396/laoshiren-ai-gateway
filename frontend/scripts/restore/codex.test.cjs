const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { restore } = require(process.env.CODEX_RESTORE_TEST_MODULE || '../../public/auto-config/restore-codex.cjs')
const managed = 'model_provider = "laoshirenai_responses"\nmodel = "group-only-model"\nmodel_catalog_json="laoshirenai-model-catalog.json"\npreferred_auth_method="apikey"\n[mcp_servers.keep]\ncommand="dummy"\n[model_providers.laoshirenai_responses]\nbase_url="https://api.laoshirenai.com"\n[model_providers.other]\nbase_url="https://example.invalid"\n'
function fixture(t, config = managed, auth = '{"OPENAI_API_KEY":"dummy-secret","tokens":{"access_token":"dummy-a","refresh_token":"dummy-r"},"other":"keep"}') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), '还原 Codex '))
  fs.writeFileSync(path.join(dir, 'config.toml'), config)
  if (auth !== null) fs.writeFileSync(path.join(dir, 'auth.json'), auth)
  fs.writeFileSync(path.join(dir, 'history.jsonl'), 'keep history')
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }))
  return dir
}
const read = (dir, name) => fs.readFileSync(path.join(dir, name), 'utf8')
test('backup, owned fields only, official tokens preserved, second run byte-identical', t => {
  const dir = fixture(t)
  const backup = restore(dir, {})
  assert.equal(read(backup, 'config.toml'), managed)
  assert.match(read(dir, 'config.toml'), /model_provider = "openai"/)
  assert.match(read(dir, 'config.toml'), /mcp_servers.keep/)
  assert.match(read(dir, 'config.toml'), /model_providers.other/)
  assert.doesNotMatch(read(dir, 'config.toml'), /laoshirenai|group-only-model|preferred_auth_method/)
  assert.deepEqual(JSON.parse(read(dir, 'auth.json')), { tokens: { access_token: 'dummy-a', refresh_token: 'dummy-r' }, other: 'keep', auth_mode: 'chatgpt' })
  assert.equal(read(dir, 'history.jsonl'), 'keep history')
  const files = fs.readdirSync(dir)
  const before = read(dir, 'config.toml')
  assert.equal(restore(dir, {}), null)
  assert.equal(read(dir, 'config.toml'), before)
  assert.deepEqual(fs.readdirSync(dir), files)
  if (process.platform !== 'win32') {
    assert.equal(fs.statSync(backup).mode & 0o777, 0o700)
    assert.equal(fs.statSync(path.join(backup, 'auth.json')).mode & 0o777, 0o600)
  }
})
for (const [name, config, auth, env] of [
  ['malformed TOML', 'model_provider = "dummy-secret', '{}', {}],
  ['malformed JSON', managed, '{dummy-secret', {}],
  ['array auth', managed, '[]', {}],
  ['null auth', managed, 'null', {}],
  ['third-party provider', 'model_provider="other"', '{}', {}],
  ['profile override', 'profile="work"\n' + managed, '{}', {}],
  ['official endpoint override', 'openai_base_url="https://example.invalid"\n' + managed, '{}', {}],
  ['environment override', managed, '{}', { OPENAI_BASE_URL: 'https://example.invalid' }],
]) test(`fail closed: ${name}`, t => {
  const dir = fixture(t, config, auth)
  assert.throws(() => restore(dir, env))
  assert.equal(read(dir, 'config.toml'), config)
  assert.equal(read(dir, 'auth.json'), auth)
  assert.equal(fs.readdirSync(dir).length, 3)
})
test('missing auth is safe and multiline values, quoted keys, arrays survive', t => {
  const dir = fixture(t, '"model_provider"="laoshirenai_responses"\nnotes="""hello\nworld"""\n[[items]]\nname="keep"\n[model_providers.laoshirenai_responses]\nbase_url="https://api.laoshirenai.com"\n', null)
  restore(dir, {})
  assert.match(read(dir, 'config.toml'), /world/)
  assert.match(read(dir, 'config.toml'), /\[\[items\]\]/)
  assert.equal(fs.existsSync(path.join(dir, 'auth.json')), false)
})
test('clean installation is a no-op', t => {
  const dir = fixture(t, '', null)
  assert.equal(restore(dir, {}), null)
  assert.equal(read(dir, 'config.toml'), '')
})

test('failed auth replacement rolls configuration back', t => {
  const dir = fixture(t)
  const rename = fs.renameSync
  let calls = 0
  fs.renameSync = (...args) => {
    if (++calls === 2) throw new Error('injected write failure')
    return rename(...args)
  }
  try { assert.throws(() => restore(dir, {}), /还原未完成/) } finally { fs.renameSync = rename }
  assert.equal(read(dir, 'config.toml'), managed)
  assert.match(read(dir, 'auth.json'), /dummy-secret/)
  assert.equal(fs.readdirSync(dir).filter(name => name.includes('.restore-')).length, 0)
})

test('CC Switch custom provider is restored only for our exact endpoint', t => {
  const config = managed.replaceAll('laoshirenai_responses', 'custom')
  const dir = fixture(t, config)
  restore(dir, {})
  assert.match(read(dir, 'config.toml'), /model_provider = "openai"/)
  assert.doesNotMatch(read(dir, 'config.toml'), /model_providers.custom/)
  for (const endpoint of ['https://third-party.invalid', 'https://api.laoshirenai.com.evil.invalid']) {
    const before = config.replace('https://api.laoshirenai.com', endpoint)
    const other = fixture(t, before)
    assert.throws(() => restore(other, {}))
    assert.equal(read(other, 'config.toml'), before)
  }
})
