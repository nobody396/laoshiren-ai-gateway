import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'

const {
  getAgentDashboard,
  getAgentInviteCode,
  getAgentPaymentProfile,
  getAgentPaymentQRCode,
  updateAgentPaymentProfile,
  uploadAgentPaymentQRCode
} = vi.hoisted(() => ({
  getAgentDashboard: vi.fn(),
  getAgentInviteCode: vi.fn(),
  getAgentPaymentProfile: vi.fn(),
  getAgentPaymentQRCode: vi.fn(),
  updateAgentPaymentProfile: vi.fn(),
  uploadAgentPaymentQRCode: vi.fn()
}))

const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('@/api/agent', () => ({
  getAgentDashboard,
  getAgentInviteCode,
  getAgentPaymentProfile,
  getAgentPaymentQRCode,
  updateAgentPaymentProfile,
  uploadAgentPaymentQRCode
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 2, role: 'agent' }
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: false,
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key}:${JSON.stringify(params)}`
      }
    })
  }
})

const createDashboard = () => ({
  invited_user_count: 0,
  total_commission: 0,
  settled_commission: 0,
  unsettled_commission: 0,
  period_commission: 0,
  this_month_commission: 0,
  consumption_rate: 0.05,
  first_recharge_invitee_rate: 0.1,
  rate_source: 'agent_level',
  current_level: 'light',
  current_level_name: '轻代理',
  permanent_level: 'light',
  permanent_level_name: '轻代理',
  assessment_period_start: '2026-05-01T00:00:00+08:00',
  assessment_period_end: '2026-05-31T23:59:59+08:00',
  next_assessment_at: '2026-06-01T00:00:00+08:00',
  settlement_minimum_amount: 50,
  settlement_eligible: false,
  settlement_gap: 50,
  payment_profile_complete: false
})

const createProfile = (overrides = {}) => ({
  agent_id: 2,
  alipay_real_name: '',
  alipay_account: '',
  contact_phone: '',
  payment_note: '',
  has_alipay_qr: false,
  complete: false,
  updated_at: '',
  ...overrides
})

describe('agent DashboardView payment profile', () => {
  const originalCreateObjectURL = URL.createObjectURL
  const originalRevokeObjectURL = URL.revokeObjectURL

  beforeEach(() => {
    getAgentDashboard.mockReset()
    getAgentInviteCode.mockReset()
    getAgentPaymentProfile.mockReset()
    getAgentPaymentQRCode.mockReset()
    updateAgentPaymentProfile.mockReset()
    uploadAgentPaymentQRCode.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    getAgentDashboard.mockResolvedValue(createDashboard())
    getAgentInviteCode.mockResolvedValue({ invite_code: 'ABC123' })
    getAgentPaymentProfile.mockResolvedValue(createProfile())
    getAgentPaymentQRCode.mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
    updateAgentPaymentProfile.mockImplementation(async (payload) => createProfile(payload))
    uploadAgentPaymentQRCode.mockImplementation(async () => createProfile({
      alipay_real_name: '傅俊豪',
      alipay_account: 'agent@example.com',
      contact_phone: '13800000000',
      has_alipay_qr: true,
      complete: true,
      updated_at: '2026-05-17T14:21:00+08:00'
    }))

    URL.createObjectURL = vi.fn(() => 'blob:agent-payment-qr')
    URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('saves typed payment fields before uploading a QR code and then shows a preview', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          RouterLink: { template: '<a><slot /></a>' }
        }
      }
    })

    await flushPromises()

    await wrapper.find('input[placeholder="agent.alipayRealNamePlaceholder"]').setValue('傅俊豪')
    await wrapper.find('input[placeholder="agent.alipayAccountPlaceholder"]').setValue('agent@example.com')
    await wrapper.find('input[placeholder="agent.optional"]').setValue('13800000000')

    const file = new File(['fake png data'], 'alipay.png', { type: 'image/png' })
    const fileInput = wrapper.find('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [file],
      configurable: true
    })

    await fileInput.trigger('change')
    await flushPromises()

    expect(updateAgentPaymentProfile).toHaveBeenCalledWith({
      alipay_real_name: '傅俊豪',
      alipay_account: 'agent@example.com',
      contact_phone: '13800000000',
      payment_note: ''
    })
    expect(uploadAgentPaymentQRCode).toHaveBeenCalledWith(file)
    expect(updateAgentPaymentProfile.mock.invocationCallOrder[0]).toBeLessThan(
      uploadAgentPaymentQRCode.mock.invocationCallOrder[0]
    )
    expect(getAgentPaymentQRCode).toHaveBeenCalled()
    expect(wrapper.find('img[alt="agent.alipayQRCode"]').exists()).toBe(true)
    expect(showSuccess).toHaveBeenCalledWith('agent.alipayQRCodeSaved')
  })
})
