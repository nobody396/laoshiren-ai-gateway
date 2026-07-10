/**
 * Credential-free URL helpers for externally embedded pages.
 *
 * General bearer credentials and user identity must never cross an iframe or
 * new-tab boundary in a URL. Authenticated consumers use the separate,
 * audience-bound embed-ticket contract in embedded-session.ts.
 */

const FORBIDDEN_QUERY_KEYS = new Set([
  'token',
  'access_token',
  'refresh_token',
  'api_key',
  'apikey',
  'user_id',
  'src_url',
])

function isLocalDevelopmentUrl(url: URL): boolean {
  if (!import.meta.env.DEV) return false
  return url.hostname === 'localhost' || url.hostname === '127.0.0.1' || url.hostname === '[::1]'
}

export function isAllowedEmbeddedUrl(baseUrl: string): boolean {
  try {
    const url = new URL(baseUrl)
    if (url.username || url.password) return false
    if (url.protocol === 'https:') return true
    return url.protocol === 'http:' && isLocalDevelopmentUrl(url)
  } catch {
    return false
  }
}

export function buildEmbeddedUrl(
  baseUrl: string,
  theme: 'light' | 'dark' = 'light',
  lang?: string,
): string {
  const trimmed = baseUrl.trim()
  if (!trimmed || !isAllowedEmbeddedUrl(trimmed)) return ''

  const url = new URL(trimmed)
  for (const key of Array.from(url.searchParams.keys())) {
    if (FORBIDDEN_QUERY_KEYS.has(key.toLowerCase())) {
      url.searchParams.delete(key)
    }
  }
  url.searchParams.set('theme', theme)
  if (lang) url.searchParams.set('lang', lang)
  else url.searchParams.delete('lang')
  url.searchParams.set('ui_mode', 'embedded')
  return url.toString()
}

export function detectTheme(): 'light' | 'dark' {
  if (typeof document === 'undefined') return 'light'
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}
