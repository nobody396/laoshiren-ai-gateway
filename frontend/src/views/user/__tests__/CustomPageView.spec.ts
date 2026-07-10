import { describe, expect, it } from 'vitest'
import source from '../CustomPageView.vue?raw'

describe('CustomPageView embed boundary', () => {
  it('uses explicit referrer, sandbox and permission contracts', () => {
    expect(source).toContain('referrerpolicy="no-referrer"')
    expect(source).toContain('sandbox="allow-scripts allow-forms allow-popups allow-same-origin"')
    expect(source).toContain('allow="clipboard-write"')
    expect(source).not.toContain('authStore.token')
  })
})
