import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const indexPath = resolve(process.cwd(), 'index.html')
const indexHTML = readFileSync(indexPath, 'utf8')
const bootstrapScript = indexHTML.match(
  /<script nonce="__CSP_NONCE_VALUE__">([\s\S]*?)<\/script>/
)?.[1]

describe('app loading fallback', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    document.documentElement.className = ''
    document.body.innerHTML = `
      <div id="app-load-status">
        <h1 id="app-load-title"></h1>
        <p id="app-load-message"></p>
        <button id="app-load-retry" type="button"></button>
      </div>
    `
    window.sessionStorage.clear()
    delete window.__APP_LOAD_STATE__
  })

  afterEach(() => {
    vi.useRealTimers()
    document.documentElement.className = ''
    document.body.innerHTML = ''
    delete window.__APP_LOAD_STATE__
  })

  it('keeps SEO content hidden while JavaScript waits for the app', () => {
    expect(indexHTML).toContain('html.js .seo-static-content')
    expect(indexHTML).not.toContain('show-seo-fallback')
    expect(indexHTML).not.toContain('fonts.googleapis.com')
    expect(indexHTML).not.toContain('fonts.gstatic.com')
  })

  it('shows a retry state only after the slow-load threshold', () => {
    expect(bootstrapScript).toBeTruthy()
    window.eval(bootstrapScript!)

    expect(document.documentElement.classList.contains('js')).toBe(true)
    expect(document.documentElement.classList.contains('app-load-slow')).toBe(false)

    vi.advanceTimersByTime(14_999)
    expect(document.documentElement.classList.contains('app-load-slow')).toBe(false)

    vi.advanceTimersByTime(1)
    expect(document.documentElement.classList.contains('app-load-slow')).toBe(true)
    expect(document.getElementById('app-load-title')?.textContent).toBe('Still thinking…')
    expect(document.getElementById('app-load-message')?.textContent).toContain('网络有点慢')
  })

  it('clears fallback state after the Vue app mounts', () => {
    expect(bootstrapScript).toBeTruthy()
    window.eval(bootstrapScript!)

    window.__APP_LOAD_STATE__?.markSlow()
    window.__APP_LOAD_STATE__?.succeed()

    expect(document.documentElement.classList.contains('app-mounted')).toBe(true)
    expect(document.documentElement.classList.contains('app-load-slow')).toBe(false)
    expect(document.documentElement.classList.contains('app-load-failed')).toBe(false)
  })
})
