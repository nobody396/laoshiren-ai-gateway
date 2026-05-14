import type { UsageRequestType } from '@/types'

export interface UsageRequestTypeLike {
  request_type?: string | null
  stream?: boolean | null
  openai_ws_mode?: boolean | null
  model?: string | null
  upstream_model?: string | null
  billing_mode?: string | null
  image_count?: number | null
  inbound_endpoint?: string | null
  upstream_endpoint?: string | null
}

const VALID_REQUEST_TYPES = new Set<UsageRequestType>(['unknown', 'sync', 'stream', 'ws_v2', 'async'])

export const isUsageRequestType = (value: unknown): value is UsageRequestType => {
  return typeof value === 'string' && VALID_REQUEST_TYPES.has(value as UsageRequestType)
}

export const resolveUsageRequestType = (value: UsageRequestTypeLike): UsageRequestType => {
  if (isGPTImageAsyncUsage(value)) {
    return 'async'
  }
  if (isUsageRequestType(value.request_type)) {
    return value.request_type
  }
  if (value.openai_ws_mode) {
    return 'ws_v2'
  }
  return value.stream ? 'stream' : 'sync'
}

const isGPTImageAsyncUsage = (value: UsageRequestTypeLike): boolean => {
  const model = `${value.model || ''}`.trim().toLowerCase()
  const upstreamModel = `${value.upstream_model || ''}`.trim().toLowerCase()
  const inboundEndpoint = `${value.inbound_endpoint || ''}`.trim().toLowerCase()
  const upstreamEndpoint = `${value.upstream_endpoint || ''}`.trim().toLowerCase()
  return model === 'gpt-image-2' ||
    upstreamModel === 'gpt-image-2' ||
    inboundEndpoint.includes('/gpt-image/') ||
    upstreamEndpoint.includes('/gpt-image/')
}

export const requestTypeToLegacyStream = (requestType?: UsageRequestType | null): boolean | null | undefined => {
  if (!requestType || requestType === 'unknown') {
    return null
  }
  if (requestType === 'sync') {
    return false
  }
  if (requestType === 'async') {
    return null
  }
  return true
}
