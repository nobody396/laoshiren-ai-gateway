import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CommissionsView from '../CommissionsView.vue'

const getAgentCommissions = vi.hoisted(() => vi.fn())

vi.mock('@/api/agent', () => ({
  getAgentCommissions
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountView() {
  return mount(CommissionsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' }
      }
    }
  })
}

describe('agent CommissionsView self-consumption records', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getAgentCommissions.mockReset()
    getAgentCommissions.mockResolvedValue({
      items: [
        {
          id: 1,
          beneficiary_id: 8,
          user_id: 8,
          amount: 1,
          source_amount: 10,
          type: 'consumption_commission',
          created_at: '2026-07-30T08:00:00Z'
        },
        {
          id: 2,
          beneficiary_id: 8,
          user_id: 19,
          user_email: 'cu***@example.com',
          amount: 1,
          source_amount: 10,
          type: 'consumption_commission',
          created_at: '2026-07-30T08:00:00Z'
        }
      ],
      pagination: { page: 1, page_size: 20, total: 2, total_pages: 1 }
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('labels same-beneficiary consumption as own usage while keeping downstream labels', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('agent.type_self_consumption_commission')
    expect(wrapper.text()).toContain('agent.selfConsumptionTrigger')
    expect(wrapper.text()).toContain('agent.type_consumption_commission')

    wrapper.unmount()
  })

  it('sends the native self-consumption filter without changing the existing consumption filter', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('select').setValue('self_consumption_commission')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(getAgentCommissions).toHaveBeenLastCalledWith(expect.objectContaining({
      type: 'self_consumption_commission'
    }))

    await wrapper.get('select').setValue('consumption')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()

    expect(getAgentCommissions).toHaveBeenLastCalledWith(expect.objectContaining({
      type: 'consumption'
    }))

    wrapper.unmount()
  })
})
