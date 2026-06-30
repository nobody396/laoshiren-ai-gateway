#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import {
  SEO_DIR,
  P0_PATHS,
  todayISO,
  ensureDir,
  readJSON,
  writeJSON,
  writeText,
} from './seo_geo_lib.mjs'

const args = new Map()
for (let i = 2; i < process.argv.length; i += 1) {
  const item = process.argv[i]
  if (item.startsWith('--')) {
    const key = item.slice(2)
    const next = process.argv[i + 1]
    if (next && !next.startsWith('--')) { args.set(key, next); i += 1 } else args.set(key, '1')
  }
}

const date = args.get('date') || todayISO()
const apply = args.has('apply') || process.env.SEO_GEO_AUTO_APPLY === '1'
const decisionPath = args.get('decision') || path.join(SEO_DIR, 'weekly', `${date}-decision.json`)
const outDir = path.join(SEO_DIR, 'repairs')
ensureDir(outDir)

const root = process.cwd()
const docsContentDir = path.join(root, 'frontend/src/docs/content')
const docsConfigPath = path.join(root, 'frontend/src/docs/config.ts')
const decision = readJSON(decisionPath, { pages: [] })

const pageConfig = {
  '/docs/claude-code-china-guide': {
    slug: 'claude-code-china-guide',
    title: 'Claude Code 国内使用',
    intent: 'Claude Code 国内怎么用、登录失败、API Key、Base URL、走官方地址',
    checks: [
      ['Claude Code 国内怎么用最稳？', '先确认地区与账号合规，再把安装、认证、Base URL、API Key 和终端环境变量配对。只要后台有调用记录，说明请求已经进入老实人AI；没有记录先查本地变量和旧终端。'],
      ['Claude Code 登录失败还要反复 login 吗？', '不要反复登录。登录链路失败时，优先用 API Key 加 Claude 兼容 Base URL 跑通本地任务，再排查浏览器回跳和官方账号状态。'],
      ['为什么配置后还是走官方地址？', '本质是当前进程没有读到正确的 ANTHROPIC_BASE_URL，或被旧 shell、旧配置、代理、工具缓存覆盖。先打印当前终端变量，再开新终端验证。'],
    ],
    next: '创建一把低额度 API Key，配置 `ANTHROPIC_BASE_URL=https://api.laoshirenai.com` 和 `ANTHROPIC_AUTH_TOKEN`，跑一个只读任务后看后台调用记录。',
  },
  '/docs/codex-china-guide': {
    slug: 'codex-china-guide',
    title: 'Codex 国内使用',
    intent: 'Codex 国内怎么用、CLI、App、VS Code、ChatGPT 登录、API Key 模式',
    checks: [
      ['Codex 国内使用先看什么？', '先选入口：官方登录、API Key、自定义 Provider、App、CLI、VS Code、WSL 不是一件事。入口选错，后面 Base URL 和 auth.json 都会错。'],
      ['Codex ChatGPT 登录和 API Key 模式冲突吗？', '可以共存，但要明确当前 Provider。官方登录解决账号生态，API Key 模式解决本地模型调用、团队 Key 和成本记录。'],
      ['后台没有调用记录说明什么？', '说明请求大概率没有进入老实人AI。优先查 config.toml、auth.json、当前用户 home、WSL/Docker 隔离和旧环境变量覆盖。'],
    ],
    next: '如果目标是本地 CLI 稳定调用，先走 API Key + Base URL；如果目标是官方云端能力，再走官方登录路径。',
  },
  '/docs/codex-no-api-key-guide': {
    slug: 'codex-no-api-key-guide',
    title: 'Codex 免 API Key 使用',
    intent: 'Codex 免 API Key、ChatGPT 登录、是否免费、Plus 登录、API Key 边界',
    checks: [
      ['Codex 免 API Key 是不是免费？', '不是。免 API Key 只是不用手动创建 Platform API Key，仍然依赖官方账号、订阅、额度、产品权限和网络回跳。'],
      ['什么时候可以不填 API Key？', '当你走官方 ChatGPT/OpenAI 登录路径，并且账号拥有对应 Codex 能力时，通常不需要手动把 OPENAI_API_KEY 写入 auth.json。'],
      ['什么时候必须要 API Key？', '只要你要自定义 Base URL、走老实人AI后台记录、团队成本控制、WSL/Docker/服务器运行，就需要 API Key 或兼容服务 Key。'],
    ],
    next: '先判断目标是“官方完整生态”还是“本地 CLI 可控调用”。前者看官方账号权限，后者用 API Key 模式更清晰。',
  },
  '/docs/codex-custom-api-guide': {
    slug: 'codex-custom-api-guide',
    title: 'Codex 自定义 API 配置',
    intent: 'Codex 第三方 API、config.toml、auth.json、Base URL、Responses 模式、/v1',
    checks: [
      ['Codex 自定义 API 最容易错在哪里？', '错在把 SDK 的 /v1 写法、旧 openai_base_url 字段、官方登录态和 Provider 配置混在一起。Codex CLI 要看当前版本读取哪个字段。'],
      ['Base URL 到底要不要加 /v1？', 'Codex CLI + Responses 模式通常填根地址 `https://api.laoshirenai.com`；普通 OpenAI SDK 或明确要求 v1 的客户端才填 `/v1`。'],
      ['为什么一直要求登录或 401？', '说明当前没有启用 API Key Provider，或 auth.json 没被当前用户/WSL/容器读到，也可能 Key 填错、填成订单号或被删除。'],
    ],
    next: '先固定一套 `model_provider`、`base_url`、`wire_api`、`auth.json`，用只读任务验证后台调用记录，再扩展到大项目。',
  },
}

