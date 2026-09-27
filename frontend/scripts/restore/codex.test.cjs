const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawn } = require('node:child_process')
const http = require('node:http')
const { restore } = require(process.env.CODEX_RESTORE_TEST_MODULE || '../../public/auto-config/restore-codex.cjs')
const root = path.resolve(__dirname, '../..')
const bundle = fs.readFileSync(path.join(root, 'public/auto-config/restore-codex.cjs'))
const hash = require('node:crypto').createHash('sha256').update(bundle).digest('hex')
const source = fs.readFileSync(path.join(root, 'src/utils/codexRestore.ts'), 'utf8').replace(/^import .*\n/m, '').replace('export function', 'function').replace('windows: boolean): string', 'windows)')
const command = new Function('codexRestoreSha256', `${source}; return buildCodexRestoreCommand`)(hash)
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
  const dir = fixture(t, '"model_provider"="laoshirenai_responses"\nnotes="""hello\nworld"""\n[[items]]\nname="keep"\n', null)
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

// Execute the actual page-generated command, not a second handwritten installer.
for (const scenario of ['success', 'checksum', 'network', 'child failure']) {
  test(`copied command: ${scenario}`, async t => {
    const dir = fixture(t, scenario === 'child failure' ? 'model_provider="other"' : managed)
    const server = http.createServer((req, res) => {
      if (scenario === 'network') { res.writeHead(503); return res.end('unavailable') }
      res.end(scenario === 'checksum' ? 'throw Error("must not execute")' : bundle)
    })
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
    t.after(() => new Promise(resolve => server.close(resolve)))
    const windows = process.platform === 'win32'
    let line = command(windows).replace(/https:\/\/laoshirenai.com\/auto-config\/restore-codex.cjs/g, `http://127.0.0.1:${server.address().port}/restore`)
    const env = { ...process.env, CODEX_HOME: dir, HOME: dir, TMPDIR: dir, TEMP: dir, TMP: dir }
    for (const key of ['OPENAI_API_KEY', 'OPENAI_BASE_URL', 'CODEX_API_KEY']) delete env[key]
    if (windows) {
      fs.writeFileSync(path.join(dir, 'node.ps1'), 'throw "unsafe shim executed"')
      fs.writeFileSync(path.join(dir, 'node.cmd'), '@echo unsafe shim executed\r\nexit /b 99')
      env.PATH = dir + path.delimiter + env.PATH
      line = `Set-Variable HOME '${dir.replace(/'/g, "''")}' -Force; Set-ExecutionPolicy -Scope Process Restricted -Force; ${line}`
    }
    const shell = windows ? (process.env.RESTORE_TEST_SHELL || 'pwsh') : '/bin/bash'
    const args = windows ? ['-NoProfile', '-NonInteractive', '-Command', line] : ['-c', line]
    const result = await new Promise((resolve, reject) => {
      const child = spawn(shell, args, { env, timeout: 15000 })
      let output = ''
      child.stdout.on('data', chunk => { output += chunk })
      child.stderr.on('data', chunk => { output += chunk })
      child.on('error', reject)
      child.on('exit', code => resolve({ code, output }))
    })
    assert.doesNotMatch(result.output, /dummy-secret|unsafe shim executed/)
    if (scenario === 'success') {
      assert.equal(result.code, 0, result.output)
      assert.match(read(dir, 'config.toml'), /model_provider = "openai"/)
    } else {
      assert.notEqual(result.code, 0, result.output)
      assert.equal(read(dir, 'config.toml'), scenario === 'child failure' ? 'model_provider="other"' : managed)
      assert.doesNotMatch(result.output, /已切换/)
    }
    // Downloaded script must be removed on success and every failure path.
    assert.equal(fs.readdirSync(dir).filter(name => /^(tmp|tmp\.)/i.test(name)).length, 0)
  })
}

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
