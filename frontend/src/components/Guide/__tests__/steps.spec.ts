import { describe, expect, it } from 'vitest'

import { getUserSteps } from '@/components/Guide/steps'

describe('user onboarding flow', () => {
  it('keeps the education steps and ends on the newly created key setup choices', () => {
    const steps = getUserSteps((key) => key)
    const selectors = steps
      .map((step) => step.element)
      .filter((element): element is string => typeof element === 'string')

    expect(selectors).toEqual([
      '[data-tour="sidebar-topup"]',
      '[data-tour="topup-panel"]',
      '[data-tour="sidebar-model-pricing"]',
      '[data-tour="sidebar-my-keys"]',
      '[data-tour="keys-create-btn"]',
      '[data-tour="key-form-name"]',
      '[data-tour="key-form-group"]',
      '[data-tour="key-form-submit"]'
    ])
    expect(typeof steps.at(-1)?.element).toBe('function')
    expect(steps.at(-1)?.optional).toBe(true)
  })

  it('highlights the visible setup actions instead of DataTable hidden duplicate', () => {
    document.body.innerHTML = `
      <div data-tour="keys-setup-options" id="hidden"></div>
      <div data-tour="keys-setup-options" id="visible"></div>
    `
    const hidden = document.querySelector<HTMLElement>('#hidden')!
    const visible = document.querySelector<HTMLElement>('#visible')!
    hidden.getBoundingClientRect = () => ({ width: 0, height: 0 }) as DOMRect
    visible.getBoundingClientRect = () => ({ width: 160, height: 46 }) as DOMRect

    const element = getUserSteps((key) => key).at(-1)?.element
    expect(typeof element).toBe('function')
    expect((element as () => Element)()).toBe(visible)
  })

  it('does not route the main onboarding through docs or a separate CC Switch step', () => {
    const serialized = JSON.stringify(getUserSteps((key) => key))

    expect(serialized).not.toContain('sidebar-docs')
    expect(serialized).not.toContain('keys-save-official-provider')
    expect(serialized).not.toContain('sidebar-resources')
  })
})
