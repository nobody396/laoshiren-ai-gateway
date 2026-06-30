const DEFAULT_GA4_MEASUREMENT_ID = 'G-KY7X4XJS6B'

type AnalyticsParams = Record<string, string | number | boolean | undefined | null>

declare global {
  interface Window {
    dataLayer?: unknown[]
    gtag?: (...args: unknown[]) => void
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
  window.dataLayer = window.dataLayer || []
  window.gtag =
    window.gtag ||
    function gtag(...args: unknown[]) {
      window.dataLayer?.push(args)
    }

  const existingScript = document.getElementById('ga4-gtag-js')
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

export function trackPageView(path: string, params: AnalyticsParams = {}): void {
  if (!shouldEnableAnalytics()) return
  initAnalytics()

  const pagePath = cleanPath(path)
  window.gtag?.('event', 'page_view', {
    page_path: pagePath,
    page_location: `${window.location.origin}${pagePath}`,
    page_title: document.title,
    ...cleanParams(params),
  })
}

export function trackEvent(eventName: string, params: AnalyticsParams = {}): void {
  if (!shouldEnableAnalytics()) return
  initAnalytics()
  window.gtag?.('event', eventName, cleanParams(params))
}
