import { describe, expect, it } from 'vitest'

import { formatFlipNumber, isFlipDigit } from '../flipNumber'

describe('formatFlipNumber', () => {
  it('pads the integer part and keeps two decimals by default', () => {
    expect(formatFlipNumber(19216)).toBe('19216.00')
    expect(formatFlipNumber(42.5, { pad: 5 })).toBe('00042.50')
  })

  it('rounds to the requested decimals instead of truncating', () => {
    expect(formatFlipNumber(17838.006, { pad: 4 })).toBe('17838.01') // 4 位 pad 不截断更长的整数部分
    expect(formatFlipNumber(1.999, { pad: 3, decimals: 1 })).toBe('002.0')
  })

  it('never emits grouping separators (they would break digit columns)', () => {
    expect(formatFlipNumber(219638.6)).toBe('219638.60')
    expect(formatFlipNumber(1234567.891, { pad: 2 })).toBe('1234567.89')
  })

  it('clamps negative and non-finite values to zero', () => {
    expect(formatFlipNumber(-12, { pad: 3 })).toBe('000.00')
    expect(formatFlipNumber(Number.NaN, { pad: 3 })).toBe('000.00')
    expect(formatFlipNumber(Number.POSITIVE_INFINITY, { pad: 3 })).toBe('000.00')
  })

  it('respects decimals = 0', () => {
    expect(formatFlipNumber(7.6, { pad: 2, decimals: 0 })).toBe('08')
  })
})

describe('isFlipDigit', () => {
  it('detects digits', () => {
    expect(isFlipDigit('0')).toBe(true)
    expect(isFlipDigit('9')).toBe(true)
  })

  it('rejects separators and other glyphs', () => {
    expect(isFlipDigit('.')).toBe(false)
    expect(isFlipDigit(',')).toBe(false)
    expect(isFlipDigit('¥')).toBe(false)
    expect(isFlipDigit('')).toBe(false)
  })
})
