import { describe, expect, it } from 'vitest'
import { docsConfig } from '@/docs/config'
import apiReference from '@/docs/content/api-reference.md?raw'

describe('API reference documentation contract', () => {
  it('is registered in the public documentation navigation', () => {
    const items = docsConfig.flatMap(category => category.items)
    const entry = items.find(item => item.slug === 'api-reference')

    expect(entry?.title).toBe('API 参考总览')
    expect(entry?.lastModified).toBe('2026-08-29')
  })

  it('documents every supported primary protocol boundary', () => {
    const requiredContracts = [
      'https://api.laoshirenai.com/v1',
      'https://api.laoshirenai.com/gpt-image/v1',
      'POST /v1/responses',
      'POST /v1/chat/completions',
      'POST /v1/messages',
      'POST /v1beta/models/{model}:generateContent',
      'POST /gpt-image/v1/images/generations',
      'GET /v1/models',
      'response.completed',
      'X-Request-ID',
    ]

    for (const contract of requiredContracts) {
      expect(apiReference).toContain(contract)
    }
  })

  it('contains parameter, retry, security and production-check guidance', () => {
    for (const heading of [
      '## 2. 鉴权与请求头',
      '### 4.3 常用参数',
      '## 9. 用量与计费字段',
      '## 10. HTTP 状态码与重试',
      '## 13. 上线前检查清单',
    ]) {
      expect(apiReference).toContain(heading)
    }

    expect(apiReference).not.toMatch(/Bearer sk-[A-Za-z0-9_-]{8,}/)
    expect(apiReference).not.toContain('YOUR_REAL_API_KEY')
  })
})
