#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import {
  SEO_DIR,
  SITE_ORIGIN,
  P0_PATHS,
  todayISO,
  latestSubdir,
  readJSON,
  writeJSON,
  writeText,
  markdownTable,
  safeNumber,
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
function defaultDataDirFor(runDate) {
  const dateDir = path.join(SEO_DIR, 'data', runDate)
  if (fs.existsSync(path.join(dateDir, 'manifest.json'))) return dateDir
  return path.join(SEO_DIR, 'data', latestSubdir(path.join(SEO_DIR, 'data')) || runDate)
}

const dataDir = args.get('data-dir') || defaultDataDirFor(date)
const outFile = args.get('out') || path.join(SEO_DIR, 'weekly', `${date}-decision.md`)

function urlPath(url) {
  try { return new URL(url, SITE_ORIGIN).pathname } catch { return url || '' }
}

function aggregateGSC(rows) {
  const byPage = new Map()
  for (const row of rows || []) {
    const pathKey = urlPath(row.page)
    if (!byPage.has(pathKey)) byPage.set(pathKey, { path: pathKey, clicks: 0, impressions: 0, weightedPosition: 0, queries: [] })
    const page = byPage.get(pathKey)
    const impressions = safeNumber(row.impressions)
    const clicks = safeNumber(row.clicks)
    page.clicks += clicks
    page.impressions += impressions
    page.weightedPosition += safeNumber(row.position) * Math.max(impressions, 1)
    page.queries.push({ query: row.query, clicks, impressions, ctr: safeNumber(row.ctr), position: safeNumber(row.position) })
  }
  for (const page of byPage.values()) {
    page.ctr = page.impressions ? page.clicks / page.impressions : 0
    page.position = page.impressions ? page.weightedPosition / page.impressions : 0
    page.queries = page.queries.sort((a, b) => b.impressions - a.impressions).slice(0, 10)
  }
  return byPage
}

function aggregateGA4(pageRows, eventRows) {
  const byPage = new Map()
  for (const row of pageRows || []) {
    const p = row.pagePath || ''
    if (!byPage.has(p)) byPage.set(p, { path: p, sessions: 0, users: 0, views: 0, engagedSessions: 0, events: 0, keyEvents: {} })
    const item = byPage.get(p)
    item.sessions += safeNumber(row.sessions)
    item.users += safeNumber(row.totalUsers)
    item.views += safeNumber(row.screenPageViews)
    item.engagedSessions += safeNumber(row.engagedSessions)
    item.events += safeNumber(row.eventCount)
  }
  const conversionNames = (process.env.SEO_GEO_CONVERSION_EVENTS || 'sign_up,login,generate_lead,purchase,create_api_key,api_key_created,first_api_call,redeem_success,topup_success').split(',').map((x) => x.trim()).filter(Boolean)
  for (const row of eventRows || []) {
    const p = row.pagePath || ''
    if (!byPage.has(p)) byPage.set(p, { path: p, sessions: 0, users: 0, views: 0, engagedSessions: 0, events: 0, keyEvents: {} })
    if (conversionNames.includes(row.eventName)) {
      byPage.get(p).keyEvents[row.eventName] = (byPage.get(p).keyEvents[row.eventName] || 0) + safeNumber(row.eventCount)
    }
  }
  return byPage
}

function loadTechnicalSummary(file) {
  if (!fs.existsSync(file)) return null
  const text = fs.readFileSync(file, 'utf8')
  const json = text.match(/```json\n([\s\S]*?)\n```/)
  if (!json) return null
  try { return JSON.parse(json[1]) } catch { return null }
}

