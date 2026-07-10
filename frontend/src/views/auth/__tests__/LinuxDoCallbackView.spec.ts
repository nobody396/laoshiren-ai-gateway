import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LinuxDoCallbackView from '../LinuxDoCallbackView.vue'

const mocks = vi.hoisted(() => ({
  route: {
    path: '/auth/google/callback',
    query: {}
  },
  routerReplace: vi.fn(),
  setToken: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  completeOAuthRegistration: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({
    replace: mocks.routerReplace
  })
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      locale: { value: 'en' },
      setLocaleMessage: vi.fn()
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/components/layout', () => ({
  AuthLayout: { template: '<div><slot /></div>' }
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { template: '<span />' }
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    setToken: mocks.setToken
  }),
  useAppStore: () => ({
    showSuccess: mocks.showSuccess,
    showError: mocks.showError
  })
}))

vi.mock('@/api/auth', () => ({
  completeOAuthRegistration: (...args: unknown[]) => mocks.completeOAuthRegistration(...args)
}))

const mockedCompleteOAuthRegistration = mocks.completeOAuthRegistration

function mountCallbackView(hash: string) {
  window.location.hash = hash
  return mount(LinuxDoCallbackView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /></div>' },
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' }
      }
    }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.route.path = '/auth/google/callback'
  mocks.route.query = {}
  mocks.routerReplace.mockResolvedValue(undefined)
  mocks.setToken.mockResolvedValue(undefined)
  mockedCompleteOAuthRegistration.mockResolvedValue({
    access_token: 'access-token',
    refresh_token: 'refresh-token',
    expires_in: 3600,
    token_type: 'Bearer'
  })
  localStorage.clear()
  window.location.hash = ''
})

describe('LinuxDoCallbackView OAuth completion', () => {
  it('hands the entire rotating token tuple to AuthSession through the store', async () => {
    mountCallbackView('#access_token=access&refresh_token=refresh&expires_in=900&provider=google&redirect=/dashboard')
    await flushPromises()

    expect(mocks.setToken).toHaveBeenCalledWith('access', 'refresh', 900)
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('token_expires_at')).toBeNull()
    expect(mocks.routerReplace).toHaveBeenCalledWith('/dashboard')
  })

  it('shows optional referral input and submits referral_code', async () => {
    const wrapper = mountCallbackView(
      '#error=referral_optional&pending_oauth_token=pending-token&provider=google&redirect=/dashboard'
    )
    await flushPromises()

    const referralInput = wrapper.find('input[placeholder="auth.referralCodePlaceholder"]')
    expect(referralInput.exists()).toBe(true)
    await referralInput.setValue('REF456')
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mockedCompleteOAuthRegistration).toHaveBeenCalledWith(
      'google',
      'pending-token',
      '',
      'REF456'
    )
    expect(mocks.setToken).toHaveBeenCalledWith('access-token', 'refresh-token', 3600)
    expect(mocks.routerReplace).toHaveBeenCalledWith('/dashboard')
  })

  it('allows skipping referral_code', async () => {
    const wrapper = mountCallbackView(
      '#error=referral_optional&pending_oauth_token=pending-token&provider=google&redirect=/dashboard'
    )
    await flushPromises()

    const buttons = wrapper.findAll('button')
    await buttons[1].trigger('click')
    await flushPromises()

    expect(mockedCompleteOAuthRegistration).toHaveBeenCalledWith(
      'google',
      'pending-token',
      '',
      ''
    )
    expect(mocks.routerReplace).toHaveBeenCalledWith('/dashboard')
  })

  it('supports invitation_required with optional referral_code', async () => {
    mocks.route.path = '/auth/github/callback'
    const wrapper = mountCallbackView(
      '#error=invitation_required&pending_oauth_token=pending-token&provider=github&redirect=/dashboard&referral_optional=true'
    )
    await flushPromises()

    await wrapper.find('input[placeholder="auth.invitationCodePlaceholder"]').setValue('INV123')
    await wrapper.find('input[placeholder="auth.referralCodePlaceholder"]').setValue('REF456')
    await wrapper.find('button.btn-primary').trigger('click')
    await flushPromises()

    expect(mockedCompleteOAuthRegistration).toHaveBeenCalledWith(
      'github',
      'pending-token',
      'INV123',
      'REF456'
    )
  })
})
