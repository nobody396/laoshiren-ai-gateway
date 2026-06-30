#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { SEO_DIR, todayISO, ensureDir, readJSON, writeJSON, writeText } from './seo_geo_lib.mjs'

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
const days = args.get('days') || process.env.SEO_GEO_DAYS || '28'
const apply = args.has('apply') || process.env.SEO_GEO_AUTO_APPLY === '1'
const commit = args.has('commit') || process.env.SEO_GEO_AUTO_COMMIT === '1'
const push = args.has('push') || process.env.SEO_GEO_AUTO_PUSH === '1'
const deploy = args.has('deploy') || process.env.SEO_GEO_AUTO_DEPLOY === '1'
const radar = args.has('radar') || process.env.SEO_GEO_RUN_RADAR === '1'
const allowDirty = args.has('allow-dirty') || process.env.SEO_GEO_ALLOW_DIRTY === '1'
const root = process.cwd()
const runDir = path.join(SEO_DIR, 'runs')
ensureDir(runDir)

const steps = []
function runStep(name, command, options = {}) {
  const startedAt = new Date().toISOString()
  const result = spawnSync(command, {
    cwd: options.cwd || root,
    shell: true,
    encoding: 'utf8',
    env: process.env,
    maxBuffer: 20 * 1024 * 1024,
  })
  const step = {
    name,
    command: sanitizeCommand(command),
    cwd: options.cwd || root,
    status: result.status ?? 0,
    startedAt,
    finishedAt: new Date().toISOString(),
    stdout: (result.stdout || '').slice(-8000),
    stderr: (result.stderr || '').slice(-8000),
  }
  steps.push(step)
  if (result.status !== 0 && !options.allowFailure) {
    throw new Error(`${name} failed with status ${result.status}`)
  }
  return step
}

function sanitizeCommand(command) {
  return String(command).replace(/(API_KEY|TOKEN|SECRET|PASSWORD)=([^\s]+)/gi, '$1=<redacted>')
}

function gitStatus() {
  const result = spawnSync('git status --short', { cwd: root, shell: true, encoding: 'utf8' })
  return result.stdout.trim().split('\n').map((x) => x.trim()).filter(Boolean)
}

function changedFiles() {
  return gitStatus().map((line) => line.replace(/^..\s+/, '').replace(/^.* -> /, ''))
}

function isProductChange(file) {
  return [
    'frontend/src/',
    'frontend/public/',
    'backend/internal/web/dist/',
    'tools/seo_geo_',
    '.github/workflows/',
  ].some((prefix) => file.startsWith(prefix))
}

function classifyGitChanges(files) {
  return {
    files,
    productFiles: files.filter(isProductChange),
    reportFiles: files.filter((f) => f.startsWith('docs/ops/seo-geo/')),
  }
}

function writeRunReport(extra = {}) {
  const files = changedFiles()
  const classified = classifyGitChanges(files)
  const manifest = readJSON(path.join(SEO_DIR, 'data', date, 'manifest.json'), {})
  const decision = readJSON(path.join(SEO_DIR, 'weekly', `${date}-decision.json`), {})
  const repair = readJSON(path.join(SEO_DIR, 'repairs', `${date}.json`), {})
  const report = {
    date,
    generatedAt: new Date().toISOString(),
    mode: { apply, commit, push, deploy, radar, allowDirty },
    dataAvailability: {
      technicalAudit: Boolean(manifest.outputs?.technicalAudit),
      gsc: Boolean(manifest.outputs?.gsc),
      ga4Pages: Boolean(manifest.outputs?.ga4Pages),
      ga4Events: Boolean(manifest.outputs?.ga4Events),
    },
    dataFailures: manifest.failed || [],
    pages: decision.pages || [],
    repair,
    git: classified,
    steps: steps.map((s) => ({ name: s.name, status: s.status, command: s.command })),
    ...extra,
  }
  const jsonPath = path.join(runDir, `${date}-closed-loop.json`)
  const mdPath = path.join(runDir, `${date}-closed-loop.md`)
  writeJSON(jsonPath, report)
  writeText(mdPath, renderReport(report))
  return { jsonPath, mdPath, report }
}

