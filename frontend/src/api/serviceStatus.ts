import { apiClient } from './client'

export type ServiceStatus =
  | 'operational'
  | 'degraded_performance'
  | 'partial_outage'
  | 'major_outage'
  | 'maintenance'
  | 'monitoring'

export interface ServiceStatusComponent {
  code: string
  display_name: string
  access_mode: 'http'
  status: ServiceStatus
  reason: string
  evidence_at?: string | null
  computed_at: string
}

export interface ServiceStatusProduct {
  code: string
  display_name: string
  status: ServiceStatus
  reason: string
  evidence_at?: string | null
  computed_at: string
  components: ServiceStatusComponent[]
}

export interface ServiceStatusFamily {
  code: string
  display_name: string
  products: ServiceStatusProduct[]
}

export interface ServiceStatusSnapshot {
  enabled: boolean
  generated_at: string
  families: ServiceStatusFamily[]
}

export async function getServiceStatus(): Promise<ServiceStatusSnapshot> {
  const response = await apiClient.get<ServiceStatusSnapshot>('/service-status')
  return response.data
}

export type PublicIncidentPhase = 'investigating' | 'identified' | 'mitigating' | 'monitoring' | 'resolved'
export interface PublicIncidentUpdate { phase: PublicIncidentPhase; message: string; published_at: string }
export interface PublicIncident { id: string; phase: PublicIncidentPhase; started_at: string; resolved_at?: string; affected_products: string[]; timeline: PublicIncidentUpdate[] }
export interface PublicIncidentSnapshot { enabled: boolean; generated_at: string; incidents: PublicIncident[] }

export async function getPublicIncidents(): Promise<PublicIncidentSnapshot> {
  const response = await apiClient.get<PublicIncidentSnapshot>('/service-incidents')
  return response.data
}
