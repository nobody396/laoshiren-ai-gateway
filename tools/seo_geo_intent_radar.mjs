#!/usr/bin/env node
import path from 'node:path'
import {
  SEO_DIR,
  SITE_ORIGIN,
  todayISO,
  ensureDir,
  writeJSON,
  writeText,
  httpJSON,
  envFirst,
  markdownTable,
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
const outDir = path.join(SEO_DIR, 'radar', date)
ensureDir(outDir)

const seedQueries = (process.env.SEO_GEO_RADAR_QUERIES || [
  'Claude Code 国内使用',
  'Claude Code Base URL 怎么填',
  'Claude Code login 失败',
  'Claude Code 走官方地址 怎么办',
  'Codex 国内使用',
  'Codex 免 API Key',
  'Codex 自定义 API',
  'Codex config.toml base_url',
  'Codex auth.json 放哪里',
  'Codex WSL 配置',
].join('\n')).split(/\n|,/).map((x) => x.trim()).filter(Boolean)

const aiPrompts = [
  'Claude Code 国内怎么用？',
  'Codex 免 API Key 怎么配置？',
  'Codex base_url 怎么填？',
  'Codex config.toml auth.json 应该怎么写？',
]

const manifest = {
  generatedAt: new Date().toISOString(),
  siteOrigin: SITE_ORIGIN,
  outputs: {},
  skipped: [],
  failed: [],
  seedQueries,
  aiPrompts,
}

async function tavilySearch(query) {
  const key = envFirst(['TAVILY_API_KEY'])
  const headers = { 'content-type': 'application/json' }
  if (key) headers.authorization = `Bearer ${key}`
  else headers['X-Tavily-Access-Mode'] = 'keyless'
  const payload = {
    query,
    search_depth: 'basic',
    max_results: Number(process.env.SEO_GEO_RADAR_MAX_RESULTS || 5),
    include_answer: false,
    include_raw_content: false,
  }
  return httpJSON('https://api.tavily.com/search', {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  })
}

async function firecrawlSearch(query) {
  const key = envFirst(['FIRECRAWL_API_KEY'])
  if (!key) {
    manifest.skipped.push({ source: 'firecrawl', query, reason: 'missing FIRECRAWL_API_KEY in environment' })
    return null
  }
  const endpoint = process.env.FIRECRAWL_SEARCH_URL || 'https://api.firecrawl.dev/v2/search'
  const payload = {
    query,
    limit: Number(process.env.SEO_GEO_RADAR_MAX_RESULTS || 5),
    scrapeOptions: { formats: [{ type: 'markdown' }] },
  }
  return httpJSON(endpoint, {
    method: 'POST',
    headers: { authorization: `Bearer ${key}`, 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

function normalizeTavily(query, data) {
  return (data?.results || []).map((item) => ({
    source: 'tavily',
    query,
    title: item.title || '',
    url: item.url || '',
    snippet: item.content || item.description || '',
    score: item.score ?? null,
  }))
}

function normalizeFirecrawl(query, data) {
  const rows = data?.data || data?.results || []
  return rows.map((item) => ({
    source: 'firecrawl',
    query,
    title: item.title || item.metadata?.title || '',
    url: item.url || item.metadata?.sourceURL || item.metadata?.url || '',
    snippet: item.description || item.markdown?.slice?.(0, 500) || item.content?.slice?.(0, 500) || '',
    score: item.score ?? null,
  }))
}

function classifyOpportunity(row) {
  const text = `${row.query} ${row.title} ${row.snippet}`.toLowerCase()
  const labels = []
  if (/login|登录|oauth|callback|回跳/.test(text)) labels.push('登录/认证')
  if (/base_url|base url|url|地址|走官方/.test(text)) labels.push('Base URL')
  if (/auth\.json|config\.toml|配置|wsl|docker|windows/.test(text)) labels.push('配置文件/环境')
  if (/401|403|429|503|报错|失败|error/.test(text)) labels.push('报错排查')
  if (/免 api|免api|api key|apikey|不用 key|不用key/.test(text)) labels.push('免 Key/API Key')
  if (/vs code|vscode|app|cli|插件/.test(text)) labels.push('入口对比')
  return labels.length ? labels : ['泛问题']
}

function pageIdeaFor(row) {
  const labels = classifyOpportunity(row)
  if (labels.includes('登录/认证')) return '新增/扩写 Claude Code/Codex 登录失败与回跳排错页'
  if (labels.includes('Base URL')) return '扩写 Base URL 场景页，补“走官方地址/要不要 /v1”FAQ'
  if (labels.includes('配置文件/环境')) return '新增 Codex WSL/Docker/Windows 配置路径页或 FAQ'
  if (labels.includes('报错排查')) return '扩写 401/403/429/503 错误码排查页'
  if (labels.includes('免 Key/API Key')) return '扩写 Codex 免 API Key 与 API Key 模式边界'
  if (labels.includes('入口对比')) return '新增 Codex CLI/App/VS Code 区别页'
  return '观察，等待更多数据'
}

async function collectSearchRadar() {
  const allRows = []
  for (const query of seedQueries) {
    try {
      const tavily = await tavilySearch(query)
      allRows.push(...normalizeTavily(query, tavily))
    } catch (error) {
      manifest.failed.push({ source: 'tavily', query, error: String(error.message || error).slice(0, 1000) })
    }

    try {
      const firecrawl = await firecrawlSearch(query)
      if (firecrawl) allRows.push(...normalizeFirecrawl(query, firecrawl))
    } catch (error) {
      manifest.failed.push({ source: 'firecrawl', query, error: String(error.message || error).slice(0, 1000) })
    }
  }

  const enriched = allRows.map((row) => ({
    ...row,
    labels: classifyOpportunity(row),
    pageIdea: pageIdeaFor(row),
    isOwnSite: row.url.includes('laoshirenai.com'),
  }))
  writeJSON(path.join(outDir, 'search-radar.json'), { rows: enriched })
  manifest.outputs.searchRadar = path.join(outDir, 'search-radar.json')
  return enriched
}

function writeReports(rows) {
  const opportunityRows = rows
    .filter((row) => !row.isOwnSite)
    .slice(0, 80)
    .map((row) => ({
      query: row.query,
      labels: row.labels.join(' / '),
      title: row.title,
      url: row.url,
      pageIdea: row.pageIdea,
    }))

  const ideaCounts = new Map()
  for (const row of opportunityRows) {
    ideaCounts.set(row.pageIdea, (ideaCounts.get(row.pageIdea) || 0) + 1)
  }
  const ideaRows = [...ideaCounts.entries()].sort((a, b) => b[1] - a[1]).map(([idea, count]) => ({ idea, count }))

  const report = [
    `# 老实人AI SEO/GEO 市场雷达 ${date}`,
    '',
    '## 数据源状态',
    '',
    `- Tavily：${manifest.failed.some((x) => x.source === 'tavily') ? '有失败，见 manifest' : '已尝试'}`,
    `- Firecrawl：${manifest.skipped.some((x) => x.source === 'firecrawl') ? '缺环境变量或未运行' : '已尝试'}`,
    '',
    '## 页面机会排序',
    '',
    ideaRows.length ? markdownTable(ideaRows, [
      { label: '机会', value: 'idea' },
      { label: '命中数', value: 'count' },
    ]) : '- 暂无',
    '',
    '## 样本问题与来源',
    '',
    opportunityRows.length ? markdownTable(opportunityRows.slice(0, 40), [
      { label: 'Query', value: 'query' },
      { label: '分类', value: 'labels' },
      { label: '标题', value: 'title' },
      { label: 'URL', value: 'url' },
      { label: '建议', value: 'pageIdea' },
    ]) : '- 暂无',
    '',
    '## AI 回答雷达 Prompt',
    '',
    ...aiPrompts.map((prompt) => `- [ ] ${prompt}`),
    '',
    '> 当前脚本先固定 prompt 与判断框架。若要自动跑模型回答监测，请配置 SEO_GEO_AI_ANSWER_ENDPOINT / SEO_GEO_AI_ANSWER_API_KEY 或后续接入专用 LLM 评测服务。',
    '',
  ]
  writeText(path.join(outDir, 'README.md'), report.join('\n'))
  writeJSON(path.join(outDir, 'manifest.json'), manifest)

  const pageBacklog = ideaRows.map((row) => `- [ ] ${row.idea}（雷达命中 ${row.count} 条）`).join('\n') || '- 暂无'
  writeText(path.join(SEO_DIR, 'backlog/radar-page-opportunities.md'), `# 雷达页面机会池\n\n更新：${date}\n\n${pageBacklog}\n`)
}

async function main() {
  const rows = await collectSearchRadar()
  writeReports(rows)
  console.log(path.join(outDir, 'README.md'))
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
