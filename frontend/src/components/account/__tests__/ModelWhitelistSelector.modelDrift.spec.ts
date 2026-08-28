import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { inspectMock } = vi.hoisted(() => ({ inspectMock: vi.fn() }))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: { inspectOpenAIModelDrift: inspectMock }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() })
}))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

describe('ModelWhitelistSelector upstream model drift', () => {
  beforeEach(() => {
    inspectMock.mockReset()
    inspectMock.mockResolvedValue({
      status: 'review_required',
      applied: false,
      upstream_models: ['gpt-5.6-sol', 'gpt-5.7'],
      configured_request_models: ['gpt-current'],
      configured_upstream_models: ['gpt-5.6-sol'],
      upstream_unmapped: ['gpt-5.7'],
      configured_missing_upstream: []
    })
  })

  it('shows a read-only drift result and only changes the form after explicit review action', async () => {
    const wrapper = mount(ModelWhitelistSelector, {
      props: { modelValue: ['gpt-current'], platform: 'openai', accountId: 42 },
      global: { stubs: { ModelIcon: true, Icon: true } }
    })

    await wrapper.get('[data-testid="inspect-upstream-models"]').trigger('click')
    await vi.waitFor(() => expect(inspectMock).toHaveBeenCalledWith(42))
    expect(wrapper.text()).toContain('gpt-5.7')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await wrapper.get('[data-testid="add-reviewed-upstream-models"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual(['gpt-current', 'gpt-5.7'])
  })
})
