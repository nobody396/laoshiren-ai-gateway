import type { ModelStat } from '@/types'
import type { ReceiptModelLine } from './types'

const BEIJING_TIME_ZONE = 'Asia/Shanghai'

export function formatCompactNumber(value: number): string {
  const safeValue = Number.isFinite(value) ? Math.max(0, value) : 0
  if (safeValue >= 1_000_000_000) return `${trimTrailingZero(safeValue / 1_000_000_000)}B`
  if (safeValue >= 1_000_000) return `${trimTrailingZero(safeValue / 1_000_000)}M`
  if (safeValue >= 1_000) return `${trimTrailingZero(safeValue / 1_000)}K`
  return Math.round(safeValue).toLocaleString('zh-CN')
}

function trimTrailingZero(value: number): string {
  const formatted = value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)
  return formatted.includes('.') ? formatted.replace(/0+$/, '').replace(/\.$/, '') : formatted
}

export function formatModelName(model: string): string {
  const cleaned = model
    .trim()
    .replace(/-\d{8}$/, '')
    .replace(/^openai\//i, '')
    .replace(/^anthropic\//i, '')

  if (!cleaned) return '未知模型'

  return cleaned
    .split('-')
    .map((part) => {
      const lower = part.toLowerCase()
      if (['gpt', 'api', 'ai', 'xl'].includes(lower)) return lower.toUpperCase()
      if (/^\d+(?:\.\d+)?$/.test(part)) return part
      return part.charAt(0).toUpperCase() + part.slice(1)
    })
    .join(' ')
}

export function buildReceiptModelLines(models: ModelStat[], limit = 5): ReceiptModelLine[] {
  const sorted = [...models]
    .filter((model) => model.requests > 0 || model.total_tokens > 0 || model.actual_cost > 0)
    .sort((left, right) => right.actual_cost - left.actual_cost || right.total_tokens - left.total_tokens)

  const visible = sorted.slice(0, limit).map(toReceiptModelLine)
  const hidden = sorted.slice(limit)

  if (hidden.length > 0) {
    visible.push({
      model: `其他 ${hidden.length} 个模型`,
      requests: hidden.reduce((sum, model) => sum + model.requests, 0),
      totalTokens: hidden.reduce((sum, model) => sum + model.total_tokens, 0),
      actualCost: hidden.reduce((sum, model) => sum + model.actual_cost, 0)
    })
  }

  return visible
}

function toReceiptModelLine(model: ModelStat): ReceiptModelLine {
  return {
    model: formatModelName(model.model),
    requests: model.requests,
    totalTokens: model.total_tokens,
    actualCost: model.actual_cost
  }
}

export function createReceiptNumber(now = new Date()): string {
  const date = new Intl.DateTimeFormat('en-CA', {
    timeZone: BEIJING_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  }).format(now).split('-').join('')

  const bytes = new Uint8Array(3)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    const fallback = now.getTime()
    bytes[0] = fallback & 255
    bytes[1] = (fallback >> 8) & 255
    bytes[2] = (fallback >> 16) & 255
  }
  const suffix = Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('').toUpperCase()
  return `LSR-${date}-${suffix}`
}

export function formatBeijingDateTime(date: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: BEIJING_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(date).split('/').join('-')
}

export function formatReceiptDateRange(startDate: string, endDate: string): string {
  if (startDate === endDate) return startDate
  return `${startDate} — ${endDate}`
}

export function buildInviteUrl(origin: string, inviteCode: string): string {
  const normalizedOrigin = origin.replace(/\/$/, '')
  if (!inviteCode) return `${normalizedOrigin}/register`
  return `${normalizedOrigin}/register?ref=${encodeURIComponent(inviteCode)}`
}

export function receiptImageFilename(startDate: string, endDate: string): string {
  return `老实人AI-使用小票-${startDate}-${endDate}.png`
}
