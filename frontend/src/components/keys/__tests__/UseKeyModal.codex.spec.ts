import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { copyToClipboardMock, getGatewayModelsMock } = vi.hoisted(() => ({
  copyToClipboardMock: vi.fn().mockResolvedValue(true),
  getGatewayModelsMock: vi.fn()
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyToClipboardMock
  })
}))

vi.mock('@/api/gatewayModels', () => ({
  getGatewayModels: getGatewayModelsMock
}))

import UseKeyModal from '../UseKeyModal.vue'

const mountModal = (overrideProps: Record<string, unknown> = {}) =>
  mount(UseKeyModal, {
    props: {
      show: true,
      apiKey: 'sk-codex-test',
      baseUrl: 'https://example.com/v1',
      platform: 'openai',
      ...overrideProps
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

const codeBlocks = (wrapper: ReturnType<typeof mountModal>): string[] =>
  wrapper.findAll('pre code').map((code) => code.text())

const catalogBlock = (wrapper: ReturnType<typeof mountModal>): string => {
  const block = codeBlocks(wrapper).find((content) => content.includes('"slug":'))
  expect(block).toBeDefined()
  return block!
}

const configBlock = (wrapper: ReturnType<typeof mountModal>): string => {
  const block = codeBlocks(wrapper).find((content) => content.includes('model_provider = "OpenAI"'))
  expect(block).toBeDefined()
  return block!
}

describe('UseKeyModal Codex catalog', () => {
  beforeEach(() => {
    getGatewayModelsMock.mockReset()
  })

  it('lists the 7 selectable enterprise group models in the downloaded catalog', async () => {
    const enterpriseModels = [
      'gpt-5.4',
      'gpt-5.4-mini',
      'gpt-5.5',
      'gpt-5.6-luna',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.3-codex-spark',
      'codex-auto-review'
    ]
    getGatewayModelsMock.mockResolvedValue(enterpriseModels)

    const wrapper = mountModal({ defaultMappedModel: 'gpt-5.5' })
    await flushPromises()

    expect(getGatewayModelsMock).toHaveBeenCalledWith('https://example.com/v1', 'sk-codex-test')
    const catalog = catalogBlock(wrapper)
    // codex-auto-review is the auto-review feature target, not user-pickable.
    const selectable = enterpriseModels.filter((model) => model !== 'codex-auto-review')
    expect(catalog.match(/"slug":/g)).toHaveLength(7)
    for (const model of selectable) {
      expect(catalog).toContain(`"slug": "${model}"`)
    }
    expect(catalog).not.toContain('codex-auto-review')
    // Enterprise group default_mapped_model is gpt-5.5 in production.
    expect(configBlock(wrapper)).toContain('model = "gpt-5.5"')
  })

  it('keeps the legacy Pro 20X style catalog at exactly five models on gpt-5.5', async () => {
    getGatewayModelsMock.mockResolvedValue([
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6',
      'gpt-5.5',
      'gpt-5.4'
    ])

    // Production group 6 has default_mapped_model = gpt-5.5; the legacy default
    // must survive either way.
    for (const defaultMappedModel of ['gpt-5.5', '']) {
      const wrapper = mountModal({ defaultMappedModel })
      await flushPromises()

      const catalog = catalogBlock(wrapper)
      expect(catalog.match(/"slug":/g)).toHaveLength(5)
      expect(catalog).not.toContain('gpt-5.6-luna')
      expect(catalog).not.toContain('gpt-5.4-mini')
      expect(catalog).not.toContain('gpt-5.3-codex-spark')
      expect(catalog).not.toContain('codex-auto-review')
      expect(configBlock(wrapper)).toContain('model = "gpt-5.5"')
      wrapper.unmount()
    }
  })

  it('uses the group default model when it is listed', async () => {
    getGatewayModelsMock.mockResolvedValue(['gpt-5.4', 'gpt-5.4-mini'])

    const wrapper = mountModal({ defaultMappedModel: 'gpt-5.4-mini' })
    await flushPromises()

    expect(configBlock(wrapper)).toContain('model = "gpt-5.4-mini"')
    expect(configBlock(wrapper)).toContain('review_model = "gpt-5.4-mini"')
  })

  it('falls back to gpt-5.5 when the group default is not listed', async () => {
    getGatewayModelsMock.mockResolvedValue(['gpt-5.6-sol', 'gpt-5.5', 'gpt-5.4'])

    const wrapper = mountModal({ defaultMappedModel: 'gpt-5.6-luna' })
    await flushPromises()

    expect(configBlock(wrapper)).toContain('model = "gpt-5.5"')
  })

  it('uses the first listed model when neither default applies', async () => {
    getGatewayModelsMock.mockResolvedValue(['gpt-5.6-sol', 'gpt-5.6-terra'])

    const wrapper = mountModal()
    await flushPromises()

    expect(configBlock(wrapper)).toContain('model = "gpt-5.6-sol"')
  })

  it('falls back to the static catalog when group model discovery fails', async () => {
    getGatewayModelsMock.mockRejectedValue(new Error('network down'))

    const wrapper = mountModal()
    await flushPromises()

    const catalog = catalogBlock(wrapper)
    expect(catalog.match(/"slug":/g)).toHaveLength(5)
    expect(catalog).toContain('"slug": "gpt-5.6-sol"')
    expect(catalog).not.toContain('gpt-5.6-luna')
    expect(configBlock(wrapper)).toContain('model = "gpt-5.5"')
  })
})
