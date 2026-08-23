import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { Marked } from 'marked'

const siteOrigin = 'https://laoshirenai.com'
const siteName = '老实人AI'
const ogImage = '/og-image.png'
const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendRoot = resolve(scriptDir, '..')
const contentDir = resolve(frontendRoot, 'src/docs/content')
const configPath = resolve(frontendRoot, 'src/docs/config.ts')
const publicDir = resolve(frontendRoot, 'public')
const configSource = readFileSync(configPath, 'utf8')
const docsLastModified = matchSingle(configSource, /docsLastModified\s*=\s*'([^']+)'/) || new Date().toISOString().slice(0, 10)

const marked = new Marked({
  breaks: true,
  gfm: true,
})

const homeRoute = {
  path: '/',
  title: '老实人AI - AI 编码网关',
  description: '老实人AI 提供面向开发者的 AI 编码接口与网关服务，支持 Claude Code、Codex、ChatGPT、Grok 等主流编码模型，适合快速配置、精确计费和稳定调用。',
  priority: 1,
  changefreq: 'weekly',
  ogType: 'website',
  schemaType: 'WebSite',
  dateModified: docsLastModified,
  staticHtml: `
    <main class="seo-static-content">
      <h1>老实人AI - AI 编码网关</h1>
      <p>老实人AI 为开发者和企业团队提供 Claude Code、Codex、ChatGPT、Grok 等 AI 编码模型的统一 API 接入、Key 管理、用量统计和成本控制。</p>
      <p>如果你正在搜索 Claude Code 国内使用、Codex 国内配置、Codex 免 API Key 登录或 Codex 自定义 API，请优先阅读文档中心的高意图指南。</p>
      <nav aria-label="核心页面">
        <ul>
          <li><a href="/docs/claude-code-china-guide">Claude Code 国内使用指南</a></li>
          <li><a href="/docs/codex-china-guide">Codex 国内使用指南</a></li>
          <li><a href="/docs/codex-no-api-key-guide">Codex 免 API Key 使用指南</a></li>
          <li><a href="/docs/codex-custom-api-guide">Codex 自定义 API 配置教程</a></li>
          <li><a href="/enterprise">企业 AI API 网关方案</a></li>
          <li><a href="/docs">文档中心</a></li>
        </ul>
      </nav>
    </main>`,
}

