import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import RestoreOfficialConfigModal from '../RestoreOfficialConfigModal.vue'
import { buildOfficialRestoreCommand } from '@/utils/officialConfigRestore'
const copy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
const create = () => mount(RestoreOfficialConfigModal, {
  props: { show: true }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } }
})
beforeEach(() => copy.mockReset().mockResolvedValue(true))
it('offers both tool icons and copies all six client/OS commands without a key or ticket', async () => {
  const wrapper = create()
  expect(wrapper.findAll('[data-client]')).toHaveLength(2)
  for (const client of ['codex', 'claude'] as const) {
    expect(wrapper.get(`[data-client-icon="${client}"] img`).attributes('src')).toContain('/brand/client-tools/')
    for (const os of ['macOS', 'Linux', 'Windows']) {
      await wrapper.findAll('button').find(b => b.text() === os)!.trigger('click')
      await wrapper.get(`[data-client="${client}"]`).trigger('click')
      await flushPromises()
      const command = buildOfficialRestoreCommand(client, os === 'Windows')
      expect(copy).toHaveBeenLastCalledWith(command)
      expect(wrapper.text()).toContain('已复制')
      expect(wrapper.get('textarea').element.value).toBe(command)
      expect(command).not.toMatch(/SETUP_TOKEN|\n/)
      expect(command).toContain(`restore-${client}.cjs`)
    }
  }
  expect(wrapper.text()).toContain('/login')
  expect(wrapper.text()).toContain('暂未支持还原')
})
it('shows manual fallback and resets it on OS changes and close', async () => {
  copy.mockResolvedValue(false)
  const wrapper = create()
  await wrapper.get('[data-client="claude"]').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('自动复制失败')
  expect(wrapper.get('textarea').attributes('aria-label')).toBe('Claude Code 还原命令')
  await wrapper.findAll('button')[1].trigger('click')
  expect(wrapper.find('textarea').exists()).toBe(false)
  await wrapper.setProps({ show: false })
  expect(wrapper.find('textarea').exists()).toBe(false)
})
it('ignores stale clipboard results after changing tool or reopening the modal', async () => {
  let resolveOld!: (value: boolean) => void
  copy.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
  const wrapper = create()
  await wrapper.get('[data-client="codex"]').trigger('click')
  await wrapper.get('[data-client="claude"]').trigger('click')
  await flushPromises()
  resolveOld(false)
  await flushPromises()
  expect(wrapper.get('[role="status"]').text()).toBe('Claude Code · 已复制，请在终端执行')
  copy.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
  await wrapper.get('[data-client="codex"]').trigger('click')
  await wrapper.setProps({ show: false })
  await wrapper.setProps({ show: true })
  resolveOld(true)
  await flushPromises()
  expect(wrapper.find('textarea').exists()).toBe(false)
})
