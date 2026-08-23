import type { ServiceStatus, ServiceStatusProduct } from '@/api/serviceStatus'

export const SERVICE_STATUS_STALE_AFTER_MS = 2 * 60_000

export const serviceStatusLabels: Record<ServiceStatus, string> = {
  operational: '正常',
  degraded_performance: '性能下降',
  partial_outage: '部分中断',
  major_outage: '大范围中断',
  maintenance: '维护中',
  monitoring: '观察中'
}

const severity: Record<ServiceStatus, number> = {
  operational: 1,
  monitoring: 2,
  degraded_performance: 3,
  maintenance: 4,
  partial_outage: 5,
  major_outage: 6
}

export function serviceStatusDateMs(value?: string | null): number {
  if (!value) return 0
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

export function formatServiceStatusTime(value?: string | null): string {
  const parsed = serviceStatusDateMs(value)
  if (!parsed) return '时间未知'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false
  }).format(parsed)
}

export function isServiceStatusProductStale(product: ServiceStatusProduct, nowMs: number): boolean {
  const computedAt = serviceStatusDateMs(product.computed_at)
  return !computedAt || nowMs - computedAt > SERVICE_STATUS_STALE_AFTER_MS
}

export function maxServiceStatus(left: ServiceStatus, right: ServiceStatus): ServiceStatus {
  return severity[left] >= severity[right] ? left : right
}

export function serviceStatusForPresentation(product: ServiceStatusProduct, nowMs: number): ServiceStatus {
  return isServiceStatusProductStale(product, nowMs) ? 'monitoring' : product.status
}

export function worstServiceStatus(products: ServiceStatusProduct[], nowMs: number): ServiceStatus {
  return products.reduce<ServiceStatus>((worst, product) => maxServiceStatus(worst, serviceStatusForPresentation(product, nowMs)), 'operational')
}