function renderReport(report) {
  const failedSteps = report.steps.filter((s) => s.status !== 0)
  const actions = (report.pages || []).flatMap((p) => (p.actions || []).map((a) => `${a.priority} ${p.path} ${a.type}: ${a.action}（${a.reason}）`))
  return [
    `# SEO/GEO 全链路闭环 ${report.date}`,
    '',
    '## 结论',
    '',
    `- 数据：技术=${report.dataAvailability.technicalAudit ? '有' : '缺'}，GSC=${report.dataAvailability.gsc ? '有' : '缺'}，GA4页面=${report.dataAvailability.ga4Pages ? '有' : '缺'}，GA4事件=${report.dataAvailability.ga4Events ? '有' : '缺'}`,
    `- 自动修复：${report.repair?.changedCount || 0} 个改动`,
    `- 产品改动：${report.git.productFiles.length} 个文件`,
    `- 自动部署：${report.deployment?.attempted ? report.deployment.status : '未触发'}`,
    `- 失败步骤：${failedSteps.length}`,
    '',
    '## 动作队列',
    '',
    actions.length ? actions.map((x) => `- ${x}`).join('\n') : '- 无',
    '',
    '## 已变更文件',
    '',
    report.git.files.length ? report.git.files.map((x) => `- ${x}`).join('\n') : '- 无',
    '',
    '## 数据失败',
    '',
    report.dataFailures.length ? report.dataFailures.map((x) => `- ${x.source}: ${x.error || x.reason || x.status || 'unknown'}`).join('\n') : '- 无',
    '',
    '## 执行步骤',
    '',
    report.steps.map((s) => `- ${s.status === 0 ? 'OK' : 'FAIL'} ${s.name}`).join('\n'),
    '',
  ].join('\n')
}

async function main() {
  const initialStatus = gitStatus()
  if (initialStatus.length && !allowDirty) {
    steps.push({ name: 'preflight', status: 2, command: 'git status --short', stdout: initialStatus.join('\n'), stderr: 'dirty worktree; aborting to avoid clobbering local work' })
    writeRunReport({ blocked: true, blocker: 'dirty_worktree' })
    console.error('Dirty worktree. Re-run with --allow-dirty only when these changes belong to the SEO/GEO loop.')
    process.exit(2)
  }

  runStep('collect', `node tools/seo_geo_collect.mjs --date ${quote(date)} --days ${quote(days)}`)
  runStep('decision', `node tools/seo_geo_decision.mjs --date ${quote(date)}`)
  if (radar) runStep('intent-radar', `node tools/seo_geo_intent_radar.mjs --date ${quote(date)}`, { allowFailure: true })
  if (apply) runStep('auto-repair', `node tools/seo_geo_auto_repair.mjs --date ${quote(date)} --apply`)

  const beforeBuildFiles = changedFiles()
  const beforeBuildProduct = beforeBuildFiles.filter(isProductChange)
  if (apply && beforeBuildProduct.length) {
    runStep('frontend-build', 'npm run build', { cwd: path.join(root, 'frontend') })
  }

  let deployment = { attempted: false, status: 'not_needed', reason: 'no_product_change' }
  const reportBeforeCommit = writeRunReport({ deployment })
  const filesBeforeCommit = changedFiles()
  const productFilesBeforeCommit = filesBeforeCommit.filter(isProductChange)

  if (commit && filesBeforeCommit.length) {
    runStep('git-add', 'git add frontend/src frontend/public backend/internal/web/dist tools/seo_geo_*.mjs docs/ops/seo-geo')
    const diffCached = spawnSync('git diff --cached --quiet', { cwd: root, shell: true })
    if (diffCached.status !== 0) {
      runStep('git-commit', `git commit -m ${quote('feat(seo): close geo seo self-healing loop')}`)
      if (push) runStep('git-push', 'git push')
    }
  }

  if (deploy && productFilesBeforeCommit.length) {
    deployment = { attempted: true, status: 'running', reason: 'product_change_detected' }
    writeRunReport({ deployment })
    runStep('production-deploy', 'bash /Users/fujunhao/laoshirenai/.agents/skills/laoshirenai-deploy/scripts/release-after-push.sh --deploy --confirm-production-deploy')
    deployment = { attempted: true, status: 'succeeded', reason: 'product_change_detected' }
  } else if (deploy) {
    deployment = { attempted: false, status: 'skipped', reason: 'no_product_change' }
  }

  writeRunReport({ deployment, committed: commit, pushed: push })
  console.log(reportBeforeCommit.mdPath)
}

function quote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`
}

main().catch((error) => {
  steps.push({ name: 'fatal', status: 1, command: 'seo_geo_closed_loop', stderr: String(error.message || error) })
  try { writeRunReport({ failed: true, error: String(error.message || error) }) } catch {}
  console.error(error)
  process.exit(1)
})
