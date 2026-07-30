import type { MonthlyUpstreamProbeAccount } from '@/api/admin/monthlyUpstreams'

type DirectDiagnosticAccount = Pick<
  MonthlyUpstreamProbeAccount,
  'latest_status' | 'latest_checked_at' | 'latest_direct_upstream'
>

const unresolvedDirectStatuses = new Set(['failed', 'not_schedulable'])

function timestamp(value?: string | null): number | null {
  if (!value) return null
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : null
}

export function isDirectUpstreamDiagnosticStale(account: DirectDiagnosticAccount): boolean {
  const diagnostic = account.latest_direct_upstream
  if (!diagnostic || !unresolvedDirectStatuses.has(diagnostic.status)) return false
  if (account.latest_status !== 'ok') return false

  const gatewayCheckedAt = timestamp(account.latest_checked_at)
  const directCheckedAt = timestamp(diagnostic.checked_at)
  if (gatewayCheckedAt == null || directCheckedAt == null) return false

  return gatewayCheckedAt > directCheckedAt
}
