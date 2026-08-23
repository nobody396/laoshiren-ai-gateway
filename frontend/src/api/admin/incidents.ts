import { apiClient } from '@/api/client'
import type { ServiceStatus } from '@/api/serviceStatus'

export type IncidentPhase = 'investigating' | 'identified' | 'mitigating' | 'monitoring' | 'resolved'

export interface IncidentSettings { enabled: boolean; public_enabled: boolean }
export interface IncidentCandidateProduct { product_code: string; product_name: string; latest_status: ServiceStatus; first_observed_at: string; last_observed_at: string; first_customer_failure_at?: string }
export interface IncidentCandidate { id: number; state: 'open' | 'confirmed' | 'dismissed' | 'recovered'; first_observed_at: string; last_observed_at: string; dismissed_reason?: string; products: IncidentCandidateProduct[] }
export interface IncidentSegment { id: number; started_at: string; ended_at?: string; duration_seconds: number; start_observation_id?: number; end_observation_id?: number }
export interface IncidentProduct { id: number; product_code: string; product_name: string; current_status: ServiceStatus; affected_at: string; monitoring_since?: string; recovered_at?: string; last_customer_failure_at?: string; compensable_seconds: number; segments: IncidentSegment[] }
export interface IncidentUpdate { id: number; phase: IncidentPhase; kind: 'system' | 'operator' | 'transition'; internal_message: string; created_at: string }
export interface IncidentPublicUpdate { id: number; phase: IncidentPhase; message: string; published_at: string }
export interface Incident {
  id: number; public_id: string; phase: IncidentPhase; title: string; internal_summary: string
  observation_started_at: string; observation_ended_at?: string; customer_impact_started_at?: string; customer_impact_ended_at?: string
  monitoring_since?: string; resolved_at?: string; version: number; products: IncidentProduct[]; updates: IncidentUpdate[]; public_timeline: IncidentPublicUpdate[]
  evidence_gap: boolean
}
export interface IncidentAdminSnapshot { settings: IncidentSettings; candidates: IncidentCandidate[]; incidents: Incident[]; generated_at: string }

export async function getIncidentAdminSnapshot(signal?: AbortSignal) {
  const response = await apiClient.get<IncidentAdminSnapshot>('/admin/ops/incidents', { signal })
  return response.data
}
export async function updateIncidentSettings(settings: IncidentSettings) {
  const response = await apiClient.put<IncidentSettings>('/admin/ops/incidents/settings', settings)
  return response.data
}
export async function confirmIncidentCandidate(id: number, title: string, internalSummary: string) {
  const response = await apiClient.post<Incident>(`/admin/ops/incidents/candidates/${id}/confirm`, { title, internal_summary: internalSummary })
  return response.data
}
export async function dismissIncidentCandidate(id: number, message: string) {
  await apiClient.post(`/admin/ops/incidents/candidates/${id}/dismiss`, { message })
}
export async function transitionIncident(id: number, phase: IncidentPhase, message: string) {
  await apiClient.post(`/admin/ops/incidents/${id}/transition`, { phase, message })
}
export async function addIncidentUpdate(id: number, message: string) {
  await apiClient.post(`/admin/ops/incidents/${id}/updates`, { message })
}
export async function publishIncidentUpdate(id: number, message: string) {
  await apiClient.post(`/admin/ops/incidents/${id}/public-updates`, { message })
}
export async function acknowledgeIncidentEvidenceGap(id: number, message: string) {
  await apiClient.post(`/admin/ops/incidents/${id}/evidence-gap/acknowledge`, { message })
}
