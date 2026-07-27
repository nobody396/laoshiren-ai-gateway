import type { Group } from '@/types'

export const SUBSCRIPTION_CREDIT_DISPLAY_SCALE = 10

type SubscriptionUsageGroup = Pick<Group, 'subscription_type'> | null | undefined

function safeNumber(value: number | null | undefined): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function safePositiveNumber(value: number | null | undefined): number | null {
  const numberValue = safeNumber(value)
  return numberValue > 0 ? numberValue : null
}

export function isCreditSubscriptionGroup(group: SubscriptionUsageGroup): boolean {
  return group?.subscription_type === 'credit'
}

export function toSubscriptionDisplayCredits(value: number | null | undefined): number {
  return Math.max(safeNumber(value), 0) * SUBSCRIPTION_CREDIT_DISPLAY_SCALE
}

export function subscriptionUsagePercent(
  used: number | null | undefined,
  limit: number | null | undefined
): number {
  const safeLimit = safePositiveNumber(limit)
  if (!safeLimit) return 0
  const percentage = (Math.max(safeNumber(used), 0) / safeLimit) * 100
  return Math.min(Math.max(percentage, 0), 100)
}

export function formatSubscriptionUsagePercent(
  used: number | null | undefined,
  limit: number | null | undefined
): string {
  return `${subscriptionUsagePercent(used, limit).toFixed(1)}%`
}

export function formatSubscriptionCredits(value: number | null | undefined): string {
  const credits = toSubscriptionDisplayCredits(value)
  return new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: credits >= 100 ? 0 : 1
  }).format(credits)
}

function formatPlatformCredit(value: number | null | undefined): string {
  return `⚡${safeNumber(value).toFixed(2)}`
}

export function formatSubscriptionUsageValue(
  value: number | null | undefined,
  group: SubscriptionUsageGroup
): string {
  if (isCreditSubscriptionGroup(group)) {
    return `${formatSubscriptionCredits(value)} credits`
  }
  return formatPlatformCredit(value)
}

export function formatSubscriptionUsageRatio(
  used: number | null | undefined,
  limit: number | null | undefined,
  group: SubscriptionUsageGroup
): string {
  if (!limit || limit <= 0) {
    if (isCreditSubscriptionGroup(group)) {
      return `${formatSubscriptionCredits(used)} / ∞ credits`
    }
    return `${formatPlatformCredit(used)} / ∞`
  }
  if (isCreditSubscriptionGroup(group)) {
    return `${formatSubscriptionCredits(used)} / ${formatSubscriptionCredits(limit)} credits`
  }
  return `${formatPlatformCredit(used)} / ${formatPlatformCredit(limit)}`
}

export function formatSubscriptionUsageDisplay(
  used: number | null | undefined,
  limit: number | null | undefined,
  group: SubscriptionUsageGroup
): string {
  const percent = formatSubscriptionUsagePercent(used, limit)
  return `${formatSubscriptionUsageRatio(used, limit, group)} · 已用 ${percent}`
}
