import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import router from '@/router'

describe('public Service Status route', () => {
  it('uses the standalone live status view with public SEO metadata', () => {
    const route = router.getRoutes().find((item) => item.path === '/status')
    expect(route).toBeDefined()
    expect(route?.meta.requiresAuth).toBe(false)
    expect(route?.meta.title).toBe('服务状态')
    expect(String(route?.meta.description)).toContain('Builder Pass')
    expect(route?.meta.publicDocSlug).toBeUndefined()
  })

  it('publishes status-specific build-time SEO instead of the legacy support article', () => {
    const manifest = JSON.parse(readFileSync(resolve(process.cwd(), 'public/seo-manifest.json'), 'utf8'))
    const status = manifest.routes.find((route: { path: string }) => route.path === '/status')
    expect(status.description).toContain('Builder Pass')
    expect(status.staticHtml).toContain('查看实时服务状态')
    expect(status.staticHtml).not.toContain('模型检测报告：首页')
  })
})
