import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const indexHTML = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8')
const bootstrapScript = indexHTML.match(
  /<script id="ga4-bootstrap" nonce="__CSP_NONCE_VALUE__">([\s\S]*?)<\/script>/
)?.[1]

describe('GA4 document bootstrap', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/docs/codex-no-api-key-guide?source=test')
    window.dataLayer = []
    delete window.gtag
    delete window.__GA4_BOOTSTRAP__
  })

  afterEach(() => {
    vi.restoreAllMocks()
    window.dataLayer = []
    delete window.gtag
    delete window.__GA4_BOOTSTRAP__
  })

  it('queues config and one initial page_view before the SPA mounts', () => {
    expect(bootstrapScript).toBeTruthy()
    window.eval(bootstrapScript!)

    expect(window.__GA4_BOOTSTRAP__).toEqual({
      measurementId: 'G-KY7X4XJS6B',
      initialPagePath: '/docs/codex-no-api-key-guide',
      initialPageViewConsumed: false,
    })
    expect(window.dataLayer).toHaveLength(3)
    expect(Array.from(window.dataLayer?.[1] as IArguments)).toEqual([
      'config',
      'G-KY7X4XJS6B',
      { send_page_view: false },
    ])
    expect(Array.from(window.dataLayer?.[2] as IArguments)).toEqual([
      'event',
      'page_view',
      expect.objectContaining({ page_path: '/docs/codex-no-api-key-guide' }),
    ])
  })

  it('does not make the initial document wait for the external GA loader', () => {
    expect(indexHTML).not.toContain('id="ga4-gtag-js"')
    expect(indexHTML).not.toContain('src="https://www.googletagmanager.com/gtag/js')
  })
})
