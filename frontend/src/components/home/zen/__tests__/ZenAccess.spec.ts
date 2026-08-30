import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ZenAccess from '../ZenAccess.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: { value: 'zh-CN' } })
}))

describe('ZenAccess', () => {
  it('shows the ten named workflow clients without generic explanatory copy', () => {
    const wrapper = mount(ZenAccess)
    const cells = wrapper.findAll('.zen-access__cell')

    expect(cells).toHaveLength(10)
    expect(cells.map(cell => cell.text())).toEqual([
      'Claude Code',
      'Codex',
      'Kimi Code',
      'Z Code',
      'Gemini CLI',
      'AntiGravity',
      'Grok Build',
      'Hermes Agent',
      'opencode',
      'OpenClaw'
    ])
    expect(wrapper.find('.zen-access__title p').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('任意 OpenAI 兼容客户端')
  })
})
