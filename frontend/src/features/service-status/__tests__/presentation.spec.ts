import { describe, expect, it } from 'vitest'
import { serviceStatusForPresentation } from '../presentation'
import type { ServiceStatusProduct } from '@/api/serviceStatus'

function product(status: ServiceStatusProduct['status'], computedAt: string): ServiceStatusProduct {
  return { code: status, display_name: status, status, reason: 'test', computed_at: computedAt, components: [] }
}

describe('service status presentation', () => {
  it('uses Monitoring for every stale product instead of presenting old state as current', () => {
    const now = Date.parse('2026-08-23T00:10:00Z')
    const stale = '2026-08-23T00:00:00Z'
    expect(serviceStatusForPresentation(product('operational', stale), now)).toBe('monitoring')
    expect(serviceStatusForPresentation(product('partial_outage', stale), now)).toBe('monitoring')
    expect(serviceStatusForPresentation(product('major_outage', stale), now)).toBe('monitoring')
  })
})
