import { describe, expect, it } from 'vitest'

import { defaultSlug, docsConfig, getAllDocItems, resolveDocSlug } from '../config'
import { clientMatrix } from '@/generated/clientMatrix'

describe('docs information architecture', () => {
  it('keeps one Docs surface with four primary sections', () => {
    expect(defaultSlug).toBe('quickstart')
    expect(docsConfig.map((section) => section.title)).toEqual([
      '快速开始',
      'API 参考',
      '工具集成',
      '模型目录',
    ])
    expect(getAllDocItems()).toHaveLength(25)
    expect(new Set(getAllDocItems().map((item) => item.slug)).size).toBe(25)
    const integrations = docsConfig.find(section => section.key === 'integrations')!
    expect(integrations.items).toHaveLength(14)
    expect(integrations.items.map(item => item.slug)).toEqual(clientMatrix.map(client => client.slug))
    expect(integrations.items.map(item => item.title)).toEqual(clientMatrix.map(client => client.name))
  })

  it('routes the highest-traffic legacy docs to the shortest replacement', () => {
    expect(resolveDocSlug('claude-code-quickstart')).toBe('integration-claude-code')
    expect(resolveDocSlug('codex-quickstart')).toBe('integration-codex')
    expect(resolveDocSlug('base-url-guide')).toBe('api-overview')
    expect(resolveDocSlug('api-key-group-guide')).toBe('models')
  })
})
