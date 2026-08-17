import { afterEach, describe, expect, it, vi } from 'vitest'

import { getGatewayModels } from '@/api/gatewayModels'

describe('getGatewayModels', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('reads and deduplicates the selected API key model list', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      object: 'list',
      data: [
        { id: 'gpt-5.6-sol' },
        { id: 'gpt-daybreak-blue-latest' },
        { id: 'gpt-5.6-sol' }
      ]
    }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(getGatewayModels('https://api.laoshirenai.com/v1/', 'test-key-placeholder'))
      .resolves.toEqual(['gpt-5.6-sol', 'gpt-daybreak-blue-latest'])
    expect(fetchMock).toHaveBeenCalledWith('https://api.laoshirenai.com/v1/models', {
      method: 'GET',
      headers: { Authorization: 'Bearer test-key-placeholder' }
    })
  })

  it('fails closed when model discovery is unavailable', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 503 })))

    await expect(getGatewayModels('https://api.laoshirenai.com', 'test-key-placeholder'))
      .rejects.toThrow(/HTTP 503/)
  })
})
