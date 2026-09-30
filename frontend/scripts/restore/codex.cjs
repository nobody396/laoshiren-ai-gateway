// Bundled as a standalone script: customers need Node, not npm or Python.
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const { parse, stringify } = require('smol-toml')

const provider = 'laoshirenai_responses'
const owned = ['model_provider', 'model', 'review_model', 'model_reasoning_effort',
  'model_catalog_json', 'disable_response_storage', 'network_access',
  'preferred_auth_method', 'model_context_window', 'model_auto_compact_token_limit']

function restore(home = process.env.CODEX_HOME || path.join(os.homedir(), '.codex'), env = process.env) {
  const configPath = path.join(home, 'config.toml')
  const authPath = path.join(home, 'auth.json')
  function read(file) {
    if (!fs.existsSync(file)) return null
    if (!fs.lstatSync(file).isFile()) throw new Error('配置文件不能是符号链接或目录，请手动检查')
    return fs.readFileSync(file, 'utf8')
  }
  const original = read(configPath)
  const config = parse((original || '').replace(/^\uFEFF/, ''))
  // Do not silently promise official routing when a higher-priority override exists.
  if (config.profile || config.openai_base_url || config.chatgpt_base_url ||
      config.model_providers?.openai || config.forced_login_method || config.forced_chatgpt_workspace_id ||
      ['OPENAI_BASE_URL', 'OPENAI_API_KEY', 'CODEX_API_KEY'].some(key => env[key])) {
    throw new Error('检测到 profile、官方端点或登录覆盖设置，请先手动检查；未修改任何文件')
  }
  if ((!config.model_provider || config.model_provider === 'openai') && !config.model_providers?.[provider]) return null
  const activeProvider = config.model_provider
  // CC Switch uses "custom" instead of the one-click provider ID. Only accept
  // that generic name when its endpoint is exactly ours.
  const endpoint = config.model_providers?.[activeProvider]?.base_url
  const gatewayEndpoint = ['https://api.laoshirenai.com', 'https://api.laoshirenai.com/',
    'https://api.laoshirenai.com/v1', 'https://api.laoshirenai.com/v1/'].includes(endpoint)
  if (![provider, 'custom'].includes(activeProvider) || !gatewayEndpoint) {
    throw new Error('当前未使用老实人AI一键配置的 Codex provider，未修改任何文件；请检查 CODEX_HOME 或手动配置')
  }
  const authOriginal = read(authPath)
  const auth = authOriginal === null ? null : JSON.parse(authOriginal.replace(/^\uFEFF/, ''))
  if (auth !== null && (typeof auth !== 'object' || Array.isArray(auth))) {
    throw new Error('登录文件不是 JSON 对象，未修改任何文件')
  }
  if (authOriginal !== null && auth === null) throw new Error('登录文件不能为 null')
  for (const key of owned) delete config[key]
  delete config.model_providers?.[activeProvider]
  if (config.model_providers && Object.keys(config.model_providers).length === 0) delete config.model_providers
  // Explicitly choose the built-in provider; no hard-coded model or subscription.
  config.model_provider = 'openai'
  const output = stringify(config)
  if (parse(output).model_provider !== 'openai') throw new Error('配置校验失败')
  if (auth) {
    delete auth.OPENAI_API_KEY
    if (auth.auth_mode === 'apikey' || auth.auth_mode === 'api') delete auth.auth_mode
    if (auth.tokens?.access_token && auth.tokens?.refresh_token) auth.auth_mode = 'chatgpt'
  }
  const changes = [[configPath, original, output]]
  if (auth) {
    const outputAuth = JSON.stringify(auth, null, 2) + '\n'
    if (outputAuth !== authOriginal) changes.push([authPath, authOriginal, outputAuth])
  }
  // All parsing/preflight precedes mutation. Backups never overwrite old backups.
  const backup = fs.mkdtempSync(path.join(home, 'before-official-'))
  fs.chmodSync(backup, 0o700)
  for (const [file, before] of changes) {
    fs.writeFileSync(path.join(backup, path.basename(file)), before, { flag: 'wx', mode: 0o600 })
  }
  function atomicWrite(file, content) {
    const temp = `${file}.restore-${process.pid}`
    try {
      fs.writeFileSync(temp, content, { flag: 'wx', mode: 0o600 })
      fs.renameSync(temp, file)
    } finally {
      if (fs.existsSync(temp)) fs.unlinkSync(temp)
    }
  }
  const written = []
  try {
    for (const [file, before, after] of changes) {
      if (fs.readFileSync(file, 'utf8') !== before) throw new Error('文件被其他程序修改，请退出 Codex 后重试')
      atomicWrite(file, after)
      written.push([file, before, after])
    }
    for (const [file, , after] of changes) {
      if (fs.readFileSync(file, 'utf8') !== after) throw new Error('回读校验失败')
    }
  } catch (error) {
    for (const [file, before, after] of written.reverse()) {
      if (fs.readFileSync(file, 'utf8') === after) atomicWrite(file, before)
    }
    throw new Error(`还原未完成，原文件备份保留在 ${backup}`)
  }
  return backup
}

if (require.main === module) {
  try {
    const backup = restore()
    console.log(backup ? `已切换为 OpenAI 官方 provider，配置文件回读通过。备份：${backup}` : '没有需要还原的老实人AI配置，未修改任何文件。')
    console.log('请完全退出并重启 Codex，必要时运行 codex login 或在 App 中选择 Sign in with ChatGPT。官方登录及请求尚未验证。')
    console.log('会话、MCP、其他 provider 和模型目录文件未删除。请勿分享备份文件；命令行 --profile/-c 和 CC Switch 可能再次覆盖配置。')
  } catch (error) {
    // Never echo parser errors: malformed input may contain credentials.
    console.error(error.name === 'Error' ? error.message : '配置解析失败，未修改任何文件，请检查 TOML/JSON 格式')
    process.exitCode = 1
  }
}
module.exports = { restore }