function decide(page, dataAvailability) {
  const actions = []
  const isP0 = P0_PATHS.includes(page.path)
  const imp = page.impressions || 0
  const clicks = page.clicks || 0
  const ctr = page.ctr || 0
  const position = page.position || 0
  const sessions = page.sessions || 0
  const conversions = Object.values(page.keyEvents || {}).reduce((a, b) => a + b, 0)

  if (page.techStatus && page.techStatus !== 200) actions.push({ priority: 'P0', type: 'technical', action: '修 HTTP 状态', reason: `status=${page.techStatus}` })
  if (page.visibleChars != null && page.visibleChars < 100) actions.push({ priority: 'P0', type: 'technical', action: '补原始 HTML 正文', reason: `visible_chars=${page.visibleChars}` })
  if (!isP0 && !imp && !clicks && !sessions && !conversions && !actions.length) return actions
  if (imp >= 100 && ctr < 0.02) actions.push({ priority: 'P1', type: 'copy', action: '重写 title/description/首段结论', reason: `曝光 ${imp} 但 CTR ${(ctr * 100).toFixed(1)}%` })
  if (imp >= 50 && position > 8 && position <= 20) actions.push({ priority: 'P1', type: 'content', action: '扩充 FAQ、排错步骤、内链，提高相关性', reason: `平均排名 ${position.toFixed(1)}` })
  if (dataAvailability.gsc && imp < 20 && isP0) actions.push({ priority: 'P1', type: 'distribution', action: '增加站内入口和外部引用，检查 sitemap lastmod', reason: 'P0 页面曝光不足' })
  if (dataAvailability.ga4 && clicks >= 10 && sessions >= 10 && conversions === 0) actions.push({ priority: 'P1', type: 'conversion', action: '强化 CTA、注册/创建 Key 下一步、配置成功路径', reason: `点击/会话有量但关键事件为 0` })
  if (dataAvailability.gsc && dataAvailability.ga4 && clicks >= 1 && sessions === 0) actions.push({ priority: 'P2', type: 'measurement', action: '检查 GA4 页面路径/跨域/事件埋点', reason: `GSC 有点击 ${clicks} 但 GA4 会话为 0` })
  if (!actions.length && (imp || sessions)) actions.push({ priority: 'P3', type: 'observe', action: '继续观察，不做大改', reason: '当前无明显异常' })
  if (!actions.length && isP0) actions.push({ priority: 'P2', type: 'data', action: '等待真实数据或补充内链', reason: '暂无 GSC/GA4 数据' })
  return actions
}

const manifest = readJSON(path.join(dataDir, 'manifest.json'), {})
const gsc = readJSON(path.join(dataDir, 'gsc-query-page.json'), { rows: [] })
const ga4Pages = readJSON(path.join(dataDir, 'ga4-pages.json'), { rows: [] })
const ga4Events = readJSON(path.join(dataDir, 'ga4-events.json'), { rows: [] })
const tech = loadTechnicalSummary(path.join(dataDir, 'technical-audit.md'))

const gscByPage = aggregateGSC(gsc.rows)
const ga4ByPage = aggregateGA4(ga4Pages.rows, ga4Events.rows)
const techByPath = new Map((tech?.pages || []).map((p) => [urlPath(p.url), p]))
const allPaths = new Set([...P0_PATHS, ...gscByPage.keys(), ...ga4ByPage.keys(), ...techByPath.keys()].filter(Boolean))
const dataAvailability = { gsc: Boolean(manifest.outputs?.gsc), ga4: Boolean(manifest.outputs?.ga4Pages) }
const pages = [...allPaths].sort().map((p) => {
  const g = gscByPage.get(p) || {}
  const a = ga4ByPage.get(p) || {}
  const t = techByPath.get(p) || {}
  const page = {
    path: p,
    impressions: safeNumber(g.impressions),
    clicks: safeNumber(g.clicks),
    ctr: safeNumber(g.ctr),
    position: safeNumber(g.position),
    sessions: safeNumber(a.sessions),
    users: safeNumber(a.users),
    views: safeNumber(a.views),
    keyEvents: a.keyEvents || {},
    topQueries: g.queries || [],
    techStatus: t.status,
    visibleChars: t.visible_chars,
    h1Count: t.h1_count,
    ldJsonCount: t.ld_json_count,
  }
  page.actions = decide(page, dataAvailability)
  page.topPriority = page.actions[0]?.priority || 'P3'
  return page
})

