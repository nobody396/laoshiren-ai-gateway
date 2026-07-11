import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import OpsDashboardHeader from '../OpsDashboardHeader.vue'

vi.mock('@/api', () => ({
  adminAPI: {
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRealtimeTrafficSummary: vi.fn()
  }
}))

vi.mock('@/stores', () => ({
  useAdminSettingsStore: () => ({
    opsRealtimeMonitoringEnabled: false,
    setOpsRealtimeMonitoringEnabledLocal: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('OpsDashboardHeader', () => {
  it('opens the TTFT drilldown with first-token sorting', async () => {
    const wrapper = mount(OpsDashboardHeader, {
      props: {
        overview: {} as any,
        platform: '',
        groupId: null,
        timeRange: '1h',
        queryMode: 'auto',
        loading: false,
        lastUpdated: null,
        fullscreen: false
      },
      global: {
        stubs: {
          Select: true,
          HelpTooltip: true,
          BaseDialog: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    const ttftCard = wrapper.findAll('div.rounded-2xl').find((card) =>
      card.findAll('span').some((span) => span.text().trim() === 'TTFT')
    )
    expect(ttftCard).toBeDefined()

    const detailsButton = ttftCard!.find('button')
    expect(detailsButton.exists()).toBe(true)
    await detailsButton.trigger('click')

    expect(wrapper.emitted('openRequestDetails')?.at(-1)?.[0]).toEqual({
      title: 'admin.ops.ttftLabel',
      sort: 'first_token_desc'
    })
  })
})
