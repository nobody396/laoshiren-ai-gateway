const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { restore } = require(process.env.CLAUDE_RESTORE_TEST_MODULE || '../../public/auto-config/restore-claude.cjs')
const managed = {
  model: 'gateway-model',
  env: { ANTHROPIC_BASE_URL: 'https://api.laoshirenai.com', ANTHROPIC_AUTH_TOKEN: 'dummy-secret',
    ANTHROPIC_MODEL: 'gateway-model', ANTHROPIC_DEFAULT_OPUS_MODEL: 'gateway-model',
    ANTHROPIC_DEFAULT_SONNET_MODEL: 'gateway-model', ANTHROPIC_DEFAULT_HAIKU_MODEL: 'gateway-model',
    ANTHROPIC_DEFAULT_FABLE_MODEL: 'gateway-fable', CLAUDE_CODE_ATTRIBUTION_HEADER: '0',
    CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1', CLAUDE_CODE_EFFORT_LEVEL: 'high', KEEP: 'yes' },
  modelSettings: { 'gateway-model': { effortLevel: 'high', keep: true }, other: { effortLevel: 'low' } },
  hooks: { keep: [] }, permissions: { allow: ['Read'] }, mcpServers: { keep: {} }, theme: 'dark'
}
function fixture(t, content = JSON.stringify(managed)) {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), '还原 Claude '))
  if (content !== null) fs.writeFileSync(path.join(home, 'settings.json'), content)
  for (const file of ['.credentials.json', 'claude.json', 'history.jsonl']) fs.writeFileSync(path.join(home, file), 'keep untouched')
  t.after(() => fs.rmSync(home, { recursive: true, force: true }))
  return home
}
const read = home => fs.readFileSync(path.join(home, 'settings.json'), 'utf8')
test('Claude: backs up, removes owned fields only, preserves login/MCP/hooks, second run is byte-identical', t => {
  const home = fixture(t)
  const before = read(home)
  const backup = restore(home, {})
  assert.equal(read(backup), before)
  assert.deepEqual(JSON.parse(read(home)), {
    env: { KEEP: 'yes' }, modelSettings: { 'gateway-model': { keep: true }, other: { effortLevel: 'low' } },
    hooks: { keep: [] }, permissions: { allow: ['Read'] }, mcpServers: { keep: {} }, theme: 'dark'
  })
  for (const file of ['.credentials.json', 'claude.json', 'history.jsonl']) assert.equal(fs.readFileSync(path.join(home, file), 'utf8'), 'keep untouched')
  const after = read(home)
  const files = fs.readdirSync(home)
  assert.equal(restore(home, {}), null)
  assert.equal(read(home), after)
  assert.deepEqual(fs.readdirSync(home), files)
  if (process.platform !== 'win32') {
    assert.equal(fs.statSync(backup).mode & 0o777, 0o700)
    assert.equal(fs.statSync(path.join(home, 'settings.json')).mode & 0o777, 0o600)
  }
})
for (const base of ['https://api.laoshirenai.com/v1/', 'https://api.laoshirenai.com/antigravity']) {
  test(`Claude: supports installer endpoint ${base}`, t => {
    const config = structuredClone(managed)
    config.env.ANTHROPIC_BASE_URL = base
    config.model = 'later-user-choice'
    const home = fixture(t, JSON.stringify(config))
    restore(home, {})
    assert.equal(JSON.parse(read(home)).model, 'later-user-choice')
  })
}
for (const [name, config, env] of [
  ['malformed JSON', '{"dummy-secret', {}], ['null', 'null', {}], ['array', '[]', {}],
  ['non-object env', '{"env":[]}', {}], ['non-object models', '{"modelSettings":[]}', {}],
  ['third-party endpoint', JSON.stringify({ env: { ANTHROPIC_BASE_URL: 'https://other.invalid' } }), {}],
  ['lookalike endpoint', JSON.stringify({ env: { ANTHROPIC_BASE_URL: 'https://api.laoshirenai.com.evil.invalid' } }), {}],
  ['helper', JSON.stringify({ ...managed, apiKeyHelper: 'keep-helper' }), {}],
  ['forced login', JSON.stringify({ ...managed, forceLoginMethod: 'gateway' }), {}],
  ['other API credential', JSON.stringify({ ...managed, env: { ...managed.env, ANTHROPIC_API_KEY: 'dummy-other' } }), {}],
  ['terminal token', JSON.stringify(managed), { ANTHROPIC_AUTH_TOKEN: 'dummy-override' }],
  ['cloud provider', JSON.stringify(managed), { CLAUDE_CODE_USE_BEDROCK: '1' }],
]) test(`Claude fail closed: ${name}`, t => {
  const home = fixture(t, config)
  assert.throws(() => restore(home, env), error => !error.message.includes('dummy-secret'))
  assert.equal(read(home), config)
  assert.equal(fs.readdirSync(home).length, 4)
})
test('Claude: missing and already-official configs are no-ops', t => {
  for (const content of [null, '{}', '{"env":{"KEEP":"yes"},"model":"user-model"}']) {
    const home = fixture(t, content)
    assert.equal(restore(home, {}), null)
    assert.equal(fs.readdirSync(home).length, content === null ? 3 : 4)
  }
})
test('Claude: failed atomic replacement preserves original and recoverable backup', t => {
  const home = fixture(t)
  const before = read(home)
  const rename = fs.renameSync
  fs.renameSync = () => { throw new Error('injected') }
  try { assert.throws(() => restore(home, {}), /还原未完成/) } finally { fs.renameSync = rename }
  assert.equal(read(home), before)
  const backup = fs.readdirSync(home).find(name => name.startsWith('before-official-'))
  assert.equal(read(path.join(home, backup)), before)
  assert.equal(fs.readdirSync(home).filter(name => name.includes('.restore-')).length, 0)
})
test('Claude: rejects symlink file without touching its target', { skip: process.platform === 'win32' }, t => {
  const home = fixture(t)
  fs.renameSync(path.join(home, 'settings.json'), path.join(home, 'target.json'))
  fs.symlinkSync('target.json', path.join(home, 'settings.json'))
  assert.throws(() => restore(home, {}), /符号链接/)
  assert.equal(fs.readFileSync(path.join(home, 'target.json'), 'utf8'), JSON.stringify(managed))
})
