export interface GatewayModel {
  id: string
}

interface GatewayModelsResponse {
  data?: Array<{ id?: unknown }>
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
    throw new Error(`Gateway model discovery failed with HTTP ${response.status}`)
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
