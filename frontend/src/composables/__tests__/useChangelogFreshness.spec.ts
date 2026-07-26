import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockLatest = vi.fn()

vi.mock('@/api', () => ({
  changelogAPI: {
    latest: (...args: unknown[]) => mockLatest(...args)
  }
}))

describe('useChangelogFreshness', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    window.localStorage.clear()
  })

  it('shows a new-update dot until the latest published entry is seen', async () => {
    mockLatest.mockResolvedValue({
      id: 1,
      slug: 'first-public-update',
      published_at: '2026-07-25T12:00:00Z'
    })
    const { useChangelogFreshness } = await import('../useChangelogFreshness')
    const sidebarFreshness = useChangelogFreshness()
    const pageFreshness = useChangelogFreshness()

    await sidebarFreshness.refreshChangelogFreshness()

    expect(sidebarFreshness.hasNewChangelog.value).toBe(true)

    pageFreshness.markChangelogSeen('2026-07-25T12:00:00Z')

    expect(sidebarFreshness.hasNewChangelog.value).toBe(false)
    expect(window.localStorage.getItem('laoshirenai:changelog:last-seen-at'))
      .toBe('2026-07-25T12:00:00Z')
  })

  it('does not show a dot when there are no published updates', async () => {
    mockLatest.mockResolvedValue(null)
    const { useChangelogFreshness } = await import('../useChangelogFreshness')
    const freshness = useChangelogFreshness()

    await freshness.refreshChangelogFreshness()

    expect(freshness.hasNewChangelog.value).toBe(false)
  })

  it('does not move the last-seen watermark backward for an older detail page', async () => {
    mockLatest.mockResolvedValue({
      id: 2,
      slug: 'latest-update',
      published_at: '2026-07-25T12:00:00Z'
    })
    const { useChangelogFreshness } = await import('../useChangelogFreshness')
    const freshness = useChangelogFreshness()

    await freshness.refreshChangelogFreshness()
    freshness.markChangelogSeen('2026-07-25T12:00:00Z')
    freshness.markChangelogSeen('2026-07-20T12:00:00Z')

    expect(freshness.hasNewChangelog.value).toBe(false)
    expect(window.localStorage.getItem('laoshirenai:changelog:last-seen-at'))
      .toBe('2026-07-25T12:00:00Z')
  })

  it('deduplicates concurrent freshness checks from the header and sidebar', async () => {
    let resolveLatest: ((value: unknown) => void) | undefined
    mockLatest.mockImplementation(() => new Promise((resolve) => {
      resolveLatest = resolve
    }))
    const { useChangelogFreshness } = await import('../useChangelogFreshness')
    const headerFreshness = useChangelogFreshness()
    const sidebarFreshness = useChangelogFreshness()

    const headerRequest = headerFreshness.refreshChangelogFreshness()
    const sidebarRequest = sidebarFreshness.refreshChangelogFreshness()

    expect(mockLatest).toHaveBeenCalledTimes(1)
    resolveLatest?.({
      id: 3,
      slug: 'shared-update',
      published_at: '2026-07-25T12:00:00Z'
    })
    await Promise.all([headerRequest, sidebarRequest])

    expect(headerFreshness.hasNewChangelog.value).toBe(true)
    expect(sidebarFreshness.hasNewChangelog.value).toBe(true)
  })
})
