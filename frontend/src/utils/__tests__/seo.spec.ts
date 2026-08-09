import { beforeEach, describe, expect, it } from 'vitest'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { updateRouteSeo } from '../seo'

function route(overrides: Partial<RouteLocationNormalizedLoaded>): RouteLocationNormalizedLoaded {
  return {
    name: 'Home',
    path: '/',
    fullPath: '/',
    query: {},
    hash: '',
    params: {},
    matched: [],
    redirectedFrom: undefined,
    meta: {},
    ...overrides,
  } as RouteLocationNormalizedLoaded
}

function content(selector: string): string | null {
  return document.head.querySelector(selector)?.getAttribute('content') ?? null
}

describe('updateRouteSeo', () => {
  beforeEach(() => {
    document.title = ''
    document.head.innerHTML = ''
  })

  it('sets indexable metadata for the home page', () => {
    updateRouteSeo(route({
      name: 'Home',
      meta: {
        title: 'AI 编码网关',
        titleSiteNameFirst: true,
      },
    }))

    expect(document.title).toBe('老实人AI - AI 编码网关')
    expect(content('meta[name="robots"]')).toBe('index,follow')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://laoshirenai.com/')
    expect(content('meta[property="og:type"]')).toBe('website')
    expect(document.head.querySelectorAll('script[type="application/ld+json"]').length).toBe(1)
  })

  it('sets article metadata for docs pages', () => {
    updateRouteSeo(route({
      name: 'DocsPage',
      path: '/docs/claude-code-quickstart',
      params: { slug: 'claude-code-quickstart' },
      meta: { title: '文档' },
    }))

    expect(document.title).toBe('Claude Code 快速开始指南 - 文档 - 老实人AI')
    expect(content('meta[name="robots"]')).toBe('index,follow')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://laoshirenai.com/docs/claude-code-quickstart'
    )
    expect(content('meta[property="og:type"]')).toBe('article')
    expect(document.head.querySelector('script[type="application/ld+json"]')?.textContent).toContain('TechArticle')
  })

  it('sets indexable article metadata for a public changelog entry', () => {
    updateRouteSeo(route({
      name: 'ChangelogDetail',
      path: '/changelog/first-public-update',
      params: { slug: 'first-public-update' },
      meta: { title: '更新日志', description: '公开构建记录' },
    }), {
      customTitle: '我们开始公开记录产品进展 - 更新日志',
      customDescription: '记录已经做成的事情和背后的原因。',
    })

    expect(document.title).toBe('我们开始公开记录产品进展 - 更新日志 - 老实人AI')
    expect(content('meta[name="description"]')).toBe('记录已经做成的事情和背后的原因。')
    expect(content('meta[name="robots"]')).toBe('index,follow')
    expect(content('meta[property="og:type"]')).toBe('article')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://laoshirenai.com/changelog/first-public-update'
    )
    expect(document.head.querySelector('script[type="application/ld+json"]')?.textContent).toContain('Article')
  })

  it('marks private routes as noindex and removes structured data', () => {
    updateRouteSeo(route({
      name: 'Login',
      path: '/login',
      meta: { title: 'Login' },
    }))

    expect(document.title).toBe('Login - 老实人AI')
    expect(content('meta[name="robots"]')).toBe('noindex,nofollow')
    expect(document.head.querySelectorAll('script[type="application/ld+json"]').length).toBe(0)
  })

  it('keeps the affiliate payment privacy notice indexable', () => {
    updateRouteSeo(route({
      name: 'LegalAffiliatePaymentPrivacy',
      path: '/legal/affiliate-payment-privacy',
      meta: {
        title: '合伙人收款资料隐私告知',
        description: '说明合伙人收款资料的处理方式。',
      },
    }))

    expect(content('meta[name="robots"]')).toBe('index,follow')
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(
      'https://laoshirenai.com/legal/affiliate-payment-privacy'
    )
    expect(document.head.querySelectorAll('script[type="application/ld+json"]').length).toBe(1)
  })

  it('updates existing tags instead of duplicating them', () => {
    updateRouteSeo(route({
      name: 'Home',
      meta: {
        title: 'AI 编码网关',
        titleSiteNameFirst: true,
      },
    }))
    updateRouteSeo(route({
      name: 'DocsPage',
      path: '/docs/codex-quickstart',
      params: { slug: 'codex-quickstart' },
      meta: { title: '文档' },
    }))

    expect(document.head.querySelectorAll('meta[name="description"]').length).toBe(1)
    expect(document.head.querySelectorAll('meta[name="robots"]').length).toBe(1)
    expect(document.head.querySelectorAll('meta[property="og:title"]').length).toBe(1)
    expect(document.head.querySelectorAll('link[rel="canonical"]').length).toBe(1)
    expect(document.title).toBe('Codex 快速开始指南 - 文档 - 老实人AI')
  })
})