const actionRows = pages.flatMap((page) => page.actions.map((action) => ({ path: page.path, ...action })))
const p0Rows = pages.filter((p) => P0_PATHS.includes(p.path))
const report = [
  `# 老实人AI SEO/GEO 效果决策 ${date}`,
  '',
  `数据目录：\`${dataDir}\``,
  '',
  '## 数据可用性',
  '',
  `- 技术体检：${manifest.outputs?.technicalAudit ? '有' : '缺失'}`,
  `- GSC：${manifest.outputs?.gsc ? '有' : '缺失'}`,
  `- GA4 页面：${manifest.outputs?.ga4Pages ? '有' : '缺失'}`,
  `- GA4 事件：${manifest.outputs?.ga4Events ? '有' : '缺失'}`,
  '',
  '## P0 页面仪表盘',
  '',
  markdownTable(p0Rows, [
    { label: '页面', value: 'path' },
    { label: '曝光', value: (r) => r.impressions },
    { label: '点击', value: (r) => r.clicks },
    { label: 'CTR', value: (r) => `${(r.ctr * 100).toFixed(1)}%` },
    { label: '排名', value: (r) => r.position ? r.position.toFixed(1) : '-' },
    { label: '会话', value: (r) => r.sessions },
    { label: '正文', value: (r) => r.visibleChars ?? '-' },
    { label: '首要动作', value: (r) => r.actions[0]?.action || '-' },
  ]),
  '',
  '## 动作队列',
  '',
  markdownTable(actionRows.sort((a, b) => a.priority.localeCompare(b.priority)), [
    { label: '优先级', value: 'priority' },
    { label: '类型', value: 'type' },
    { label: '页面', value: 'path' },
    { label: '动作', value: 'action' },
    { label: '原因', value: 'reason' },
  ]),
  '',
  '## P0 页面 Top Queries',
  '',
  ...p0Rows.flatMap((page) => [
    `### ${page.path}`,
    page.topQueries.length ? markdownTable(page.topQueries, [
      { label: 'Query', value: 'query' },
      { label: '曝光', value: 'impressions' },
      { label: '点击', value: 'clicks' },
      { label: 'CTR', value: (r) => `${(r.ctr * 100).toFixed(1)}%` },
      { label: '排名', value: (r) => safeNumber(r.position).toFixed(1) },
    ]) : '- 暂无 GSC query 数据',
    '',
  ]),
]

writeText(outFile, report.join('\n'))
writeJSON(path.join(SEO_DIR, 'weekly', `${date}-decision.json`), { dataDir, pages })

const pageBacklog = actionRows
  .filter((row) => ['content', 'distribution', 'copy', 'conversion'].includes(row.type))
  .map((row) => `- [ ] ${row.priority} ${row.path}：${row.action}（${row.reason}）`)
writeText(path.join(SEO_DIR, 'backlog/page-opportunities.md'), `# 页面机会池\n\n更新：${date}\n\n${pageBacklog.length ? pageBacklog.join('\n') : '- 暂无'}\n`)

const faqBacklog = pages.flatMap((page) => page.topQueries || [])
  .filter((q) => /怎么|如何|为什么|失败|报错|配置|登录|api|key|base/i.test(q.query || ''))
  .slice(0, 100)
  .map((q) => `- [ ] ${q.query}（曝光 ${q.impressions}，点击 ${q.clicks}，排名 ${safeNumber(q.position).toFixed(1)}）`)
writeText(path.join(SEO_DIR, 'backlog/faq-opportunities.md'), `# FAQ 机会池\n\n更新：${date}\n\n${faqBacklog.length ? faqBacklog.join('\n') : '- 暂无 GSC query 数据'}\n`)

console.log(outFile)
