const DEFAULT_GA4_MEASUREMENT_ID = 'G-KY7X4XJS6B'

type AnalyticsParams = Record<string, string | number | boolean | undefined | null>

declare global {
  interface Window {
    dataLayer?: unknown[]
    gtag?: (...args: unknown[]) => void
    __GA4_BOOTSTRAP__?: {
      measurementId: string
      initialPagePath: string
      initialPageViewConsumed: boolean
    }
  }
}

let initialized = false

function measurementId(): string {
  return (import.meta.env.VITE_GA4_MEASUREMENT_ID || DEFAULT_GA4_MEASUREMENT_ID).trim()
}

function shouldEnableAnalytics(): boolean {
  return Boolean(
    typeof window !== 'undefined' &&
      typeof document !== 'undefined' &&
      measurementId() &&
      import.meta.env.VITE_GA4_DISABLED !== 'true' &&
      (import.meta.env.PROD || import.meta.env.VITE_GA4_FORCE_ENABLE === 'true')
  )
}

function cleanPath(path: string): string {
  try {
    const parsed = new URL(path, window.location.origin)
    return parsed.pathname || '/'
  } catch {
    const normalized = path.startsWith('/') ? path : `/${path}`
    return normalized.split(/[?#]/)[0] || '/'
  }
}

function cleanParams(params: AnalyticsParams = {}): AnalyticsParams {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => value !== undefined && value !== null)
  ) as AnalyticsParams
}

export function initAnalytics(): void {
  if (initialized || !shouldEnableAnalytics()) return

  const id = measurementId()
  const existingScript = document.getElementById('ga4-gtag-js')
  if (window.__GA4_BOOTSTRAP__?.measurementId === id && window.gtag && existingScript) {
    initialized = true
    return
  }

  window.dataLayer = window.dataLayer || []
  window.gtag =
    window.gtag ||
    function gtag(...args: unknown[]) {
      window.dataLayer?.push(args)
    }

  if (!existingScript) {
    const script = document.createElement('script')
    script.id = 'ga4-gtag-js'
    script.async = true
    script.src = `https://www.googletagmanager.com/gtag/js?id=${encodeURIComponent(id)}`
    document.head.appendChild(script)
  }

  window.gtag('js', new Date())
  window.gtag('config', id, {
    send_page_view: false,
  })
  initialized = true
}

// Loading googletagmanager from the initial HTML keeps the browser's native
// progress indicator running until a blocked/slow third-party request ends.
// Queue page views immediately, but fetch the optional analytics runtime only
// after the first-party document has fully loaded.
export function scheduleAnalyticsInit(): void {
  if (!shouldEnableAnalytics()) return

  const start = () => {
    const idleWindow = window as Window & {
      requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number
    }
    if (idleWindow.requestIdleCallback) {
      idleWindow.requestIdleCallback(() => initAnalytics(), { timeout: 2000 })
    } else {
      window.setTimeout(() => initAnalytics(), 0)
    }
  }

  if (document.readyState === 'complete') {
    start()
  } else {
    window.addEventListener('load', start, { once: true })
  }
}

export function trackPageView(path: string, params: AnalyticsParams = {}): void {
  if (!shouldEnableAnalytics()) return

  const pagePath = cleanPath(path)
  const bootstrap = window.__GA4_BOOTSTRAP__
  if (bootstrap && !bootstrap.initialPageViewConsumed && bootstrap.measurementId === measurementId()) {
    bootstrap.initialPageViewConsumed = true
    if (bootstrap.initialPagePath === pagePath) return
  }

  if (!window.gtag) initAnalytics()

  window.gtag?.('event', 'page_view', {
    page_path: pagePath,
    page_location: `${window.location.origin}${pagePath}`,
    page_title: document.title,
    ...cleanParams(params),
  })
}

export function trackEvent(eventName: string, params: AnalyticsParams = {}): void {
  if (!shouldEnableAnalytics()) return
  if (!window.gtag) initAnalytics()
  window.gtag?.('event', eventName, cleanParams(params))
}
