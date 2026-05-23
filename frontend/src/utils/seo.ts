import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { i18n } from '@/i18n'
import { findDocItemBySlug, resolveDocSlug, docsLastModified } from '@/docs/config'
import { resolveDocumentTitle } from '@/router/title'

const DEFAULT_SITE_NAME = '老实人AI'
const DEFAULT_SITE_ORIGIN = 'https://laoshirenai.com'
const DEFAULT_SITE_LOGO = '/laoshirenai-icon.jpg'
const DEFAULT_OG_IMAGE = '/og-image.png'
const HOME_DESCRIPTION = '老实人AI 提供面向开发者的 AI 编码中转服务，支持 Claude Code、Codex、ChatGPT、Gemini 等主流编码模型，适合快速配置、精确计费和稳定调用。'
const DOCS_DESCRIPTION = '老实人AI 文档中心提供 Claude Code、Codex、OpenClaw、Hermes、Cherry Studio、GPT-Image 和企业接入的配置教程与常见问题。'

const INDEXABLE_ROUTE_NAMES = new Set(['Home', 'Docs', 'DocsPage', 'Enterprise', 'Security', 'Status'])

type SeoOptions = {
  siteName?: string
  siteLogo?: string
  customTitle?: string
}

type RouteSeo = {
  title: string
  description: string
  canonicalUrl: string
  robots: string
  ogType: 'website' | 'article'
  structuredData: Record<string, unknown> | null
}

function normalizeOrigin(value?: string): string {
  const raw = value?.trim() || DEFAULT_SITE_ORIGIN
  try {
    const url = new URL(raw)
    return url.origin
  } catch {
    return DEFAULT_SITE_ORIGIN
  }
}

function getSiteOrigin(): string {
  return normalizeOrigin(import.meta.env.VITE_SITE_URL)
}

function normalizePath(path: string): string {
  if (!path || path === '/') return '/'
  const withoutQuery = path.split('?')[0].split('#')[0]
  return withoutQuery.endsWith('/') ? withoutQuery.slice(0, -1) : withoutQuery
}

function absoluteUrl(pathOrUrl: string): string {
  try {
    return new URL(pathOrUrl, getSiteOrigin()).toString()
  } catch {
    return new URL('/', getSiteOrigin()).toString()
  }
}

function translate(key?: unknown): string {
  if (typeof key !== 'string' || !key.trim()) return ''
  const translated = i18n.global.t(key)
  return translated && translated !== key ? translated : ''
}

function routeName(route: RouteLocationNormalizedLoaded): string {
  return typeof route.name === 'string' ? route.name : ''
}

function isIndexableRoute(route: RouteLocationNormalizedLoaded): boolean {
  if (route.meta.noindex === true) return false
  return INDEXABLE_ROUTE_NAMES.has(routeName(route))
}

function resolveCanonicalPath(route: RouteLocationNormalizedLoaded): string {
  if (route.name === 'Home') return '/'
  if (route.name === 'Docs') return '/docs'
  if (route.name === 'DocsPage') {
    const slug = resolveDocSlug(String(route.params.slug || ''))
    return `/docs/${slug}`
  }
  return normalizePath(route.path)
}

function resolveDescription(route: RouteLocationNormalizedLoaded): string {
  if (route.name === 'Home') return HOME_DESCRIPTION
  if (route.name === 'Docs') return DOCS_DESCRIPTION
  if (route.name === 'DocsPage') {
    const doc = findDocItemBySlug(String(route.params.slug || ''))
    return doc?.description || DOCS_DESCRIPTION
  }

  const translated = translate(route.meta.descriptionKey)
  if (translated) return translated
  if (typeof route.meta.description === 'string' && route.meta.description.trim()) {
    return route.meta.description.trim()
  }
  return HOME_DESCRIPTION
}

function resolveTitle(route: RouteLocationNormalizedLoaded, siteName: string, customTitle?: string): string {
  if (customTitle?.trim()) return `${customTitle.trim()} - ${siteName}`
  if (route.name === 'Docs') return `文档 - ${siteName}`
  if (route.name === 'DocsPage') {
    const doc = findDocItemBySlug(String(route.params.slug || ''))
    if (doc?.title) return `${doc.title} - 文档 - ${siteName}`
  }
  return resolveDocumentTitle(route.meta.title, siteName, route.meta.titleKey as string, {
    siteNameFirst: route.meta.titleSiteNameFirst === true
  })
}

function setMetaByName(name: string, content: string): void {
  let element = document.head.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute('name', name)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

function setMetaByProperty(property: string, content: string): void {
  let element = document.head.querySelector<HTMLMetaElement>(`meta[property="${property}"]`)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute('property', property)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

function setCanonical(url: string): void {
  let element = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!element) {
    element = document.createElement('link')
    element.setAttribute('rel', 'canonical')
    document.head.appendChild(element)
  }
  element.setAttribute('href', url)
}

function setStructuredData(data: Record<string, unknown> | null): void {
  document.head
    .querySelectorAll<HTMLScriptElement>('script[type="application/ld+json"][data-seo]')
    .forEach((element) => element.remove())

  if (!data) return

  const element = document.createElement('script')
  element.type = 'application/ld+json'
  element.dataset.seo = 'structured-data'
  element.textContent = JSON.stringify(data)
  document.head.appendChild(element)
}

function organizationNode(siteName: string, siteLogo: string) {
  return {
    '@type': 'Organization',
    '@id': `${getSiteOrigin()}/#organization`,
    name: siteName,
    alternateName: ['老实人 AI', '老实人ai', 'Laoshiren AI', 'laoshirenai'],
    url: getSiteOrigin(),
    logo: absoluteUrl(siteLogo || DEFAULT_SITE_LOGO)
  }
}

function buildBreadcrumb(items: Array<{ name: string; url: string }>) {
  return {
    '@type': 'BreadcrumbList',
    itemListElement: items.map((item, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: item.name,
      item: item.url
    }))
  }
}