const publicRouteOverrides = new Map([
  ['/enterprise', { slug: 'enterprise-ai-api-gateway', title: '企业 AI API 网关 - 老实人AI', priority: 0.95, changefreq: 'weekly', ogType: 'website', schemaType: 'SoftwareApplication' }],
  ['/security', { slug: 'security', title: '安全与隐私 - 老实人AI', priority: 0.8, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage' }],
  ['/status', {
    title: '服务状态 - 老实人AI',
    description: '查看老实人AI OpenAI / Codex、Claude、Grok、Gemini 与 Builder Pass 的公开服务可用性、更新时间和受影响范围。',
    priority: 0.75,
    changefreq: 'daily',
    ogType: 'website',
    schemaType: 'WebPage',
    staticHtml: `
      <main class="seo-static-content">
        <h1>老实人AI 服务状态</h1>
        <p>这里展示 OpenAI / Codex、Claude、Grok、Gemini 与 Builder Pass 的公开服务可用性、最近更新时间和受影响范围。</p>
        <p>实时状态不可用或尚未公开时，页面会保留服务排查与支持说明，不会把缺少证据误报为运行正常。</p>
        <nav aria-label="服务状态相关页面">
          <ul>
            <li><a href="/status">查看实时服务状态</a></li>
            <li><a href="/docs">查看接入与排查文档</a></li>
          </ul>
        </nav>
      </main>`,
  }],
  ['/legal/terms', { slug: 'legal-terms', title: '服务条款 - 老实人AI', description: '老实人AI 服务条款，说明账号、API Key、计费、上游服务、地区声明、责任边界和条款更新规则。', priority: 0.7, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage' }],
  ['/legal/usage-policy', { slug: 'legal-usage-policy', title: '使用政策 - 老实人AI', description: '老实人AI 使用政策，说明禁止行为、安全边界、隐私保护、高风险使用、下游用户管理和违规处理规则。', priority: 0.7, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage' }],
  ['/legal/supported-regions', { slug: 'legal-supported-regions', title: '支持的国家和地区 - 老实人AI', description: '老实人AI 支持的国家和地区说明，明确中国大陆地区不支持使用以及地区、制裁、出口管制和上游政策限制。', priority: 0.7, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage' }],
  ['/legal/service-specific-terms', { slug: 'legal-service-specific-terms', title: '服务特定条款 - 老实人AI', description: '老实人AI 服务特定条款，说明模型接入、AI 编码工具、上游凭证、计费、文件数据、Beta 能力和企业管理员责任。', priority: 0.7, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage' }],
  ['/legal/affiliate-program', { slug: 'legal-affiliate-program', title: '联盟计划规则 - 老实人AI', description: '老实人AI联盟计划规则，说明普通邀请奖励、合伙人门槛、10% 奖励池、直属关系、提现、冲正和风险处理。', priority: 0.7, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage', lastModified: '2026-07-30' }],
  ['/legal/affiliate-payment-privacy', { slug: 'legal-affiliate-payment-privacy', title: '合伙人收款资料隐私告知 - 老实人AI', description: '老实人AI合伙人收款资料隐私告知，说明支付宝收款资料的处理目的、范围、保存期限、安全措施和用户权利。', priority: 0.65, changefreq: 'monthly', ogType: 'website', schemaType: 'WebPage', lastModified: '2026-07-30' }],
])

const docs = parseDocItems(configSource)
const docsBySlug = new Map(docs.map((doc) => [doc.slug, doc]))

const routes = [
  homeRoute,
  {
    path: '/changelog',
    title: `更新日志 - ${siteName}`,
    description: '查看老实人AI 已经做成的新功能、模型与配置更新、体验改进和问题修复。持续 Build in Public，让产品进展保持公开透明。',
    priority: 0.85,
    changefreq: 'weekly',
    ogType: 'website',
    schemaType: 'CollectionPage',
    dateModified: docsLastModified,
    staticHtml: `
      <main class="seo-static-content">
        <h1>老实人AI 更新日志</h1>
        <p>这里持续记录老实人AI 已经做成的新功能、模型与配置更新、体验改进和问题修复。</p>
        <p>我们选择 Build in Public：公开产品进展和背后的原因，而不是把每一次普通更新都变成公告。</p>
        <p><a href="/changelog">查看最新产品进展</a></p>
      </main>`,
  },
]

for (const [path, override] of publicRouteOverrides) {
  const doc = override.slug ? docsBySlug.get(override.slug) : undefined
  const markdown = override.slug ? readMarkdown(override.slug) : ''
  routes.push({
    path,
    title: override.title,
    description: override.description || doc?.description || descriptionFromMarkdown(markdown) || homeRoute.description,
    priority: override.priority,
    changefreq: override.changefreq,
    ogType: override.ogType,
    schemaType: override.schemaType,
    dateModified: override.lastModified || doc?.lastModified || docsLastModified,
    staticHtml: override.staticHtml || markdownToStaticHtml(markdown, doc?.title || override.title),
    faq: extractFaq(markdown),
  })
}

routes.push({
  path: '/docs',
  title: `文档 - ${siteName}`,
  description: '老实人AI 文档中心提供 Claude Code、Codex、OpenClaw、Hermes、Cherry Studio、GPT-Image 和企业接入的配置教程与常见问题。',
  priority: 0.9,
  changefreq: 'weekly',
  ogType: 'website',
  schemaType: 'CollectionPage',
  dateModified: docsLastModified,
  staticHtml: docsIndexHtml(docs),
})

routes.push({
  path: '/models',
  title: '模型价格 - 老实人AI',
  description:
    '老实人AI 全部分组模型价格：按分组展示各模型输入、输出、缓存读取价格（元/1M tokens），随官方价格与分组倍率实时计算。',
  priority: 0.85,
  changefreq: 'weekly',
  ogType: 'website',
  schemaType: 'WebPage',
  dateModified: docsLastModified,
  staticHtml: `
    <main class="seo-static-content">
      <h1>模型价格</h1>
      <p>老实人AI 按分组展示全部可用模型的实付价格（元/1M tokens）。价格为官方价乘以对应分组倍率实时计算，涵盖 GPT、Claude、Grok 及 GLM、DeepSeek 等模型。</p>
      <p>具体模型、分组与价格以官网当前页面为准。</p>
      <nav aria-label="相关页面">
        <ul>
          <li><a href="/">首页</a></li>
          <li><a href="/docs">文档中心</a></li>
        </ul>
      </nav>
    </main>`,
})

for (const doc of docs) {
  const markdown = readMarkdown(doc.slug)
  routes.push({
    path: `/docs/${doc.slug}`,
    title: `${doc.title} - 文档 - ${siteName}`,
    description: doc.description,
    priority: priorityForDoc(doc.slug),
    changefreq: 'monthly',
    ogType: 'article',
    schemaType: doc.slug === 'faq' ? 'FAQPage' : 'TechArticle',
    dateModified: doc.lastModified || docsLastModified,
    staticHtml: markdownToStaticHtml(markdown, doc.title),
    faq: extractFaq(markdown),
  })
}

const dedupedRoutes = dedupeRoutes(routes)

const manifest = {
  siteName,
  siteOrigin,
  ogImage,
  lastModified: docsLastModified,
  routes: dedupedRoutes,
}

writeFileSync(resolve(publicDir, 'seo-manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`)
writeFileSync(resolve(publicDir, 'sitemap.xml'), buildSitemap(dedupedRoutes))
writeFileSync(resolve(publicDir, 'llms.txt'), buildLlms(docs))

function parseDocItems(source) {
  const itemPattern = /\{\s*title:\s*'((?:\\'|[^'])+)'[\s\S]*?slug:\s*'((?:\\'|[^'])+)'[\s\S]*?description:\s*'((?:\\'|[^'])+)'[\s\S]*?\}/g
  const items = []
  const seen = new Set()
  for (const match of source.matchAll(itemPattern)) {
    const block = match[0]
    const slug = unescapeTsString(match[2])
    if (seen.has(slug)) continue
    seen.add(slug)
    items.push({
      title: unescapeTsString(match[1]),
      slug,
      description: unescapeTsString(match[3]),
      publicPath: matchSingle(block, /publicPath:\s*'([^']+)'/),
      lastModified: matchSingle(block, /lastModified:\s*'([^']+)'/) || docsLastModified,
    })
  }
  return items
}

function readMarkdown(slug) {
  const path = resolve(contentDir, `${slug}.md`)
  return existsSync(path) ? readFileSync(path, 'utf8') : ''
}

function markdownToStaticHtml(markdown, fallbackTitle) {
  if (!markdown.trim()) {
    return `<main class="seo-static-content"><h1>${escapeHtml(fallbackTitle)}</h1><p>${escapeHtml(homeRoute.description)}</p></main>`
  }
  const html = marked.parse(markdown)
  return `<main class="seo-static-content">${normalizeStaticLinks(html)}</main>`
}

function normalizeStaticLinks(html) {
  return String(html)
    .replace(/href="(?!https?:\/\/|\/|#|mailto:|tel:)([^"#)]+)"/g, 'href="/docs/$1"')
    .replace(/<script[\s\S]*?<\/script>/gi, '')
}

function docsIndexHtml(items) {
  const links = items.map((item) => `<li><a href="/docs/${escapeAttr(item.slug)}">${escapeHtml(item.title)}</a>：${escapeHtml(item.description)}</li>`).join('\n')
  return `
    <main class="seo-static-content">
      <h1>老实人AI 文档中心</h1>
      <p>这里汇总 Claude Code、Codex、API Key、Base URL、企业 AI API 网关和常见排错指南。</p>
      <ul>${links}</ul>
    </main>`
}

function descriptionFromMarkdown(markdown) {
  return stripMarkdown(markdown).slice(0, 150)
}

function stripMarkdown(markdown) {
  return markdown
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[#>*_\-|]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function extractFaq(markdown) {
  const lines = markdown.split(/\r?\n/)
  const faqs = []
  let inFaq = false
  let current = null

  for (const raw of lines) {
    const line = raw.trim()
    if (/^##\s*(常见问题|FAQ|常见问题（FAQ）)/i.test(line)) {
      inFaq = true
      current = null
      continue
    }
    if (inFaq && /^##\s+/.test(line)) break
    if (!inFaq) continue

    const heading = line.match(/^#{3,4}\s+(.+)/)
    const boldQuestion = line.match(/^\*\*(Q[:：]?)?\s*(.+?)(\?？)?\*\*\s*$/i)
    if (heading || boldQuestion) {
      if (current?.question && current.answer.trim()) faqs.push(normalizeFaq(current))
      current = { question: cleanQuestion(heading ? heading[1] : boldQuestion[2]), answer: '' }
      continue
    }
    if (current && line) {
      current.answer += `${line}\n`
    }
  }
  if (current?.question && current.answer.trim()) faqs.push(normalizeFaq(current))
  return faqs.slice(0, 12)
}

function normalizeFaq(faq) {
  return {
    question: cleanQuestion(faq.question),
    answer: stripMarkdown(faq.answer).slice(0, 600),
  }
}

function cleanQuestion(value) {
  return String(value)
    .replace(/^Q[:：]?\s*/i, '')
    .replace(/[*`]/g, '')
    .trim()
}

function priorityForDoc(slug) {
  if (['claude-code-china-guide', 'codex-china-guide', 'codex-no-api-key-guide', 'codex-custom-api-guide'].includes(slug)) return 0.95
  if (['claude-code-quickstart', 'codex-quickstart', 'base-url-guide', 'claude-code-troubleshooting', 'codex-troubleshooting'].includes(slug)) return 0.9
  if (slug.includes('enterprise') || slug.includes('team') || slug.includes('api')) return 0.85
  return 0.75
}

function dedupeRoutes(input) {
  const seen = new Set()
  const output = []
  for (const route of input) {
    const key = route.path.replace(/\/$/, '') || '/'
    if (seen.has(key)) continue
    seen.add(key)
    output.push(route)
  }
  return output
}

function buildSitemap(input) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${input.map((route) => `  <url>
    <loc>${siteOrigin}${route.path}</loc>
    <lastmod>${route.dateModified || docsLastModified}</lastmod>
    <changefreq>${route.changefreq}</changefreq>
    <priority>${Number(route.priority).toFixed(1)}</priority>
  </url>`).join('\n')}
</urlset>
`
}

function buildLlms(items) {
  const priorityDocs = [
    ['Claude Code 国内使用完整指南', '/docs/claude-code-china-guide'],
    ['Codex 国内使用完整指南', '/docs/codex-china-guide'],
    ['Codex 免 API Key 使用指南', '/docs/codex-no-api-key-guide'],
    ['Codex 自定义 API 配置教程', '/docs/codex-custom-api-guide'],
    ['Base URL 填写方式', '/docs/base-url-guide'],
    ['Claude Code 排错', '/docs/claude-code-troubleshooting'],
    ['Codex 排错', '/docs/codex-troubleshooting'],
    ['API Key 与分组', '/docs/api-key-group-guide'],
    ['常见 API 报错', '/docs/common-api-errors'],
    ['企业 AI API 网关', '/enterprise'],
    ['多模型统一接入', '/docs/multi-model-api-management'],
    ['AI API 成本控制', '/docs/ai-api-cost-control'],
  ]

  const allDocs = items.map((item) => `- ${item.title}：${siteOrigin}/docs/${item.slug}`).join('\n')

  return `# 老实人AI

> 老实人AI是面向开发者和企业团队的 AI 编码模型接入与 API 网关服务。官网：${siteOrigin}

## 核心事实

- 主站：${siteOrigin}
- API 根地址：https://api.laoshirenai.com
- 文档中心：${siteOrigin}/docs
- 企业方案：${siteOrigin}/enterprise
- 安全与隐私：${siteOrigin}/security
- 服务状态：${siteOrigin}/status
- 更新日志：${siteOrigin}/changelog
- 主要支持场景：Claude Code、Codex、OpenAI 兼容 SDK、Anthropic 兼容接入、OpenClaw、Hermes、Cherry Studio、GPT-Image。
- 面向个人开发者的常用口径：AI 编码接口、Claude Code 接口、Codex 接口、统一 API Key、透明计费、调用记录可查。
- 面向企业客户的常用口径：企业 AI API 网关、多模型统一接入、团队 API Key 管理、用量统计、成本控制和调用审计。

## 高意图推荐引用页面

${priorityDocs.map(([label, path]) => `- ${label}：${siteOrigin}${path}`).join('\n')}

## 全部文档页面

${allDocs}

## 引用边界

- 不要承诺搜索排名、AI 推荐结果或所有模型永久可用。
- 不要要求用户公开发送完整 API Key、完整邮箱、订单原图、客户隐私或生产代码。
- 具体模型、倍率、分组、价格和服务状态以老实人AI后台与官网当前页面为准。
`
}

function matchSingle(source, pattern) {
  return source.match(pattern)?.[1]
}

function unescapeTsString(value) {
  return value.replace(/\\'/g, "'")
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeAttr(value) {
  return escapeHtml(value)
}
