#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import {
  SEO_DIR,
  SITE_ORIGIN,
  todayISO,
  ymdDaysAgo,
  ensureDir,
  writeJSON,
  writeText,
  loadServiceAccount,
  googleAccessToken,
  httpJSON,
  envFirst,
} from './seo_geo_lib.mjs'

const args = new Map()
for (let i = 2; i < process.argv.length; i += 1) {
  const item = process.argv[i]
  if (item.startsWith('--')) {
    const key = item.slice(2)
    const next = process.argv[i + 1]
    if (next && !next.startsWith('--')) {
      args.set(key, next)
      i += 1
    } else {
      args.set(key, '1')
    }
  }
}

const runDate = args.get('date') || todayISO()
const days = Number(args.get('days') || process.env.SEO_GEO_DAYS || 28)
const endDate = args.get('end-date') || ymdDaysAgo(1)
const startDate = args.get('start-date') || ymdDaysAgo(days)
const outDir = path.join(SEO_DIR, 'data', runDate)
ensureDir(outDir)

const manifest = {
  generatedAt: new Date().toISOString(),
  siteOrigin: SITE_ORIGIN,
  startDate,
  endDate,
  outputs: {},
  skipped: [],
  failed: [],
  sourceMetadata: {},
}

async function collectTechnicalAudit() {
  const out = path.join(outDir, 'technical-audit.md')
  const result = spawnSync('python3', ['tools/seo_geo_audit.py', '--base-url', SITE_ORIGIN, '--timeout', '8', '--workers', '8', '--out', out], {
    cwd: process.cwd(),
    encoding: 'utf8',
  })
  manifest.outputs.technicalAudit = out
  if (result.status !== 0) {
    manifest.failed.push({ source: 'technical-audit', status: result.status, stderr: result.stderr.slice(0, 2000) })
  }
}

async function collectGSC() {
  const siteUrl = envFirst(['GSC_SITE_URL', 'SEARCH_CONSOLE_SITE_URL']) || `${SITE_ORIGIN}/`
  const serviceAccount = loadServiceAccount('GSC') || loadServiceAccount('GOOGLE')
  if (!serviceAccount) {
    manifest.skipped.push({ source: 'gsc', reason: 'missing GSC_SERVICE_ACCOUNT_JSON(_B64/_PATH) or GOOGLE_SERVICE_ACCOUNT_JSON(_B64/_PATH)' })
    return
  }
  const token = await googleAccessToken(serviceAccount, 'https://www.googleapis.com/auth/webmasters.readonly')
  const endpoint = `https://searchconsole.googleapis.com/webmasters/v3/sites/${encodeURIComponent(siteUrl)}/searchAnalytics/query`
  const rowLimit = Math.min(Number(process.env.GSC_ROW_LIMIT || 25000), 25000)
  const request = async ({ dimensions = [], aggregationType = 'auto' }) => {
    const payload = {
      startDate,
      endDate,
      type: 'web',
      dataState: 'final',
      aggregationType,
      rowLimit,
      startRow: 0,
      ...(dimensions.length ? { dimensions } : {}),
    }
    const data = await httpJSON(endpoint, {
      method: 'POST',
      headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
      body: JSON.stringify(payload),
    })
    return { payload, data }
  }

  const summaryResult = await request({ aggregationType: 'byProperty' })
  const summaryRow = summaryResult.data.rows?.[0] || {}
  const summaryFile = path.join(outDir, 'gsc-summary.json')
  writeJSON(summaryFile, {
    siteUrl,
    startDate,
    endDate,
    request: summaryResult.payload,
    responseAggregationType: summaryResult.data.responseAggregationType || '',
    totals: {
      clicks: summaryRow.clicks || 0,
      impressions: summaryRow.impressions || 0,
      ctr: summaryRow.ctr || 0,
      position: summaryRow.position || 0,
    },
  })

  const pagesResult = await request({ dimensions: ['page'], aggregationType: 'byPage' })
  const pageRows = (pagesResult.data.rows || []).map((row) => ({
    page: row.keys?.[0] || '',
    clicks: row.clicks || 0,
    impressions: row.impressions || 0,
    ctr: row.ctr || 0,
    position: row.position || 0,
  }))
  const pagesFile = path.join(outDir, 'gsc-pages.json')
  writeJSON(pagesFile, {
    siteUrl,
    startDate,
    endDate,
    request: pagesResult.payload,
    responseAggregationType: pagesResult.data.responseAggregationType || '',
    returnedRows: pageRows.length,
    truncated: pageRows.length >= rowLimit,
    exhaustive: false,
    rows: pageRows,
  })

  const queryPageResult = await request({ dimensions: ['query', 'page'] })
  const queryPageRows = (queryPageResult.data.rows || []).map((row) => ({
    query: row.keys?.[0] || '',
    page: row.keys?.[1] || '',
    clicks: row.clicks || 0,
    impressions: row.impressions || 0,
    ctr: row.ctr || 0,
    position: row.position || 0,
  }))
  const queryPageFile = path.join(outDir, 'gsc-query-page.json')
  writeJSON(queryPageFile, {
    siteUrl,
    startDate,
    endDate,
    request: queryPageResult.payload,
    responseAggregationType: queryPageResult.data.responseAggregationType || '',
    returnedRows: queryPageRows.length,
    truncated: queryPageRows.length >= rowLimit,
    exhaustive: false,
    rows: queryPageRows,
  })

  manifest.outputs.gsc = queryPageFile
  manifest.outputs.gscSummary = summaryFile
  manifest.outputs.gscPages = pagesFile
  manifest.outputs.gscQueryPage = queryPageFile
  manifest.sourceMetadata.gsc = {
    siteUrl,
    searchType: 'web',
    dataState: 'final',
    dateTimezone: 'America/Los_Angeles',
    filters: [],
    rowLimit,
    limitations: [
      'Search Analytics returns top rows rather than a guaranteed exhaustive export.',
      'Query dimensions may omit anonymized queries; query-page sums are lower bounds.',
    ],
  }
}

