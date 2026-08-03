import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CustomerServiceButton from '../CustomerServiceButton.vue'
import { useAppStore } from '@/stores/app'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'common.customerService': '客服',
    'common.wechatId': '微信号',
    'common.scanToAddSupport': '微信扫码添加客服',
    'common.close': '关闭',
    'common.afterSalesTitle': '售后客服',
    'common.afterSalesDesc': '处理账号和订单问题',
    'common.techSupportTitle': '技术客服',
    'common.techSupportDesc': '处理配置和使用问题'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

function mountButton() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return {
    appStore: useAppStore(),
    wrapper: mount(CustomerServiceButton, {
      global: {
        plugins: [pinia],
        stubs: {
          Icon: true,
          Teleport: true,
          Transition: false
        }
      }
    })
  }
}

describe('CustomerServiceButton', () => {
  beforeEach(() => {
    document.body.style.overflow = ''
  })

  it('keeps the support entry usable when public settings are empty', async () => {
    const { wrapper } = mountButton()

    expect(wrapper.get('[data-testid="customer-service-entry"]').text()).toContain('客服')

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('微信扫码添加客服')
    expect(wrapper.get('[data-testid="default-support-qr"]').attributes('src')).toContain(
      'wechat-jac-hh.png'
    )
    expect(wrapper.text()).toContain('Jac_Hh')
  })

  it('prefers the configured support contact over the fallback', async () => {
    const { appStore, wrapper } = mountButton()
    appStore.contactInfo = 'ConfiguredSupport'

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('ConfiguredSupport')
    expect(wrapper.text()).not.toContain('Jac_Hh')
  })

  it('does not add the fallback as a second category when only one QR code is configured', async () => {
    const { appStore, wrapper } = mountButton()
    appStore.techSupportQRCode = 'https://example.com/support.png'

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('技术客服')
    expect(wrapper.text()).not.toContain('售后客服')
    expect(wrapper.text()).not.toContain('Jac_Hh')
  })
})
