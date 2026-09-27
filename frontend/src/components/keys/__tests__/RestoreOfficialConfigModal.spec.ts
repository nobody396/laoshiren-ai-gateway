import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import RestoreOfficialConfigModal from '../RestoreOfficialConfigModal.vue'
import { buildCodexRestoreCommand } from '@/utils/codexRestore'
const copy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
const create = () => mount(RestoreOfficialConfigModal, {
  props: { show: true }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } }
})
beforeEach(() => copy.mockReset().mockResolvedValue(true))
it('only offers Codex, copies OS-specific commands with no key or ticket', async () => {
  const wrapper = create()
  expect(wrapper.findAll('[data-client]')).toHaveLength(1)
  for (const os of ['macOS', 'Linux', 'Windows']) {
    await wrapper.findAll('button').find(b => b.text() === os)!.trigger('click')
    await wrapper.get('[data-client="codex"]').trigger('click')
    await flushPromises()
    expect(copy).toHaveBeenLastCalledWith(buildCodexRestoreCommand(os === 'Windows'))
    expect(wrapper.text()).toContain('已复制')
    expect(buildCodexRestoreCommand(os === 'Windows')).not.toMatch(/SETUP_TOKEN|\n/)
  }
})
it('shows a manual copy fallback and resets it when changing OS or closing', async () => {
  copy.mockResolvedValue(false)
  const wrapper = create()
  await wrapper.get('[data-client="codex"]').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('自动复制失败')
  expect(wrapper.find('textarea').exists()).toBe(true)
  await wrapper.findAll('button')[1].trigger('click')
  expect(wrapper.find('textarea').exists()).toBe(false)
  await wrapper.setProps({ show: false })
  expect(wrapper.find('textarea').exists()).toBe(false)
})
