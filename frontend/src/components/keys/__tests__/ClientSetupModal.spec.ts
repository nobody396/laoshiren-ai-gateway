import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ClientSetupModal from '../ClientSetupModal.vue'

const mocks = vi.hoisted(() => ({
  options: vi.fn(),
  ticket: vi.fn(),
  copy: vi.fn(),
}))

vi.mock('@/api', () => ({
  resourcesAPI: {
    getClientSetupOptions: mocks.options,
    createClientSetupTicketForOption: mocks.ticket,
  },
}))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: mocks.copy }),
}))

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<section v-if="show"><h1>{{ title }}</h1><slot /></section>',
}

describe('ClientSetupModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.options.mockResolvedValue([{ client_id: 'codex', name: 'Codex' }])
    mocks.ticket.mockResolvedValue({ ticket: 'one-time-ticket', expires_in: 600, target: 'codex' })
    mocks.copy.mockResolvedValue(true)
  })

  it('shows only server-approved tools and copies immediately after selection', async () => {
    const wrapper = mount(ClientSetupModal, {
      props: { show: true, apiKeyId: 42, keyName: 'my key', groupName: 'GPT 标准线路' },
      global: { stubs: { BaseDialog: BaseDialogStub, Icon: true } },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('GPT 标准线路')
    expect(wrapper.findAll('[data-client]')).toHaveLength(1)

    await wrapper.get('[role="tab"][aria-selected="false"]:last-of-type').trigger('click')
    await flushPromises()
    await wrapper.get('[data-client="codex"]').trigger('click')
    await flushPromises()

    expect(mocks.ticket).toHaveBeenCalledWith(42, 'codex', 'windows')
    const command = mocks.copy.mock.calls[0][0] as string
    expect(command).toContain("$env:LAOSHIRENAI_SETUP_TOKEN='one-time-ticket'")
    expect(command).toContain("$env:LAOSHIRENAI_INSTALL_CODEX_APP='1'")
    expect(command).not.toContain('LAOSHIRENAI_SKIP_CLIENT_INSTALL')
    expect(command).not.toContain('sk-')
  })
})
