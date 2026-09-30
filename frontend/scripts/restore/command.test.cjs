const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const http = require('node:http')
const { spawn } = require('node:child_process')
const root = path.resolve(__dirname, '../..')
const hashes = JSON.parse(fs.readFileSync(path.join(root, 'src/generated/clientRestoreIntegrity.ts'), 'utf8').match(/= (\{[\s\S]*?\}) as const/)[1])
// Execute the page's actual renderer, with only its TypeScript annotations removed.
const source = fs.readFileSync(path.join(root, 'src/utils/officialConfigRestore.ts'), 'utf8')
  .replace(/^import .*\r?\n/m, '').replace('export function', 'function')
  .replace("client: 'codex' | 'claude', windows: boolean): string", 'client, windows)')
const command = new Function('clientRestoreSha256', `${source}; return buildOfficialRestoreCommand`)(hashes)
const shells = process.platform === 'win32' ? [process.env.RESTORE_TEST_SHELL || 'pwsh']
  : ['/bin/bash', ...(process.platform === 'darwin' ? ['/bin/zsh'] : []), ...(process.env.RESTORE_TEST_SHELL ? [process.env.RESTORE_TEST_SHELL] : [])]
for (const client of ['codex', 'claude']) {
  const bundle = fs.readFileSync(path.join(root, `public/auto-config/restore-${client}.cjs`))
  assert.equal(require('node:crypto').createHash('sha256').update(bundle).digest('hex'), hashes[client])
  for (const shell of shells) for (const scenario of ['success', 'checksum', 'network', 'child failure']) {
    test(`copied ${client} command (${shell}): ${scenario}`, async t => {
      const home = fs.mkdtempSync(path.join(os.tmpdir(), '还原 command '))
      t.after(() => fs.rmSync(home, { recursive: true, force: true }))
      const configPath = path.join(home, client === 'codex' ? 'config.toml' : 'settings.json')
      const before = client === 'codex'
        ? `model_provider="${scenario === 'child failure' ? 'other' : 'laoshirenai_responses'}"\n[model_providers.laoshirenai_responses]\nbase_url="https://api.laoshirenai.com"\n`
        : JSON.stringify({ env: { ANTHROPIC_BASE_URL: scenario === 'child failure' ? 'https://other.invalid' : 'https://api.laoshirenai.com', ANTHROPIC_AUTH_TOKEN: 'dummy-secret' } })
      fs.writeFileSync(configPath, before)
      const server = http.createServer((req, res) => {
        if (scenario === 'network') { res.writeHead(503); return res.end('unavailable') }
        res.end(scenario === 'checksum' ? 'throw Error("must not execute")' : bundle)
      })
      await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
      t.after(() => new Promise(resolve => server.close(resolve)))
      const windows = /powershell|pwsh/.test(shell)
      let line = command(client, windows)
      assert.doesNotMatch(line, /\n|SETUP_TOKEN/)
      if (!(process.env.RESTORE_VERIFY_PRODUCTION === 'true' && scenario === 'success')) {
        line = line.replace(`https://laoshirenai.com/auto-config/restore-${client}.cjs`, `http://127.0.0.1:${server.address().port}/restore`)
      }
      const env = { ...process.env, CODEX_HOME: home, CLAUDE_CONFIG_DIR: home, HOME: home, TMPDIR: home, TEMP: home, TMP: home }
      for (const key of Object.keys(env)) if (/^(ANTHROPIC_|CLAUDE_CODE_USE_|OPENAI_|CODEX_API_KEY)/.test(key)) delete env[key]
      if (windows) {
        fs.writeFileSync(path.join(home, 'node.ps1'), 'throw "unsafe shim executed"')
        fs.writeFileSync(path.join(home, 'node.cmd'), '@echo unsafe shim executed\r\nexit /b 99')
        const pathKey = Object.keys(env).find(key => key.toLowerCase() === 'path') || 'Path'
        env[pathKey] = home + path.delimiter + (env[pathKey] || '')
        // macOS pwsh is only a parser/execution regression, not Windows proof.
        if (process.platform !== 'win32') {
          const nodeDir = path.join(home, '.laoshirenai/node/current')
          fs.mkdirSync(nodeDir, { recursive: true })
          fs.symlinkSync(process.execPath, path.join(nodeDir, 'node.exe'))
        }
        line = `Set-Variable HOME '${home.replace(/'/g, "''")}' -Force; ${process.platform === 'win32' ? 'Set-ExecutionPolicy -Scope Process Restricted -Force; ' : ''}${line}`
      }
      const args = windows ? ['-NoProfile', '-NonInteractive', '-Command', line] : ['-c', line]
      const result = await new Promise((resolve, reject) => {
        const child = spawn(shell, args, { env, timeout: 20000 })
        let output = ''
        child.stdout.on('data', chunk => { output += chunk })
        child.stderr.on('data', chunk => { output += chunk })
        child.on('error', reject)
        child.on('exit', code => resolve({ code, output }))
      })
      assert.doesNotMatch(result.output, /dummy-secret|unsafe shim executed/)
      if (scenario === 'success') {
        assert.equal(result.code, 0, result.output)
        assert.notEqual(fs.readFileSync(configPath, 'utf8'), before)
      } else {
        assert.notEqual(result.code, 0, result.output)
        assert.equal(fs.readFileSync(configPath, 'utf8'), before)
        assert.doesNotMatch(result.output, /已切换|已移除/)
      }
      assert.equal(fs.readdirSync(home).filter(name => /^(tmp|tmp\.)/i.test(name)).length, 0)
    })
  }
}
