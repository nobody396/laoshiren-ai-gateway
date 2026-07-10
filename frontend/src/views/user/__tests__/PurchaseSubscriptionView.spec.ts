import { describe, expect, it } from 'vitest'
import source from '../PurchaseSubscriptionView.vue?raw'

describe('PurchaseSubscriptionView embed boundary', () => {
  it('uses explicit referrer, sandbox and payment permission contracts', () => {
    expect(source).toContain('referrerpolicy="no-referrer"')
    expect(source).toContain('sandbox="allow-scripts allow-forms allow-popups allow-same-origin"')
    expect(source).toContain('allow="payment"')
    expect(source).not.toContain('authStore.token')
  })
})
