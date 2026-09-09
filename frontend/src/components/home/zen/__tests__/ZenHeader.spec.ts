import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'

import ZenHeader from '../ZenHeader.vue'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

describe('ZenHeader', () => {
  it('keeps the public documentation entry visible beside the primary CTA', () => {
    const wrapper = mount(ZenHeader, {
      props: { isAuthenticated: false, dashboardPath: '/dashboard' },
      global: {
        plugins: [createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': {} } })],
        stubs: {
          RouterLink: RouterLinkStub,
          LocaleSwitcher: true,
        },
      },
    })

    const docs = wrapper.get('.zen-header__docs')
    expect(docs.text()).toBe('文档')
    expect(docs.attributes('href')).toBe('/docs')
  })
})
