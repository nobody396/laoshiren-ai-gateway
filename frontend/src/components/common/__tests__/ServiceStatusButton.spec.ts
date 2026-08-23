import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ServiceStatusButton from '../ServiceStatusButton.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))

describe('ServiceStatusButton', () => {
  it('opens the in-app public status page instead of an external placeholder', () => {
    const wrapper = mount(ServiceStatusButton)
    expect(wrapper.get('a').attributes('href')).toBe('/status')
    expect(wrapper.html()).not.toContain('status.your-domain.example')
  })
})
