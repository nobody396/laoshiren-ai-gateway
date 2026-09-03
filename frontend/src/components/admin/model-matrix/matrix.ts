import type { PublicModelPricingCatalog, PublicModelPrice } from '@/api/publicPricing'
import type { ModelDocContract, ModelDocProtocolName } from '@/generated/modelDocContracts'

export type MatrixStatus = 'verified' | 'unsupported' | 'blocked'
export type MatrixProtocol = Exclude<ModelDocProtocolName, 'images'>

interface EvidenceCell {
  status?: string
  evidence?: string
  observed_at?: string
  verified_at?: string
  client_version?: string
}

interface ClientMatrixV2Client {
  id: string
  name: string
  slug: string
  icon: string
  one_click_status: string
  client_protocol: {
    protocols: Array<{
      protocol: MatrixProtocol
      support: 'supported' | 'unsupported'
      evidence?: EvidenceCell
      client_transport_features?: Record<string, string>
    }>
  }
  client_reasoning: {
    control_kind: string
    level_control: { values: string[]; values_rule?: string; default?: string }
    modes?: Array<{ id: string; kind: string; requested_level?: string; notes?: string }>
    fallback?: { strategy?: string; notes?: string }
    persistence?: { scope?: string; fields?: string[]; session_override?: boolean }
    notes?: string
  }
  client_config_os: {
    release: { version_key: string; display: string }
    os_support: Array<{
      os: string
      support: string
      config_files: Array<{ path: string; format: string; scope: string }>
      evidence?: EvidenceCell
    }>
    endpoint: { base_url_rule: string; credential_location: string }
    model_discovery: string
    model_slots: string[]
    owned_fields: string[]
    mutation: {
      merge_strategy: string
      strict_parse?: boolean
      preserve_unowned?: boolean
      atomic_write?: boolean
      backup?: { required?: boolean; recoverable?: boolean }
      idempotency?: { required?: boolean; assertion?: string }
    }
    verification_commands?: Array<{ operating_systems: string[]; commands: string[]; status?: string }>
    verification_contract?: string
  }
  verification_os?: string[]
}

export interface ClientMatrixV2Data {
  clients: ClientMatrixV2Client[]
}

export interface MatrixOsResult {
  os: string
  status: MatrixStatus
  evidence: string
  observedAt?: string
  clientVersion?: string
  configFiles: Array<{ path: string; format: string; scope: string }>
}

export interface MatrixPricing {
  source: 'public' | 'contract'
  group: string
  currency: string
  unit: string
  inputPrice: number | null
  outputPrice: number | null
  cacheWritePrice: number | null
  cacheReadPrice: number | null
}

export interface ModelClientMatrixRow {
  key: string
  model: ModelDocContract
  client: ClientMatrixV2Client
  protocol: MatrixProtocol
  candidate: boolean
  status: MatrixStatus
  statusReason: string
  osResults: MatrixOsResult[]
  reasoningLevels: string[]
  clientRequestedLevels: string[]
  clientModes: Array<{ id: string; requested_level?: string; notes?: string }>
  prices: MatrixPricing[]
}

const PROTOCOLS: readonly MatrixProtocol[] = ['responses', 'chat_completions', 'messages', 'generate_content']
const STATUS_ORDER: Record<MatrixStatus, number> = { verified: 0, unsupported: 1, blocked: 2 }

