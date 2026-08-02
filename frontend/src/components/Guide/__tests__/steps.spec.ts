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
      '[data-tour="key-form-submit"]',
      '[data-tour="keys-created-setup-options"]'
    ])
    expect(steps.at(-1)?.optional).toBe(true)
  })

  it('does not route the main onboarding through docs or a separate CC Switch step', () => {
    const serialized = JSON.stringify(getUserSteps((key) => key))

    expect(serialized).not.toContain('sidebar-docs')
    expect(serialized).not.toContain('keys-save-official-provider')
    expect(serialized).not.toContain('sidebar-resources')
  })
})
