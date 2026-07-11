import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'

const { listRequestDetails } = vi.hoisted(() => ({
  listRequestDetails: vi.fn()
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: { listRequestDetails }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn().mockResolvedValue(true) })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'common.loading': 'Loading',
    'common.refresh': 'Refresh',
    'admin.ops.requestDetails.rangeHours': '1 hour',
    'admin.ops.requestDetails.rangeMinutes': '60 minutes',
    'admin.ops.requestDetails.rangeLabel': 'Window',
    'admin.ops.requestDetails.table.time': 'Time',
    'admin.ops.requestDetails.table.kind': 'Kind',
    'admin.ops.requestDetails.table.platform': 'Platform',
    'admin.ops.requestDetails.table.model': 'Model',
    'admin.ops.requestDetails.table.duration': 'Duration',
    'admin.ops.requestDetails.table.firstToken': 'TTFT',
    'admin.ops.requestDetails.table.status': 'Status',
    'admin.ops.requestDetails.table.requestId': 'Request ID',
    'admin.ops.requestDetails.table.actions': 'Actions',
    'admin.ops.requestDetails.kind.success': 'SUCCESS',
    'admin.ops.requestDetails.kind.error': 'ERROR',
    'admin.ops.requestDetails.copy': 'Copy',
    'admin.ops.requestDetails.viewError': 'View Error'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

describe('OpsRequestDetailsModal', () => {
  beforeEach(() => {
    listRequestDetails.mockReset()
    listRequestDetails.mockResolvedValue({
      items: [{
        kind: 'success',
        created_at: '2026-07-11T00:00:00Z',
        request_id: 'req-owned',
        platform: 'anthropic',
        model: 'model-a',
        duration_ms: 500,
        first_token_ms: 120,
        stream: true
      }],
      total: 1,
      page: 1,
      page_size: 10
    })
  })

  it('sorts by and displays first-token latency for a TTFT drilldown', async () => {
    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: false,
        timeRange: '1h',
        preset: { title: 'TTFT', sort: 'first_token_desc' },
        platform: '',
        groupId: null
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<section v-if="show"><h2>{{ title }}</h2><slot /></section>'
          },
          Pagination: true
        }
      }
    })

    await wrapper.setProps({ modelValue: true })
    await flushPromises()

    expect(listRequestDetails).toHaveBeenCalledTimes(1)
    expect(listRequestDetails.mock.calls[0]?.[0]).toMatchObject({ sort: 'first_token_desc' })
    expect(wrapper.text()).toContain('TTFT')
    expect(wrapper.text()).toContain('120 ms')
    expect(wrapper.text()).not.toContain('500 ms')
  })
})