function cellStatus(value: unknown): MatrixStatus | undefined {
  if (!value || typeof value !== 'object') return undefined
  const status = (value as EvidenceCell).status
  if (status === 'verified' || status === 'unsupported' || status === 'blocked') return status
  if (status === 'unverified' || status === 'carried_forward') return 'blocked'
  return undefined
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

function clientEvidence(
  contract: ModelDocContract,
  clientName: string,
  protocol: MatrixProtocol,
  os: string,
): EvidenceCell | undefined {
  const matrix = asRecord(contract.test_matrix)
  const clients = asRecord(matrix.clients)
  const client = asRecord(clients[clientName])
  const protocolCells = asRecord(client[protocol])
  const exact = protocolCells[os]
  return exact && typeof exact === 'object' ? exact as EvidenceCell : undefined
}

function modelProtocol(contract: ModelDocContract, protocol: MatrixProtocol) {
  return contract.protocols.find(item => item.name === protocol)
}

function modelReasoningLevels(contract: ModelDocContract): string[] {
  return contract.reasoning?.model_levels ?? []
}

function requestedReasoningLevels(client: ClientMatrixV2Client): string[] {
  return client.client_reasoning.level_control.values ?? []
}

function reasoningIntersection(contract: ModelDocContract, client: ClientMatrixV2Client): string[] {
  const modelLevels = modelReasoningLevels(contract)
  const requested = requestedReasoningLevels(client)
  if (!modelLevels.length) return []
  if (!requested.length) return modelLevels
  const modelSet = new Set(modelLevels)
  return requested.filter(level => modelSet.has(level))
}

function rowStatus(osResults: MatrixOsResult[]): MatrixStatus {
  if (osResults.length > 0 && osResults.every(item => item.status === 'verified')) return 'verified'
  if (osResults.length > 0 && osResults.every(item => item.status === 'unsupported')) return 'unsupported'
  return 'blocked'
}

function contractPriceRows(contract: ModelDocContract): MatrixPricing[] {
  const pricing = asRecord(asRecord(contract.test_matrix).pricing)
  return Object.entries(pricing).flatMap(([group, raw]) => {
    const item = asRecord(raw)
    if (cellStatus(item) !== 'verified') return []
    return [{
      source: 'contract' as const,
      group,
      currency: typeof item.currency === 'string' ? item.currency : 'CNY',
      unit: typeof item.unit === 'string' ? item.unit : 'per_1m_tokens',
      inputPrice: typeof item.input_price === 'number' ? item.input_price : null,
      outputPrice: typeof item.output_price === 'number' ? item.output_price : null,
      cacheWritePrice: typeof item.cache_write_price === 'number' ? item.cache_write_price : null,
      cacheReadPrice: typeof item.cache_read_price === 'number' ? item.cache_read_price : null,
    }]
  })
}

function publicPriceRows(modelId: string, catalog?: PublicModelPricingCatalog | null): MatrixPricing[] {
  if (!catalog || !Array.isArray(catalog.groups)) return []
  return catalog.groups.flatMap(group => {
    const price = (group.models ?? []).find(item => item.model === modelId)
    if (!price) return []
    return [publicPrice(group.name, catalog.currency, catalog.unit, price)]
  })
}

function publicPrice(group: string, currency: string, unit: string, price: PublicModelPrice): MatrixPricing {
  return {
    source: 'public',
    group,
    currency,
    unit,
    inputPrice: price.input_price,
    outputPrice: price.output_price,
    cacheWritePrice: price.cache_write_price,
    cacheReadPrice: price.cache_read_price,
  }
}

export function buildModelClientMatrix(
  contracts: readonly ModelDocContract[],
  matrix: ClientMatrixV2Data,
  pricing?: PublicModelPricingCatalog | null,
): ModelClientMatrixRow[] {
  const rows: ModelClientMatrixRow[] = []

  for (const contract of contracts) {
    const publicPrices = publicPriceRows(contract.model.id, pricing)
    const prices = publicPrices.length ? publicPrices : contractPriceRows(contract)
    for (const client of matrix.clients) {
      for (const protocol of PROTOCOLS) {
        const clientProtocol = client.client_protocol.protocols.find(item => item.protocol === protocol)
        const protocolClaim = modelProtocol(contract, protocol)
        const candidate = clientProtocol?.support === 'supported' && protocolClaim?.status === 'verified'
        const documentedOs = client.client_config_os.os_support

        if (!candidate) {
          const reason = clientProtocol?.support !== 'supported'
            ? `${client.name} 不支持 ${protocol}`
            : `${contract.model.display_name} 未发布 ${protocol} 协议`
          rows.push({
            key: `${contract.model.id}:${protocol}:${client.id}`,
            model: contract,
            client,
            protocol,
            candidate: false,
            status: 'unsupported',
            statusReason: reason,
            osResults: documentedOs.map(item => ({
              os: item.os,
              status: 'unsupported',
              evidence: reason,
              configFiles: item.config_files,
            })),
            reasoningLevels: [],
            clientRequestedLevels: requestedReasoningLevels(client),
            clientModes: client.client_reasoning.modes ?? [],
            prices,
          })
          continue
        }

        const coverage = contract.client_coverage.find(item => item.name === client.name && item.protocols.includes(protocol))
        const osResults = documentedOs.map(osConfig => {
          const evidence = clientEvidence(contract, client.name, protocol, osConfig.os)
          const status = cellStatus(evidence) ?? (coverage?.status === 'unsupported' ? 'unsupported' : 'blocked')
          return {
            os: osConfig.os,
            status,
            evidence: evidence?.evidence ?? coverage?.evidence ?? `缺少 ${osConfig.os} 精确版本 Agent 闭环证据`,
            observedAt: evidence?.observed_at ?? evidence?.verified_at,
            clientVersion: evidence?.client_version,
            configFiles: osConfig.config_files,
          }
        })
        const status = rowStatus(osResults)
        rows.push({
          key: `${contract.model.id}:${protocol}:${client.id}`,
          model: contract,
          client,
          protocol,
          candidate: true,
          status,
          statusReason: status === 'verified'
            ? '所有已声明 OS 均具备精确版本证据'
            : '至少一个已声明 OS 缺少精确版本终态证据',
          osResults,
          reasoningLevels: reasoningIntersection(contract, client),
          clientRequestedLevels: requestedReasoningLevels(client),
          clientModes: client.client_reasoning.modes ?? [],
          prices,
        })
      }
    }
  }

  return rows.sort((a, b) => {
    const status = STATUS_ORDER[a.status] - STATUS_ORDER[b.status]
    if (status !== 0) return status
    return a.model.model.display_name.localeCompare(b.model.model.display_name)
      || a.client.name.localeCompare(b.client.name)
      || a.protocol.localeCompare(b.protocol)
  })
}

export function protocolLabel(protocol: MatrixProtocol): string {
  return ({
    responses: 'Responses',
    chat_completions: 'Chat Completions',
    messages: 'Messages',
    generate_content: 'GenerateContent',
  } as const)[protocol]
}

export function matrixFeatureCells(contract: ModelDocContract, protocol: MatrixProtocol): Array<{ name: string; status: MatrixStatus }> {
  const matrix = asRecord(contract.test_matrix)
  const base = asRecord(asRecord(matrix.protocols)[protocol])
  const features = asRecord(asRecord(matrix.protocol_features)[protocol])
  const selected: Array<[string, unknown]> = [
    ['流式终态', base.streaming_terminal],
    ['工具调用', base.tool_call],
    ['工具结果续轮', base.tool_result_continuation],
    ['Usage', base.usage],
    ['Web Search', features.web_search],
    ['Prompt Cache', features.prompt_cache],
    ['图片输入', features.image_input],
    ['结构化输出', features.structured_output],
  ]
  return selected.map(([name, value]) => ({ name, status: cellStatus(value) ?? 'blocked' }))
}
