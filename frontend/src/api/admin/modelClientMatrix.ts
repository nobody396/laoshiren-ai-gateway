import { apiClient } from '../client'
import type { ModelDocContract } from '@/generated/modelDocContracts'

export interface AdminModelClientMatrixPayload {
  schema_version: 1
  counts: {
    models: number
    clients: number
    intersections: number
  }
  contracts: ModelDocContract[]
  // Transport layer keeps the server-owned matrix opaque. The admin feature
  // validates/interprets its domain shape at the presentation boundary.
  client_matrix: unknown
  evidence_index: Record<string, AdminEvidenceReceipt>
}

export interface AdminEvidenceReceipt {
  evidence_type?: string
  result?: string
  observed_at?: string
  artifact_uri?: string
  artifact_sha256?: string
  summary?: string
  target?: Record<string, unknown>
}

export async function getAdminModelClientMatrix(): Promise<AdminModelClientMatrixPayload> {
  const { data } = await apiClient.get<AdminModelClientMatrixPayload>('/admin/model-client-matrix')
  return data
}