function buildStructuredData(route: RouteLocationNormalizedLoaded, seo: Omit<RouteSeo, 'structuredData'>, siteName: string, siteLogo: string) {
  if (!isIndexableRoute(route)) return null

  const org = organizationNode(siteName, siteLogo)
  if (route.name === 'Home') {
    return {
      '@context': 'https://schema.org',
      '@graph': [
        org,
        {
          '@type': 'WebSite',
          '@id': `${getSiteOrigin()}/#website`,
          url: getSiteOrigin(),
          name: siteName,
          alternateName: ['老实人 AI', '老实人ai', 'Laoshiren AI', 'laoshirenai'],
          description: seo.description,
          publisher: { '@id': org['@id'] },
          inLanguage: 'zh-CN'
        }
      ]
    }
  }

  if (route.name === 'Docs') {
    return {
      '@context': 'https://schema.org',
      '@graph': [
        org,
        {
          '@type': 'CollectionPage',
          '@id': `${seo.canonicalUrl}#webpage`,
          url: seo.canonicalUrl,
          name: seo.title,
          description: seo.description,
          publisher: { '@id': org['@id'] },
          inLanguage: 'zh-CN'
        },
        buildBreadcrumb([
          { name: siteName, url: absoluteUrl('/') },
          { name: '文档', url: seo.canonicalUrl }
        ])
      ]
    }
  }

  if (route.name !== 'DocsPage') {
    return {
      '@context': 'https://schema.org',
      '@graph': [
        org,
        {
          '@type': route.name === 'Enterprise' ? 'SoftwareApplication' : 'WebPage',
          '@id': `${seo.canonicalUrl}#webpage`,
          url: seo.canonicalUrl,
          name: seo.title,
          description: seo.description,
          publisher: { '@id': org['@id'] },
          inLanguage: 'zh-CN'
        },
        buildBreadcrumb([
          { name: siteName, url: absoluteUrl('/') },
          { name: String(route.meta.title || seo.title), url: seo.canonicalUrl }
        ])
      ]
    }
  }

  const doc = findDocItemBySlug(String(route.params.slug || ''))
  return {
    '@context': 'https://schema.org',
    '@graph': [
      org,
      {
        '@type': 'TechArticle',
        '@id': `${seo.canonicalUrl}#article`,
        headline: doc?.title || seo.title,
        description: seo.description,
        url: seo.canonicalUrl,
        mainEntityOfPage: seo.canonicalUrl,
        dateModified: doc?.lastModified || docsLastModified,
        author: { '@id': org['@id'] },
        publisher: { '@id': org['@id'] },
        inLanguage: 'zh-CN'
      },
      buildBreadcrumb([
        { name: siteName, url: absoluteUrl('/') },
        { name: '文档', url: absoluteUrl('/docs') },
        { name: doc?.title || '文档', url: seo.canonicalUrl }
      ])
    ]
  }
}

function resolveRouteSeo(route: RouteLocationNormalizedLoaded, options: SeoOptions): RouteSeo {
  const siteName = options.siteName?.trim() || DEFAULT_SITE_NAME
  const canonicalUrl = absoluteUrl(resolveCanonicalPath(route))
  const title = resolveTitle(route, siteName, options.customTitle)
  const description = resolveDescription(route)
  const robots = isIndexableRoute(route) ? 'index,follow' : 'noindex,nofollow'
  const ogType: RouteSeo['ogType'] = route.name === 'DocsPage' ? 'article' : 'website'
  const withoutStructuredData = { title, description, canonicalUrl, robots, ogType }
  return {
    ...withoutStructuredData,
    structuredData: buildStructuredData(route, withoutStructuredData, siteName, options.siteLogo || DEFAULT_SITE_LOGO)
  }
}

export function updateRouteSeo(route: RouteLocationNormalizedLoaded, options: SeoOptions = {}): void {
  const siteName = options.siteName?.trim() || DEFAULT_SITE_NAME
  const seo = resolveRouteSeo(route, options)
  const ogImage = absoluteUrl(DEFAULT_OG_IMAGE)

  document.title = seo.title
  setMetaByName('description', seo.description)
  setMetaByName('robots', seo.robots)
  setCanonical(seo.canonicalUrl)

  setMetaByProperty('og:site_name', siteName)
  setMetaByProperty('og:locale', 'zh_CN')
  setMetaByProperty('og:type', seo.ogType)
  setMetaByProperty('og:title', seo.title)
  setMetaByProperty('og:description', seo.description)
  setMetaByProperty('og:url', seo.canonicalUrl)
  setMetaByProperty('og:image', ogImage)
  setMetaByProperty('og:image:width', '1200')
  setMetaByProperty('og:image:height', '630')

  setMetaByName('twitter:card', 'summary_large_image')
  setMetaByName('twitter:title', seo.title)
  setMetaByName('twitter:description', seo.description)
  setMetaByName('twitter:image', ogImage)

  setStructuredData(seo.structuredData)
}
