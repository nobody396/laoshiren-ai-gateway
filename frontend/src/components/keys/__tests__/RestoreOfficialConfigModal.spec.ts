import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import RestoreOfficialConfigModal from '../RestoreOfficialConfigModal.vue'
import { buildOfficialRestoreCommand } from '@/utils/officialConfigRestore'
import { useAppStore } from '@/stores/app'

// Vitest uses the runtime-only i18n build; translation is outside this copy-flow test.
vi.mock('@/i18n', () => ({ i18n: { global: { t: (key: string) => key === 'common.copyFailed' ? '复制失败' : key } } }))
const write = vi.fn()
const fallback = vi.fn()
const create = () => mount(RestoreOfficialConfigModal, {
  props: { show: true }, global: { plugins: [createPinia()], stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } } }
})
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  vi.stubGlobal('isSecureContext', true)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: write } })
  Object.defineProperty(document, 'execCommand', { configurable: true, value: fallback })
  write.mockReset().mockResolvedValue(undefined)
  fallback.mockReset().mockReturnValue(false)
})
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
it('keeps only OS choices, tool icons and one short instruction; never renders a command', async () => {
  const wrapper = create()
  expect(wrapper.findAll('[data-client]')).toHaveLength(2)
  expect(wrapper.find('textarea, input, pre, code, ul').exists()).toBe(false)
  expect(wrapper.text().length).toBeLessThan(80)
  for (const client of ['codex', 'claude'] as const) {
    expect(wrapper.get(`[data-client-icon="${client}"] img`).attributes('src')).toContain('/brand/client-tools/')
    for (const os of ['macOS', 'Linux', 'Windows']) {
      await wrapper.findAll('button').find(b => b.text() === os)!.trigger('click')
      await wrapper.get(`[data-client="${client}"]`).trigger('click')
      await flushPromises()
      const command = buildOfficialRestoreCommand(client, os === 'Windows')
      expect(write).toHaveBeenLastCalledWith(command)
      expect(useAppStore().toasts.at(-1)).toMatchObject({ type: 'success', message: '已复制到剪贴板，请到终端执行' })
      expect(wrapper.text()).not.toMatch(/https:|provider|MCP|Hooks|TOML|\/login|暂未支持|已复制/)
      expect(wrapper.find('textarea, input, pre, code').exists()).toBe(false)
    }
  }
})
it('failed copy shows only the shared failure toast, keeps the command hidden and permits retry', async () => {
  write.mockRejectedValueOnce(new Error('permission denied'))
  const wrapper = create()
  await wrapper.get('[data-client="claude"]').trigger('click')
  await flushPromises()
  expect(useAppStore().toasts).toHaveLength(1)
  expect(useAppStore().toasts[0]).toMatchObject({ type: 'error', message: '复制失败' })
  expect(wrapper.find('textarea, input, pre, code').exists()).toBe(false)
  expect(document.querySelector('textarea')).toBeNull()
  expect(wrapper.get('[data-client="claude"]').attributes('disabled')).toBeUndefined()
  await wrapper.get('[data-client="claude"]').trigger('click')
  await flushPromises()
  expect(useAppStore().toasts.at(-1)?.type).toBe('success')
})
it('locks controls during copying and then re-enables them without extra page state', async () => {
  let resolve!: () => void
  write.mockImplementationOnce(() => new Promise<void>(done => { resolve = done }))
  const wrapper = create()
  await wrapper.get('[data-client="codex"]').trigger('click')
  expect(wrapper.findAll('button').every(b => b.attributes('disabled') !== undefined)).toBe(true)
  resolve()
  await flushPromises()
  expect(wrapper.findAll('button').every(b => b.attributes('disabled') === undefined)).toBe(true)
  expect(wrapper.find('[role="status"]').exists()).toBe(false)
})
