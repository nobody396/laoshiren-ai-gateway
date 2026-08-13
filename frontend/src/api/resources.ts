import { apiClient } from './client'

export type DownloadToolID = 'cc-switch' | 'codex' | 'codex-plus-plus' | 'claude-desktop'

export interface DownloadAsset {
  id: string
  name: string
  size: number
  sha256: string
  platform: 'windows' | 'macos' | 'linux' | 'other'
  arch: string
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

export type ClientSetupTarget = 'claude' | 'codex' | 'grok'

export interface ClientSetupTicket {
  ticket: string
  expires_in: number
  target: ClientSetupTarget
  key_name: string
  group_name: string
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
  getVersionStatus
}
