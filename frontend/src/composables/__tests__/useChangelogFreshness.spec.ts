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
})
