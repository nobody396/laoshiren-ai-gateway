// Only undo the user settings written by our one-click/CC Switch integration.
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const modelKeys = ['ANTHROPIC_MODEL', 'ANTHROPIC_DEFAULT_OPUS_MODEL',
  'ANTHROPIC_DEFAULT_SONNET_MODEL', 'ANTHROPIC_DEFAULT_HAIKU_MODEL', 'ANTHROPIC_DEFAULT_FABLE_MODEL']
const credentialOverrides = ['ANTHROPIC_API_KEY', 'ANTHROPIC_AUTH_TOKEN', 'ANTHROPIC_BASE_URL',
  'CLAUDE_CODE_USE_BEDROCK', 'CLAUDE_CODE_USE_VERTEX', 'CLAUDE_CODE_USE_FOUNDRY',
  'ANTHROPIC_PROFILE', 'ANTHROPIC_FEDERATION_ACCOUNT', 'ANTHROPIC_FEDERATION_PROVIDER']
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value)

function restore(home = process.env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), '.claude'), env = process.env) {
  const file = path.join(home, 'settings.json')
  if (credentialOverrides.some(key => env[key])) {
    throw new Error('检测到终端鉴权或端点环境变量，请先移除覆盖设置后重试；未修改任何文件')
  }
  if (!fs.existsSync(file)) return null
  if (!fs.lstatSync(file).isFile()) throw new Error('配置文件不能是符号链接或目录，请手动检查')
  const before = fs.readFileSync(file, 'utf8')
  // Never surface parser messages, which may contain credentials.
  let config
  try { config = JSON.parse(before.replace(/^\uFEFF/, '')) } catch {
    throw new Error('Claude Code 配置解析失败，未修改任何文件，请检查 JSON 格式')
  }
  if (!object(config) || (config.env !== undefined && !object(config.env)) ||
      (config.modelSettings !== undefined && !object(config.modelSettings))) {
    throw new Error('Claude Code 配置、env 或 modelSettings 不是 JSON 对象，未修改任何文件')
  }
  const settings = config.env || {}
  const base = settings.ANTHROPIC_BASE_URL
  if (!base) return null
  // Exact origin and path, never substring-match a third-party endpoint.
  if (!['https://api.laoshirenai.com', 'https://api.laoshirenai.com/',
    'https://api.laoshirenai.com/v1', 'https://api.laoshirenai.com/v1/',
    'https://api.laoshirenai.com/antigravity', 'https://api.laoshirenai.com/antigravity/'].includes(base)) {
    throw new Error('当前未使用老实人AI Claude Code 端点，未修改任何文件；请检查 CLAUDE_CONFIG_DIR 或手动配置')
  }
  if (config.apiKeyHelper || config.forceLoginMethod || config.forceLoginOrgUUID || config.forceLoginGatewayUrl ||
      credentialOverrides.filter(key => !['ANTHROPIC_BASE_URL', 'ANTHROPIC_AUTH_TOKEN'].includes(key)).some(key => settings[key])) {
    throw new Error('检测到其他鉴权、云服务或组织登录设置，请先手动检查；未修改任何文件')
  }
  // Remove only selectors pointing at the gateway-selected models. Preserve
  // subsequently chosen models and unrelated per-model preferences.
  const models = new Set(modelKeys.map(key => settings[key]).filter(value => typeof value === 'string' && value))
  if (models.has(config.model)) delete config.model
  for (const model of models) {
    const entry = config.modelSettings?.[model]
    if (entry !== undefined && !object(entry)) throw new Error('模型设置不是 JSON 对象，未修改任何文件')
    if (entry) {
      delete entry.effortLevel
      if (!Object.keys(entry).length) delete config.modelSettings[model]
    }
  }
  if (config.modelSettings && !Object.keys(config.modelSettings).length) delete config.modelSettings
  for (const key of ['ANTHROPIC_BASE_URL', 'ANTHROPIC_AUTH_TOKEN', ...modelKeys,
    'CLAUDE_CODE_ATTRIBUTION_HEADER', 'CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY', 'CLAUDE_CODE_EFFORT_LEVEL']) delete settings[key]
  if (!Object.keys(settings).length) delete config.env
  const after = JSON.stringify(config, null, 2) + '\n'
  const backup = fs.mkdtempSync(path.join(home, 'before-official-'))
  fs.chmodSync(backup, 0o700)
  fs.writeFileSync(path.join(backup, 'settings.json'), before, { flag: 'wx', mode: 0o600 })
  const temp = `${file}.restore-${process.pid}`
  try {
    fs.writeFileSync(temp, after, { flag: 'wx', mode: 0o600 })
    if (fs.readFileSync(file, 'utf8') !== before) throw new Error('concurrent change')
    fs.renameSync(temp, file)
    if (fs.readFileSync(file, 'utf8') !== after) throw new Error('readback mismatch')
  } catch {
    throw new Error(`还原未完成，原文件备份保留在 ${backup}`)
  } finally {
    if (fs.existsSync(temp)) fs.unlinkSync(temp)
  }
  return backup
}
if (require.main === module) {
  try {
    const backup = restore()
    console.log(backup ? `已移除 Claude Code 用户设置中的本站接入配置，文件回读通过。备份：${backup}` : '没有需要还原的老实人AI配置，未修改任何文件。')
    console.log('请重启 Claude Code，必要时运行 /login 登录自己的官方账号，再用 /status 检查。官方登录及请求尚未验证。')
    console.log('会话、MCP、权限、Hooks 和官方登录文件未修改。项目/组织设置、启动参数与 CC Switch 仍可能覆盖配置。请勿分享备份文件。')
  } catch (error) {
    console.error(error.name === 'Error' ? error.message : '配置还原失败，请检查文件权限；未验证官方登录')
    process.exitCode = 1
  }
}
module.exports = { restore }
