export interface GatewayModel {
  id: string
}

interface GatewayModelsResponse {
  data?: Array<{ id?: unknown }>
}

interface GatewayErrorResponse {
  code?: unknown
  message?: unknown
  error?: { code?: unknown; message?: unknown }
}

const normalizeGatewayBaseUrl = (value: string): string => {
  const normalized = value.trim().replace(/\/+$/, '')
  return normalized.endsWith('/v1') ? normalized.slice(0, -3) : normalized
}

/**
 * Read the exact model list authorized for an API key. This is the same
 * authenticated /v1/models contract used by OpenAI-compatible clients, so CC
 * Switch imports cannot drift from the selected key's group routing.
 */
export async function getGatewayModels(apiBaseUrl: string, apiKey: string): Promise<string[]> {
  const baseUrl = normalizeGatewayBaseUrl(apiBaseUrl)
  const response = await fetch(`${baseUrl}/v1/models`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${apiKey}`
    }
  })
  if (!response.ok) {
    let payload: GatewayErrorResponse = {}
    try {
      payload = await response.json() as GatewayErrorResponse
    } catch {
      // Non-JSON errors still fall back to the HTTP status below.
    }
    const code = typeof payload.code === 'string'
      ? payload.code
      : (typeof payload.error?.code === 'string' ? payload.error.code : '')
    const upstreamMessage = typeof payload.message === 'string'
      ? payload.message
      : (typeof payload.error?.message === 'string' ? payload.error.message : '')

    if (code === 'API_KEY_QUOTA_EXHAUSTED') {
      throw new Error(`这把 API Key 设置的额度已用完。请到 API 密钥页面重置用量、提高或关闭额度上限，也可以换一把 Key。（HTTP ${response.status}）`)
    }
    if (response.status === 401) {
      throw new Error('API Key 无效、已删除或已过期，请重新创建 Key 后再试。（HTTP 401）')
    }
    if (upstreamMessage) {
      throw new Error(`读取模型失败：${upstreamMessage}（HTTP ${response.status}）`)
    }
    throw new Error(`读取模型失败，网关返回 HTTP ${response.status}`)
  }

  const payload = await response.json() as GatewayModelsResponse
  if (!Array.isArray(payload.data)) {
    throw new Error('Gateway model discovery returned an invalid response')
  }

  const seen = new Set<string>()
  return payload.data.flatMap((model) => {
    const modelID = typeof model.id === 'string' ? model.id.trim() : ''
    if (!modelID || seen.has(modelID)) return []
    seen.add(modelID)
    return [modelID]
  })
}
