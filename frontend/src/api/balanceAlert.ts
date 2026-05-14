import { apiClient } from './client'

export interface BalanceAlertConfig {
  enabled: boolean
  threshold: string | null
  email: string
  effective_threshold: string
  effective_email: string
}

export interface UpdateBalanceAlertRequest {
  enabled?: boolean
  threshold?: number | null
  clear_threshold?: boolean
  email?: string
}

export interface BalanceAlertFormInput {
  enabled: boolean
  threshold: number | null
  email: string
}

export function buildUpdateBalanceAlertRequest(
  input: BalanceAlertFormInput
): UpdateBalanceAlertRequest {
  return {
    enabled: input.enabled,
    threshold: input.threshold ?? undefined,
    clear_threshold: input.threshold === null,
    email: input.email || ''
  }
}

export async function getBalanceAlertConfig(): Promise<BalanceAlertConfig> {
  const { data } = await apiClient.get<BalanceAlertConfig>('/user/balance-alert')
  return data
}

export async function updateBalanceAlertConfig(
  req: UpdateBalanceAlertRequest
): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/user/balance-alert', req)
  return data
}

export const balanceAlertAPI = {
  getConfig: getBalanceAlertConfig,
  updateConfig: updateBalanceAlertConfig
}
