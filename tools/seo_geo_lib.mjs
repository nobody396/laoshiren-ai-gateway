#!/usr/bin/env node
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { execFileSync } from 'node:child_process'

export const ROOT = process.cwd()
export const SEO_DIR = path.join(ROOT, 'docs/ops/seo-geo')
export const SITE_ORIGIN = process.env.SEO_GEO_SITE_ORIGIN || 'https://laoshirenai.com'
export const P0_PATHS = [
  '/docs/claude-code-china-guide',
  '/docs/codex-china-guide',
  '/docs/codex-no-api-key-guide',
  '/docs/codex-custom-api-guide',
]

export function todayISO() {
  return new Date().toISOString().slice(0, 10)
}

export function ymdDaysAgo(days) {
  const date = new Date(Date.now() - days * 24 * 60 * 60 * 1000)
  return date.toISOString().slice(0, 10)
}

export function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true })
}

export function writeJSON(file, value) {
  ensureDir(path.dirname(file))
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`, 'utf8')
}

export function readJSON(file, fallback = null) {
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'))
  } catch {
    return fallback
  }
}

export function writeText(file, value) {
  ensureDir(path.dirname(file))
  fs.writeFileSync(file, value, 'utf8')
}

export function commandExists(cmd) {
  try {
    execFileSync('bash', ['-lc', `command -v ${cmd}`], { stdio: 'ignore' })
    return true
  } catch {
    return false
  }
}

export function envFirst(names) {
  for (const name of names) {
    const value = process.env[name]
    if (value && value.trim()) return value.trim()
  }
  return ''
}

export function loadServiceAccount(prefix) {
  const raw = envFirst([`${prefix}_SERVICE_ACCOUNT_JSON`, `${prefix}_SERVICE_ACCOUNT`])
  const b64 = envFirst([`${prefix}_SERVICE_ACCOUNT_JSON_B64`, `${prefix}_SERVICE_ACCOUNT_B64`])
  const file = envFirst([`${prefix}_SERVICE_ACCOUNT_JSON_PATH`, `${prefix}_SERVICE_ACCOUNT_PATH`])
  const fallbackRaw = prefix !== 'GOOGLE' ? envFirst(['GOOGLE_SERVICE_ACCOUNT_JSON']) : ''
  const fallbackB64 = prefix !== 'GOOGLE' ? envFirst(['GOOGLE_SERVICE_ACCOUNT_JSON_B64']) : ''
  const fallbackFile = prefix !== 'GOOGLE' ? envFirst(['GOOGLE_SERVICE_ACCOUNT_JSON_PATH']) : ''

  const source = raw || fallbackRaw
  if (source) return JSON.parse(source)
  const sourceB64 = b64 || fallbackB64
  if (sourceB64) return JSON.parse(Buffer.from(sourceB64, 'base64').toString('utf8'))
  const sourceFile = file || fallbackFile
  if (sourceFile) return JSON.parse(fs.readFileSync(sourceFile, 'utf8'))
  return null
}

function base64url(value) {
  return Buffer.from(value).toString('base64url')
}

export async function googleAccessToken(serviceAccount, scope) {
  if (!serviceAccount?.client_email || !serviceAccount?.private_key) {
    throw new Error('service account missing client_email/private_key')
  }
  const now = Math.floor(Date.now() / 1000)
  const header = { alg: 'RS256', typ: 'JWT' }
  const payload = {
    iss: serviceAccount.client_email,
    scope,
    aud: 'https://oauth2.googleapis.com/token',
    exp: now + 3600,
    iat: now,
  }
  const unsigned = `${base64url(JSON.stringify(header))}.${base64url(JSON.stringify(payload))}`
  const signature = crypto.createSign('RSA-SHA256').update(unsigned).sign(serviceAccount.private_key, 'base64url')
  const assertion = `${unsigned}.${signature}`
  const body = new URLSearchParams({ grant_type: 'urn:ietf:params:oauth:grant-type:jwt-bearer', assertion }).toString()
  const response = await fetch('https://oauth2.googleapis.com/token', {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body,
  })
  const text = await response.text()
  if (!response.ok) throw new Error(`google token error ${response.status}: ${text}`)
  return JSON.parse(text).access_token
}

export async function httpJSON(url, options = {}) {
  const response = await fetch(url, options)
  const text = await response.text()
  let data = null
  try { data = text ? JSON.parse(text) : null } catch { data = { raw: text } }
  if (!response.ok) {
    const detail = typeof data === 'object' ? JSON.stringify(data).slice(0, 800) : String(data).slice(0, 800)
    throw new Error(`HTTP ${response.status} ${url}: ${detail}`)
  }
  return data
}

export function markdownTable(rows, columns) {
  const esc = (v) => String(v ?? '').replace(/\|/g, '\\|').replace(/\n/g, ' ')
  return [
    `| ${columns.map((c) => esc(c.label)).join(' | ')} |`,
    `| ${columns.map(() => '---').join(' | ')} |`,
    ...rows.map((row) => `| ${columns.map((c) => esc(typeof c.value === 'function' ? c.value(row) : row[c.value])).join(' | ')} |`),
  ].join('\n')
}

export function latestSubdir(dir) {
  if (!fs.existsSync(dir)) return ''
  return fs.readdirSync(dir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort()
    .at(-1) || ''
}

export function safeNumber(value) {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}
