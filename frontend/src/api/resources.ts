import { apiClient } from './client'

export type DownloadToolID = 'cc-switch' | 'codex' | 'codex-plus-plus' | 'claude-desktop'

export interface DownloadAsset {
  id: string
  name: string
  size: number
  sha256: string
  platform: 'windows' | 'macos' | 'linux' | 'other'
  arch: string
  role?: 'installer' | 'claude-desktop-code'
  component_version?: string
  upstream_sha256?: string
  upstream_compressed_size?: number
  download_url?: string
}

export interface DownloadManifest {
  tool: string
  repo: string
  version: string
  release_name: string
  published_at: string
  updated_at: string
  assets: DownloadAsset[]
}

export interface DownloadURLResponse {
  token: string
  expires_at: string
}

export type DownloadVersionState =
  | 'current'
  | 'cached'
  | 'npm-mirror'
  | 'update-available'
  | 'cache-missing'
  | 'official-unavailable'
  | 'unknown'

export interface DownloadVersionStatus {
  tool: 'codex' | 'codex-plus-plus' | 'claude-desktop' | 'claude-code' | 'grok-build' | 'git-for-windows' | 'cc-switch'
  name: string
  cached_version: string
  cached_updated_at: string
  official_version: string
  official_published_at: string
  official_url: string
  cache_mode: 'cached' | 'npm-mirror'
  state: DownloadVersionState
  note: string
}

export type ClientSetupTarget = 'claude' | 'codex' | 'grok' | 'gemini' | 'kimi' | 'opencode'

export interface ClientSetupTicket<TTarget extends string = ClientSetupTarget> {
  ticket: string
  expires_in: number
  target: TTarget
  key_name: string
  group_name: string
  client_id?: string
  client_version_key?: string
  protocol?: ClientSetupProtocol
  model_id?: string
  os?: ClientSetupOS
}

export type ClientSetupProtocol = 'responses' | 'chat_completions' | 'messages' | 'generate_content'
export type ClientSetupOS = 'macos' | 'linux' | 'windows'

export interface ClientSetupOption {
  client_id: 'codex' | 'claude-code' | 'grok-build' | 'kimi-code' | 'opencode'
  name: string
}

export interface ClientSetupSelection {
  api_key_id: number
  client_id: string
  client_version_key: string
  protocol: ClientSetupProtocol
  model_id: string
  os: ClientSetupOS
}

export async function getDownloads(tool: DownloadToolID): Promise<DownloadManifest> {
  const { data } = await apiClient.get<DownloadManifest>(`/resources/${tool}`)
  return data
}

export async function createDownloadURL(tool: DownloadToolID, asset: DownloadAsset): Promise<DownloadURLResponse> {
  const { data } = await apiClient.post<DownloadURLResponse>(`/resources/${tool}/download-url/${asset.id}`)
  return data
}

export function buildResourceDownloadURL(token: string): string {
  const baseURL = String(apiClient.defaults.baseURL || '/api/v1').replace(/\/+$/, '')
  return `${baseURL}/resource-downloads/${encodeURIComponent(token)}`
}

export async function downloadAsset(tool: DownloadToolID, asset: DownloadAsset): Promise<void> {
  const { token } = await createDownloadURL(tool, asset)
  const url = buildResourceDownloadURL(token)
  // Use a top-level navigation so X-Frame-Options/CSP frame restrictions cannot block the file response.
  window.location.assign(url)
}

export async function getCCSwitchDownloads(): Promise<DownloadManifest> {
  return getDownloads('cc-switch')
}

export async function downloadCCSwitchAsset(asset: DownloadAsset): Promise<void> {
  return downloadAsset('cc-switch', asset)
}

export async function createClientSetupTicket(target: ClientSetupTarget): Promise<ClientSetupTicket> {
  const { data } = await apiClient.post<ClientSetupTicket>('/resources/setup-ticket', { target })
  return data
}

export async function createClientSetupTicketForAPIKey(apiKeyId: number): Promise<ClientSetupTicket> {
  const { data } = await apiClient.post<ClientSetupTicket>('/resources/setup-ticket', {
    api_key_id: apiKeyId
  })
  return data
}

export async function createClientSetupTicketForSelection(selection: ClientSetupSelection): Promise<ClientSetupTicket<string>> {
  const { data } = await apiClient.post<ClientSetupTicket<string>>('/resources/setup-ticket', selection)
  return data
}

export async function getClientSetupOptions(apiKeyId: number, os: ClientSetupOS): Promise<ClientSetupOption[]> {
  const { data } = await apiClient.get<ClientSetupOption[]>('/resources/setup-options', {
    params: { api_key_id: apiKeyId, os }
  })
  return data
}

export async function createClientSetupTicketForOption(apiKeyId: number, clientId: ClientSetupOption['client_id'], os: ClientSetupOS): Promise<ClientSetupTicket<string>> {
  const { data } = await apiClient.post<ClientSetupTicket<string>>('/resources/setup-ticket', {
    api_key_id: apiKeyId,
    client_id: clientId,
    os
  })
  return data
}

export async function getVersionStatus(): Promise<DownloadVersionStatus[]> {
  const { data } = await apiClient.get<DownloadVersionStatus[]>('/resources/version-status')
  return data
}

export const resourcesAPI = {
  getDownloads,
  createDownloadURL,
  buildResourceDownloadURL,
  downloadAsset,
  getCCSwitchDownloads,
  downloadCCSwitchAsset,
  createClientSetupTicket,
  createClientSetupTicketForAPIKey,
  createClientSetupTicketForSelection,
  getClientSetupOptions,
  createClientSetupTicketForOption,
  getVersionStatus
}
