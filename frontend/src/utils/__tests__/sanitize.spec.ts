import { describe, expect, it } from 'vitest'
import { sanitizeSvg } from '../sanitize'

describe('sanitizeSvg', () => {
  it.each([
    '<svg><g onload="alert(1)"></g></svg>',
    '<svg><a href="javascript:alert(1)">x</a></svg>',
    '<svg><use href="data:text/html,<script>alert(1)</script>"></use></svg>'
  ])('removes active content from %s', (payload) => {
    const sanitized = sanitizeSvg(payload).toLowerCase()
    expect(sanitized).not.toContain('onload=')
    expect(sanitized).not.toContain('javascript:')
    expect(sanitized).not.toContain('<script')
  })
})
