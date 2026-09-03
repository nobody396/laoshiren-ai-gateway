import { clientMatrix, type ClientMatrixEntry, type ClientMatrixProtocol } from '@/generated/clientMatrix'
import { modelDocContractById } from '@/generated/modelDocContracts'

export interface ClientAutoConfigOption {
  client: ClientMatrixEntry
  protocols: ClientMatrixProtocol[]
}

export function clientAutoConfigOptionsForModel(modelId: string): ClientAutoConfigOption[] {
  const contract = modelDocContractById[modelId]
  if (!contract) return []
  const protocols = new Set(contract.protocols.filter(row => row.status === 'verified').map(row => row.name))
  return clientMatrix
    .filter(client => client.one_click_status === 'ready')
    .map(client => ({ client, protocols: client.protocols.filter(protocol => protocols.has(protocol)) }))
    .filter(option => option.protocols.length > 0)
}

export function preferredAutoConfigProtocol(modelId: string, protocols: readonly ClientMatrixProtocol[]): ClientMatrixProtocol | undefined {
  const recommended = modelDocContractById[modelId]?.recommended_protocol
  return protocols.find(protocol => protocol === recommended) ?? protocols[0]
}
