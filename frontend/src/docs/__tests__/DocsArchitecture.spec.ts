import { describe, expect, it } from 'vitest'
import { docsConfig } from '@/docs/config'
import docsContentSource from '@/components/docs/DocsContent.vue?raw'
import docsCategoryHomeSource from '@/components/docs/DocsCategoryHome.vue?raw'
import docsLayoutSource from '@/components/docs/DocsLayout.vue?raw'
import docsModelCatalogSource from '@/components/docs/DocsModelCatalog.vue?raw'
import docsViewSource from '@/views/docs/DocsView.vue?raw'
import routerSource from '@/router/index.ts?raw'
import { clientMatrix } from '@/generated/clientMatrix'

const markdownModules = import.meta.glob('../content/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

const primarySlugs = new Set(docsConfig.flatMap(category => category.items.map(item => item.slug)))

const appRoutes = new Set(
  [...routerSource.matchAll(/path:\s*'([^']+)'/g)].map(match => match[1]),
)

function stripQueryAndHash(value: string): string {
  return value.split('#')[0].split('?')[0]
}

describe('documentation information architecture', () => {
  it('publishes the reviewed developer documentation architecture including all client contracts', () => {
    expect(docsConfig.map(category => category.title)).toEqual([
      '快速开始',
      'API 参考',
      '工具集成',
      '模型目录',
    ])

    expect(primarySlugs.size).toBe(25)
    for (const slug of [
      'api-responses',
      'api-chat-completions',
      'api-anthropic-messages',
      'api-gemini',
      'api-images',
      'integration-claude-code',
      'integration-codex',
      'integration-kimi-code',
      'integration-opencode',
      'integration-zcode',
      'integration-antigravity',
    ]) {
      expect(primarySlugs).toContain(slug)
    }
    for (const client of clientMatrix) expect(primarySlugs).toContain(client.slug)
  })

  it('removes whole-page copy while preserving code-block copy', () => {
    expect(docsContentSource).not.toContain('docs-doc-copy-btn')
    expect(docsContentSource).not.toContain('一键复制')
    expect(docsContentSource).toContain('code-copy-btn')
  })

  it('does not expose the support entry to signed-out documentation visitors', () => {
    expect(docsLayoutSource).toContain('<CustomerServiceButton v-if="authStore.isAuthenticated" />')
  })

  it('renders the client wire-protocol matrix from the generated 14-client source', () => {
    expect(docsCategoryHomeSource).toContain('客户端原生协议')
    expect(docsCategoryHomeSource).toContain('Gemini GenerateContent')
    expect(docsCategoryHomeSource).toContain("import { clientMatrix } from '@/generated/clientMatrix'")
    expect(docsCategoryHomeSource).toContain('clientMatrix.map(client =>')
    expect(clientMatrix).toHaveLength(14)
    expect(clientMatrix.map(client => client.name)).toContain('Visual Studio Code Local Agent')
  })

  it('renders model cards from strict generated contracts', () => {
    expect(docsModelCatalogSource).not.toContain('工具与模型兼容表')
    expect(docsModelCatalogSource).toContain('模型原生输入：点亮表示支持，灰色表示不支持')
    expect(docsModelCatalogSource).toContain('DocsModelContractCard')
    expect(docsModelCatalogSource).toContain('modelDocContractById[model.id]')
    expect(docsModelCatalogSource).not.toContain("model.id === 'glm-5.3'")
    expect(docsModelCatalogSource).not.toContain('月卡 Plus / Pro / Max')
  })

  it('refuses to render unregistered legacy docs through the Docs route', () => {
    expect(docsViewSource).toContain('if (!findDocItemBySlug(newSlug))')
  })

  it('resolves every internal Markdown link to a document or application route', () => {
    const failures: string[] = []

    for (const [file, markdown] of Object.entries(markdownModules)) {
      const fileSlug = file.split('/').pop()!.replace(/\.md$/, '')
      if (!primarySlugs.has(fileSlug)) continue
      for (const match of markdown.matchAll(/(?<!!)\[[^\]]+\]\(([^)\s]+)(?:\s+[^)]*)?\)/g)) {
        const href = match[1].replace(/^<|>$/g, '')
        if (/^(https?:|mailto:|tel:|#)/.test(href)) continue

        const target = stripQueryAndHash(href)
        if (target.startsWith('/docs/')) {
          const slug = target.slice('/docs/'.length)
          if (!primarySlugs.has(slug)) failures.push(`${file} -> ${href}`)
          continue
        }

        if (target.startsWith('/api/')) continue

        if (target.startsWith('/')) {
          if (!appRoutes.has(target)) failures.push(`${file} -> ${href}`)
          continue
        }

        const slug = target.replace(/^\.\//, '').replace(/\.md$/, '')
        if (!primarySlugs.has(slug)) failures.push(`${file} -> ${href}`)
      }
    }

    expect(failures).toEqual([])
  })
})
