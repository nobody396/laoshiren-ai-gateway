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

export async function getDownloads(tool: DownloadToolID): Promise<DownloadManifest> {
  const { data } = await apiClient.get<DownloadManifest>(`/resources/${tool}`)
  return data
}

export async function downloadAsset(tool: DownloadToolID, asset: DownloadAsset): Promise<void> {
  const { data } = await apiClient.get<Blob>(`/resources/${tool}/download/${asset.id}`, {
    responseType: 'blob'
  })
  const url = window.URL.createObjectURL(data)
  const link = document.createElement('a')
  link.href = url
  link.download = asset.name
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

export async function getCCSwitchDownloads(): Promise<DownloadManifest> {
  return getDownloads('cc-switch')
}

export async function downloadCCSwitchAsset(asset: DownloadAsset): Promise<void> {
  return downloadAsset('cc-switch', asset)
}

export const resourcesAPI = {
  getDownloads,
  downloadAsset,
  getCCSwitchDownloads,
  downloadCCSwitchAsset
}
