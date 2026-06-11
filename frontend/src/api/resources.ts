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
  const frame = document.createElement('iframe')
  frame.src = url
  frame.title = `download-${asset.id}`
  frame.style.cssText = 'position:absolute;width:0;height:0;border:0;opacity:0;pointer-events:none'
  document.body.appendChild(frame)
  window.setTimeout(() => {
    frame.remove()
  }, 5 * 60 * 1000)
}

export async function getCCSwitchDownloads(): Promise<DownloadManifest> {
  return getDownloads('cc-switch')
}

export async function downloadCCSwitchAsset(asset: DownloadAsset): Promise<void> {
  return downloadAsset('cc-switch', asset)
}

export const resourcesAPI = {
  getDownloads,
  createDownloadURL,
  buildResourceDownloadURL,
  downloadAsset,
  getCCSwitchDownloads,
  downloadCCSwitchAsset
}
