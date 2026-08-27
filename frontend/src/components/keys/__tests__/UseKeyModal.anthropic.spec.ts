import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

vi.mock('@/api/gatewayModels', () => ({
  getGatewayModels: vi.fn()
}))

import UseKeyModal from '../UseKeyModal.vue'

const mountModal = () => mount(UseKeyModal, {
  props: {
    show: true,
    apiKey: 'sk-anthropic-test',
    baseUrl: 'https://api.example.com',
    platform: 'anthropic',
    defaultMappedModel: 'glm-5.3'
  },
  global: {
    stubs: {
      BaseDialog: {
        template: '<div><slot /><slot name="footer" /></div>'
      },
      Icon: {
        template: '<span />'
      }
    }
  }
})

describe('UseKeyModal Anthropic config', () => {
  it('uses the group default instead of a catalog-wide Claude model', () => {
    const wrapper = mountModal()
    const settings = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('"effortLevel"'))

    expect(settings).toBeDefined()
    expect(settings).toContain('"model": "glm-5.3"')
    expect(settings).toContain('"ANTHROPIC_MODEL": "glm-5.3"')
    expect(settings).toContain('"ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.3"')
    expect(settings).toContain('"ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.3"')
    expect(settings).toContain('"ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.3"')
    expect(settings).not.toContain('claude-opus-5')
  })
})