async function collectGA4() {
  const propertyId = envFirst(['GA4_PROPERTY_ID', 'GOOGLE_ANALYTICS_PROPERTY_ID'])
  if (!propertyId) {
    manifest.skipped.push({ source: 'ga4', reason: 'missing GA4_PROPERTY_ID' })
    return
  }
  const serviceAccount = loadServiceAccount('GA4') || loadServiceAccount('GOOGLE') || loadServiceAccount('GSC')
  if (!serviceAccount) {
    manifest.skipped.push({ source: 'ga4', reason: 'missing GA4_SERVICE_ACCOUNT_JSON(_B64/_PATH) or GOOGLE_SERVICE_ACCOUNT_JSON(_B64/_PATH)' })
    return
  }
  const token = await googleAccessToken(serviceAccount, 'https://www.googleapis.com/auth/analytics.readonly')
  const endpoint = `https://analyticsdata.googleapis.com/v1beta/properties/${propertyId}:runReport`
  const common = {
    dateRanges: [{ startDate, endDate }],
    limit: '10000',
  }
  const pagePayload = {
    ...common,
    dimensions: [{ name: 'pagePath' }, { name: 'sessionSourceMedium' }],
    metrics: [
      { name: 'sessions' },
      { name: 'totalUsers' },
      { name: 'screenPageViews' },
      { name: 'engagedSessions' },
      { name: 'eventCount' },
    ],
  }
  const pageData = await httpJSON(endpoint, {
    method: 'POST',
    headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
    body: JSON.stringify(pagePayload),
  })
  const pageRows = (pageData.rows || []).map((row) => ({
    pagePath: row.dimensionValues?.[0]?.value || '',
    sourceMedium: row.dimensionValues?.[1]?.value || '',
    sessions: Number(row.metricValues?.[0]?.value || 0),
    totalUsers: Number(row.metricValues?.[1]?.value || 0),
    screenPageViews: Number(row.metricValues?.[2]?.value || 0),
    engagedSessions: Number(row.metricValues?.[3]?.value || 0),
    eventCount: Number(row.metricValues?.[4]?.value || 0),
  }))
  writeJSON(path.join(outDir, 'ga4-pages.json'), { propertyId, startDate, endDate, rows: pageRows })
  manifest.outputs.ga4Pages = path.join(outDir, 'ga4-pages.json')

  const eventPayload = {
    ...common,
    dimensions: [{ name: 'eventName' }, { name: 'pagePath' }],
    metrics: [{ name: 'eventCount' }],
  }
  const eventData = await httpJSON(endpoint, {
    method: 'POST',
    headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
    body: JSON.stringify(eventPayload),
  })
  const eventRows = (eventData.rows || []).map((row) => ({
    eventName: row.dimensionValues?.[0]?.value || '',
    pagePath: row.dimensionValues?.[1]?.value || '',
    eventCount: Number(row.metricValues?.[0]?.value || 0),
  }))
  writeJSON(path.join(outDir, 'ga4-events.json'), { propertyId, startDate, endDate, rows: eventRows })
  manifest.outputs.ga4Events = path.join(outDir, 'ga4-events.json')

  try {
    const realtimeEndpoint = `https://analyticsdata.googleapis.com/v1beta/properties/${propertyId}:runRealtimeReport`
    const realtimeData = await httpJSON(realtimeEndpoint, {
      method: 'POST',
      headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
      body: JSON.stringify({
        dimensions: [{ name: 'eventName' }],
        metrics: [{ name: 'eventCount' }],
        limit: '50',
      }),
    })
    const realtimeRows = (realtimeData.rows || []).map((row) => ({
      eventName: row.dimensionValues?.[0]?.value || '',
      eventCount: Number(row.metricValues?.[0]?.value || 0),
    }))
    writeJSON(path.join(outDir, 'ga4-realtime.json'), { propertyId, generatedAt: new Date().toISOString(), rows: realtimeRows })
    manifest.outputs.ga4Realtime = path.join(outDir, 'ga4-realtime.json')
  } catch (error) {
    manifest.failed.push({ source: 'ga4Realtime', error: String(error.message || error).slice(0, 1000) })
  }
}

async function main() {
  await collectTechnicalAudit()
  for (const collector of [collectGSC, collectGA4]) {
    try {
      await collector()
    } catch (error) {
      manifest.failed.push({ source: collector.name.replace(/^collect/, '').toLowerCase(), error: String(error.message || error).slice(0, 2000) })
    }
  }

  writeJSON(path.join(outDir, 'manifest.json'), manifest)
  const lines = [
    `# SEO/GEO 数据采集 ${runDate}`,
    '',
    `周期：${startDate} 至 ${endDate}`,
    '',
    '## 输出',
    ...Object.entries(manifest.outputs).map(([key, value]) => `- ${key}: ${value}`),
    '',
    '## 跳过',
    ...(manifest.skipped.length ? manifest.skipped.map((item) => `- ${item.source}: ${item.reason}`) : ['- 无']),
    '',
    '## 失败',
    ...(manifest.failed.length ? manifest.failed.map((item) => `- ${item.source}: ${item.error || item.stderr || item.status}`) : ['- 无']),
    '',
  ]
  writeText(path.join(outDir, 'README.md'), lines.join('\n'))
  console.log(path.join(outDir, 'manifest.json'))
}

main().catch((error) => {
  console.error(error)
  process.exit(1)
})
