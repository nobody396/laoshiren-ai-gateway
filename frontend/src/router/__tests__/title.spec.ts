import { describe, expect, it } from 'vitest'
import { resolveDocumentTitle } from '@/router/title'

describe('resolveDocumentTitle', () => {
  it('路由存在标题时，使用“路由标题 - 站点名”格式', () => {
    expect(resolveDocumentTitle('Usage Records', 'My Site')).toBe('Usage Records - My Site')
  })

  it('路由无标题时，回退到站点名', () => {
    expect(resolveDocumentTitle(undefined, 'My Site')).toBe('My Site')
  })

  it('站点名为空时，回退默认站点名', () => {
    expect(resolveDocumentTitle('Dashboard', '')).toBe('Dashboard - 老实人AI')
    expect(resolveDocumentTitle(undefined, '   ')).toBe('老实人AI')
  })

  it('站点名变更时仅影响后续路由标题计算', () => {
    const before = resolveDocumentTitle('Admin Dashboard', 'Alpha')
    const after = resolveDocumentTitle('Admin Dashboard', 'Beta')

    expect(before).toBe('Admin Dashboard - Alpha')
    expect(after).toBe('Admin Dashboard - Beta')
  })

  it('支持首页使用“站点名 - 标题”格式', () => {
    expect(resolveDocumentTitle('AI 编码中转', '老实人AI', undefined, { siteNameFirst: true })).toBe('老实人AI - AI 编码中转')
  })
})