const relatedLinks = [
  ['Claude Code 国内使用指南', 'claude-code-china-guide'],
  ['Codex 国内使用指南', 'codex-china-guide'],
  ['Codex 免 API Key 使用指南', 'codex-no-api-key-guide'],
  ['Codex 自定义 API 配置教程', 'codex-custom-api-guide'],
  ['Base URL 填写总指南', 'base-url-guide'],
  ['常见 API 报错排查', 'common-api-errors'],
]

function pathsNeedingRepair() {
  const pages = Array.isArray(decision.pages) ? decision.pages : []
  const selected = new Set()
  for (const page of pages) {
    if (!P0_PATHS.includes(page.path)) continue
    const actions = page.actions || []
    if (actions.some((a) => ['copy', 'content', 'distribution', 'conversion', 'measurement', 'data'].includes(a.type))) {
      selected.add(page.path)
    }
  }
  return [...selected]
}

function repairBlock(config) {
  const faq = config.checks.map(([q, a]) => `### ${q}\n\n${a}`).join('\n\n')
  const links = relatedLinks
    .filter(([, slug]) => slug !== config.slug)
    .map(([label, slug]) => `- [${label}](${slug})`)
    .join('\n')
  return `<!-- seo-geo-auto:start -->\n## 搜索意图补强与下一步\n\n这一节由老实人AI SEO/GEO 闭环维护，用来覆盖用户真实搜索里的高频表达：${config.intent}。\n\n${faq}\n\n### 读完之后怎么验证？\n\n${config.next}\n\n### 相关高意图页面\n\n${links}\n<!-- seo-geo-auto:end -->`
}

function upsertBlock(markdown, block) {
  const pattern = /\n?<!-- seo-geo-auto:start -->[\s\S]*?<!-- seo-geo-auto:end -->/m
  if (pattern.test(markdown)) {
    const next = markdown.replace(pattern, `\n\n${block}`)
    return next.endsWith('\n') ? next : `${next}\n`
  }
  const trimmed = markdown.trimEnd()
  return `${trimmed}\n\n${block}\n`
}

function setDocsLastModified(value) {
  if (!fs.existsSync(docsConfigPath)) return false
  const current = fs.readFileSync(docsConfigPath, 'utf8')
  const next = current.replace(/export const docsLastModified = '[^']+'/, `export const docsLastModified = '${value}'`)
  if (next === current) return false
  if (apply) fs.writeFileSync(docsConfigPath, next, 'utf8')
  return true
}

const targetPaths = pathsNeedingRepair()
const changes = []
for (const pagePath of targetPaths) {
  const config = pageConfig[pagePath]
  if (!config) continue
  const file = path.join(docsContentDir, `${config.slug}.md`)
  if (!fs.existsSync(file)) {
    changes.push({ path: pagePath, file, action: 'missing_file', applied: false })
    continue
  }
  const before = fs.readFileSync(file, 'utf8')
  const after = upsertBlock(before, repairBlock(config))
  if (after !== before) {
    if (apply) fs.writeFileSync(file, after, 'utf8')
    changes.push({ path: pagePath, file, action: 'upsert_search_intent_block', applied: apply })
  }
}

if (changes.some((c) => c.applied)) {
  const modified = setDocsLastModified(date)
  if (modified) changes.push({ path: '/docs', file: docsConfigPath, action: 'update_docs_last_modified', applied: apply })
}

const summary = {
  date,
  decisionPath,
  apply,
  targetPaths,
  changedCount: changes.filter((c) => c.applied).length,
  changes,
  rule: 'Only P0 docs with decision actions are changed; no secrets, prices, credentials, or production configs are edited.',
}

writeJSON(path.join(outDir, `${date}.json`), summary)
writeText(path.join(outDir, `${date}.md`), [
  `# SEO/GEO 自动修复 ${date}`,
  '',
  `- 模式：${apply ? 'apply' : 'dry-run'}`,
  `- 目标页面：${targetPaths.length ? targetPaths.join(', ') : '无'}`,
  `- 已应用改动：${summary.changedCount}`,
  '',
  '## 改动',
  '',
  changes.length ? changes.map((c) => `- ${c.applied ? '[x]' : '[ ]'} ${c.path} ${c.action} (${path.relative(root, c.file)})`).join('\n') : '- 无',
  '',
].join('\n'))

console.log(path.join(outDir, `${date}.json`))
