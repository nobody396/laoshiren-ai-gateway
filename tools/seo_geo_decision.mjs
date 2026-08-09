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

function aggregateGSCQueries(rows) {
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

function indexGSCPages(rows) {
  const byPage = new Map()
  for (const row of rows || []) {
    const pathKey = urlPath(row.page)
    if (!byPage.has(pathKey)) byPage.set(pathKey, { path: pathKey, clicks: 0, impressions: 0, weightedPosition: 0 })
    const page = byPage.get(pathKey)
    const impressions = safeNumber(row.impressions)
    page.clicks += safeNumber(row.clicks)
    page.impressions += impressions
    page.weightedPosition += safeNumber(row.position) * impressions
  }
  for (const page of byPage.values()) {
    page.ctr = page.impressions ? page.clicks / page.impressions : 0
    page.position = page.impressions ? page.weightedPosition / page.impressions : 0
    delete page.weightedPosition
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

function finding(priority, type, action, reason, options = {}) {
  return {
    priority,
    type,
    action,
    reason,
    evidenceLevel: options.evidenceLevel || 'inferred',
    impact: options.impact || 'medium',
    confidence: options.confidence || 'medium',
    effort: options.effort || 'medium',
    dependencies: options.dependencies || '无',
    verification: options.verification || '复跑采集并核对对应指标',
    outcomeStages: {
      implemented: false,
      deployedObservable: false,
      searchPlatformProcessed: false,
      outcomeObserved: false,
    },
  }
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

  if (page.techStatus && page.techStatus !== 200) actions.push(finding('P0', 'technical', '修 HTTP 状态', `status=${page.techStatus}`, { evidenceLevel: 'observed', impact: 'high', confidence: 'high', verification: '线上 curl 与技术体检均返回 200' }))
  if (page.visibleChars != null && page.visibleChars < 100) actions.push(finding('P0', 'technical', '补原始 HTML 正文', `visible_chars=${page.visibleChars}`, { evidenceLevel: 'observed', impact: 'high', confidence: 'high', verification: '禁用 JavaScript 抓取仍可看到正文' }))
  if (!isP0 && !imp && !clicks && !sessions && !conversions && !actions.length) return actions
  if (imp >= 20 && clicks === 0) actions.push(finding('P1', 'copy', '建立单变量 title 实验', `曝光 ${imp}，点击 0，平均排名 ${position.toFixed(1)}`, { evidenceLevel: 'observed', impact: 'medium', confidence: imp >= 100 ? 'medium' : 'low', effort: 'low', verification: '等待搜索平台重新处理后，按同查询簇比较 28 天 CTR' }))
  if (imp >= 50 && position > 8 && position <= 20) actions.push(finding('P1', 'content', '按已出现查询扩充 FAQ、排错步骤和上下文内链', `平均排名 ${position.toFixed(1)}`, { evidenceLevel: 'observed', effort: 'medium', verification: '比较相同查询簇的曝光、排名和点击' }))
  if (dataAvailability.gsc && imp < 20 && isP0) actions.push(finding('P2', 'experiment', '只登记小样本实验；先加强相关站内入口并等待更多 finalized 数据', `P0 页面仅 ${imp} 次曝光，不足以可靠判断 CTR`, { evidenceLevel: 'observed', impact: 'medium', confidence: 'low', effort: 'low', verification: 'Google 重新抓取后观察至少 28 天 finalized 数据' }))
  if (dataAvailability.ga4 && clicks >= 10 && sessions >= 10 && conversions === 0) actions.push(finding('P1', 'conversion', '强化 CTA、注册/创建 Key 下一步和配置成功路径', `点击/会话有量但关键事件为 0`, { evidenceLevel: 'observed', impact: 'high', verification: '关键事件和漏斗步骤开始稳定入数' }))
  if (dataAvailability.ga4Rows && clicks >= 1 && sessions === 0) actions.push(finding('P1', 'measurement', '核对 GA4 页面路径归一化', `GA4 已有其他页面数据，但本页 GSC 有点击 ${clicks}、GA4 会话为 0`, { evidenceLevel: 'observed', impact: 'high', confidence: 'medium', effort: 'low', dependencies: 'GA4 Realtime/DebugView 访问', verification: '受控访问后 Realtime 与次日 finalized 报告均出现本页 page_view' }))
  if (!actions.length && (imp || sessions)) actions.push(finding('P3', 'observe', '继续观察，不做大改', '当前没有达到动作阈值', { evidenceLevel: 'observed', impact: 'low', confidence: 'high', effort: 'low' }))
  if (!actions.length && isP0) actions.push(finding('P2', 'data', '补足可观测性并等待真实数据', '暂无 GSC/GA4 数据', { evidenceLevel: 'missing_evidence', impact: 'medium', confidence: 'low', effort: 'low', verification: '对应数据源返回 finalized 行' }))
  return actions
}

const manifest = readJSON(path.join(dataDir, 'manifest.json'), {})
const gscSummary = readJSON(path.join(dataDir, 'gsc-summary.json'), { totals: {} })
const gscPages = readJSON(path.join(dataDir, 'gsc-pages.json'), { rows: [] })
const gscQueries = readJSON(path.join(dataDir, 'gsc-query-page.json'), { rows: [] })
const ga4Pages = readJSON(path.join(dataDir, 'ga4-pages.json'), { rows: [] })
const ga4Events = readJSON(path.join(dataDir, 'ga4-events.json'), { rows: [] })
const tech = loadTechnicalSummary(path.join(dataDir, 'technical-audit.md'))

const gscByPage = gscPages.rows?.length ? indexGSCPages(gscPages.rows) : aggregateGSCQueries(gscQueries.rows)
const gscQueriesByPage = aggregateGSCQueries(gscQueries.rows)
const ga4ByPage = aggregateGA4(ga4Pages.rows, ga4Events.rows)
const techByPath = new Map((tech?.pages || []).map((p) => [urlPath(p.url), p]))
const allPaths = new Set([...P0_PATHS, ...gscByPage.keys(), ...ga4ByPage.keys(), ...techByPath.keys()].filter(Boolean))
const dataAvailability = {
  gsc: Boolean(manifest.outputs?.gscPages || manifest.outputs?.gsc),
  ga4: Boolean(manifest.outputs?.ga4Pages),
  ga4Rows: Boolean((ga4Pages.rows || []).length || (ga4Events.rows || []).length),
}
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
    topQueries: gscQueriesByPage.get(p)?.queries || [],
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
const propertyTotals = {
  clicks: safeNumber(gscSummary.totals?.clicks),
  impressions: safeNumber(gscSummary.totals?.impressions),
  ctr: safeNumber(gscSummary.totals?.ctr),
  position: safeNumber(gscSummary.totals?.position),
}
const pageTotals = [...gscByPage.values()].reduce((sum, row) => ({ clicks: sum.clicks + safeNumber(row.clicks), impressions: sum.impressions + safeNumber(row.impressions) }), { clicks: 0, impressions: 0 })
const queryTotals = (gscQueries.rows || []).reduce((sum, row) => ({ clicks: sum.clicks + safeNumber(row.clicks), impressions: sum.impressions + safeNumber(row.impressions) }), { clicks: 0, impressions: 0 })
if (dataAvailability.ga4 && !dataAvailability.ga4Rows && propertyTotals.clicks > 0) {
  actionRows.push({
    path: '[GA4 property]',
    ...finding('P0', 'measurement', '修复 GA4 入数链路后再判断内容转化', `GSC Property 有 ${propertyTotals.clicks} 次点击，但 GA4 pages/events/realtime 均返回 0 行`, {
      evidenceLevel: 'observed',
      impact: 'high',
      confidence: 'high',
      effort: 'low',
      dependencies: 'GA4 Realtime/DebugView 与 Data Stream 查看权限',
      verification: '受控 page_view 在 Realtime 出现，次日 finalized 报告出现页面与事件',
    }),
  })
}
const coverage = {
  discovered: tech?.pages?.length || 0,
  selected: P0_PATHS.length,
  fetched: tech?.pages?.length || 0,
  rendered: 0,
  dataBacked: [...gscByPage.values()].filter((row) => row.impressions || row.clicks).length,
  failed: manifest.failed?.length || 0,
}
const report = [
  `# 老实人AI SEO/GEO 效果决策 ${date}`,
  '',
  `数据目录：\`${dataDir}\``,
  '',
  '## 数据可用性',
  '',
  `- 技术体检：${manifest.outputs?.technicalAudit ? '有' : '缺失'}`,
  `- GSC：${manifest.outputs?.gsc ? '有' : '缺失'}`,
  `- GA4 页面：${manifest.outputs?.ga4Pages ? `API 可访问，${ga4Pages.rows?.length || 0} 行` : '缺失'}`,
  `- GA4 事件：${manifest.outputs?.ga4Events ? `API 可访问，${ga4Events.rows?.length || 0} 行` : '缺失'}`,
  `- GA4 Realtime：${manifest.outputs?.ga4Realtime ? 'API 可访问（本次 0 行）' : '缺失'}`,
  '',
  '## 数据口径与覆盖',
  '',
  `- GSC Property：${manifest.sourceMetadata?.gsc?.siteUrl || gscSummary.siteUrl || '未记录'}`,
  `- 周期：${manifest.startDate || gscSummary.startDate || '-'} 至 ${manifest.endDate || gscSummary.endDate || '-'}；Search type=web；dataState=final；日期时区=America/Los_Angeles；过滤器=无`,
  `- Property 聚合：曝光 ${propertyTotals.impressions}，点击 ${propertyTotals.clicks}，CTR ${(propertyTotals.ctr * 100).toFixed(1)}%，平均排名 ${propertyTotals.position ? propertyTotals.position.toFixed(1) : '-'}`,
  `- Page 维度合计：曝光 ${pageTotals.impressions}，点击 ${pageTotals.clicks}；Query+Page 可见行合计：曝光 ${queryTotals.impressions}，点击 ${queryTotals.clicks}`,
  '- Property、Page、Query+Page 的聚合语义不同，维度合计不要求与 Property 总量对平，也不能互相替代。',
  '- Query+Page 数据只代表可见查询下界：匿名查询可能被省略，Search Analytics 也不保证返回全部明细行。',
  `- Coverage ledger：discovered=${coverage.discovered}，selected=${coverage.selected}，fetched=${coverage.fetched}，rendered=${coverage.rendered}，data_backed=${coverage.dataBacked}，failed=${coverage.failed}`,
  '- 静态 HTML 已抓取；本轮未执行渲染后 DOM 抓取，任何渲染结论均属缺失证据。',
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
    { label: '证据', value: 'evidenceLevel' },
    { label: '影响', value: 'impact' },
    { label: '置信度', value: 'confidence' },
    { label: '工作量', value: 'effort' },
    { label: '验收', value: 'verification' },
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
writeJSON(path.join(SEO_DIR, 'weekly', `${date}-decision.json`), {
  dataDir,
  scope: manifest.sourceMetadata || {},
  propertyTotals,
  pageTotals,
  queryVisibleLowerBound: queryTotals,
  coverage,
  limitations: [
    'Query dimensions may omit anonymized queries.',
    'Search Analytics detail rows are not guaranteed exhaustive.',
    'Rendered DOM was not collected in this run.',
  ],
  pages,
})

const pageBacklog = actionRows
  .filter((row) => ['content', 'experiment', 'copy', 'conversion'].includes(row.type))
  .map((row) => `- [ ] ${row.priority} ${row.path}：${row.action}（${row.reason}）`)
writeText(path.join(SEO_DIR, 'backlog/page-opportunities.md'), `# 页面机会池\n\n更新：${date}\n\n${pageBacklog.length ? pageBacklog.join('\n') : '- 暂无'}\n`)

const faqBacklog = pages.flatMap((page) => page.topQueries || [])
  .filter((q) => /怎么|如何|为什么|失败|报错|配置|登录|api|key|base/i.test(q.query || ''))
  .slice(0, 100)
  .map((q) => `- [ ] ${q.query}（曝光 ${q.impressions}，点击 ${q.clicks}，排名 ${safeNumber(q.position).toFixed(1)}）`)
writeText(path.join(SEO_DIR, 'backlog/faq-opportunities.md'), `# FAQ 机会池\n\n更新：${date}\n\n${faqBacklog.length ? faqBacklog.join('\n') : '- 暂无 GSC query 数据'}\n`)

console.log(outFile)
