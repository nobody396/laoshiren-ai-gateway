import { afterEach, describe, expect, it } from 'vitest'
import { buildEmbeddedUrl, detectTheme, isAllowedEmbeddedUrl } from '../embedded-url'

describe('embedded-url', () => {
  afterEach(() => document.documentElement.classList.remove('dark'))

  it('keeps only display context and strips bearer or identity parameters', () => {
    const result = buildEmbeddedUrl(
      'https://pay.example.com/checkout?plan=pro&token=jwt&user_id=42&refresh_token=rt&api_key=key&src_url=https://private.example/path',
      'dark',
      'zh-CN',
    )
    const url = new URL(result)
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.get('theme')).toBe('dark')
    expect(url.searchParams.get('lang')).toBe('zh-CN')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    for (const key of ['token', 'user_id', 'refresh_token', 'api_key', 'src_url']) {
      expect(url.searchParams.has(key)).toBe(false)
    }
  })

  it('rejects invalid, credentialed and insecure non-local URLs', () => {
    expect(buildEmbeddedUrl('not a url')).toBe('')
    expect(buildEmbeddedUrl('https://user:pass@example.com/path')).toBe('')
    expect(buildEmbeddedUrl('http://pay.example.com/path')).toBe('')
    expect(isAllowedEmbeddedUrl('https://pay.example.com/path')).toBe(true)
  })

  it('keeps localhost HTTP available in development tests', () => {
    expect(buildEmbeddedUrl('http://localhost:3000/embed', 'light')).toContain('http://localhost:3000/embed')
  })

  it('detects dark mode from document root class', () => {
    document.documentElement.classList.add('dark')
    expect(detectTheme()).toBe('dark')
  })
})
