import { describe, expect, it } from 'vitest'

import { isValidPositiveMultiplier } from '../types'

describe('channel service-tier multiplier validation', () => {
  it('accepts empty and positive multipliers', () => {
    expect(isValidPositiveMultiplier(null)).toBe(true)
    expect(isValidPositiveMultiplier('')).toBe(true)
    expect(isValidPositiveMultiplier(0.5)).toBe(true)
    expect(isValidPositiveMultiplier('2.5')).toBe(true)
  })

  it('rejects zero, negative and non-finite multipliers', () => {
    expect(isValidPositiveMultiplier(0)).toBe(false)
    expect(isValidPositiveMultiplier(-1)).toBe(false)
    expect(isValidPositiveMultiplier('not-a-number')).toBe(false)
    expect(isValidPositiveMultiplier(Number.POSITIVE_INFINITY)).toBe(false)
  })
})
