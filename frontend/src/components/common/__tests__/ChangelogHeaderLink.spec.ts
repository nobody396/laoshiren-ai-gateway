import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChangelogHeaderLink from '../ChangelogHeaderLink.vue'

const freshness = vi.hoisted(() => ({
  isNew: true,
  refresh: vi.fn()
}))

vi.mock('@/composables/useChangelogFreshness', async () => {
  const { computed } = await vi.importActual<typeof import('vue')>('vue')
  return {
    useChangelogFreshness: () => ({
      hasNewChangelog: computed(() => freshness.isNew),
      refreshChangelogFreshness: freshness.refresh
    })
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'nav.changelog': '更新日志',
    'changelog.newUpdate': '有新的产品进展'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

function mountLink() {
  return mount(ChangelogHeaderLink, {
    global: {
      stubs: {
        Icon: true,
        RouterLink: {
          props: ['to'],
          template: '<a :href="to"><slot /></a>'
        }
      }
    }
  })
}

describe('ChangelogHeaderLink', () => {
  beforeEach(() => {
    freshness.isNew = true
    freshness.refresh.mockReset()
    freshness.refresh.mockResolvedValue(undefined)
  })

  it('links to the changelog and highlights a new published update', async () => {
    const wrapper = mountLink()
    await flushPromises()

    const link = wrapper.get('[data-testid="changelog-header-link"]')
    expect(link.attributes('href')).toBe('/changelog')
    expect(link.attributes('aria-label')).toBe('有新的产品进展')
    expect(link.text()).toContain('更新日志')
    expect(wrapper.find('[data-testid="changelog-new-dot"]').exists()).toBe(true)
    expect(freshness.refresh).toHaveBeenCalledTimes(1)
  })

  it('keeps the header entry visible without a dot after updates are seen', async () => {
    freshness.isNew = false
    const wrapper = mountLink()
    await flushPromises()

    expect(wrapper.get('[data-testid="changelog-header-link"]').attributes('aria-label'))
      .toBe('更新日志')
    expect(wrapper.find('[data-testid="changelog-new-dot"]').exists()).toBe(false)
  })
})
