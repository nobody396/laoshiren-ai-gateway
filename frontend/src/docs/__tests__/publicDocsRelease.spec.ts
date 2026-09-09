import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import { simpleClientGuides } from '@/docs/guides/simpleClientGuides'
import { clientMatrix } from '@/generated/clientMatrix'

describe('public documentation release', () => {
  it('keeps every public guide and category in the SEO route manifest', () => {
    const features = JSON.parse(readFileSync(resolve(process.cwd(), 'config/public-features.json'), 'utf8'))
    const manifest = JSON.parse(readFileSync(resolve(process.cwd(), 'public/seo-manifest.json'), 'utf8'))
    const paths = new Set<string>(manifest.routes.map((route: { path: string }) => route.path))

    expect(features.docs).toBe(true)
    for (const path of [
      '/docs',
      '/docs/category/start',
      '/docs/category/api',
      '/docs/category/integrations',
      '/docs/category/models',
    ]) expect(paths.has(path)).toBe(true)
    const publicClientIDs = new Set(simpleClientGuides.map((guide) => guide.id))
    for (const client of clientMatrix.filter((entry) => publicClientIDs.has(entry.id))) {
      expect(paths.has(`/docs/${client.slug}`)).toBe(true)
    }
  })
})
