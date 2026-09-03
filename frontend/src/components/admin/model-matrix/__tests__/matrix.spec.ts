import { describe, expect, it } from 'vitest'
import type { ModelDocContract } from '@/generated/modelDocContracts'
import type { PublicModelPricingCatalog } from '@/api/publicPricing'
import { buildModelClientMatrix, type ClientMatrixV2Data } from '../matrix'

const contract = {
  schema_version: 1,
  model: { id: 'claude-test', display_name: 'Claude Test', family: 'claude', context_window: 200000, max_output_tokens: 32000, input_modalities: ['text', 'image'], output_modalities: ['text'] },
  access: { base_url: 'https://api.example.test', groups: [{ name: 'Claude 标准线路', multiplier: 1 }] },
  protocols: [
    { name: 'messages', status: 'verified', evidence: 'passed' },
    { name: 'responses', status: 'unsupported', evidence: 'not exposed' },
  ],
  recommended_protocol: 'messages',
  clients: [],
  client_coverage: [{ name: 'Claude Code', protocols: ['messages'], status: 'verified', evidence: 'legacy passed' }],
  reasoning: { model_levels: ['low', 'high'], client_mappings: [] },
  test_matrix: {
    clients: {
      'Claude Code': {
        messages: {
          macos: { status: 'verified', evidence: 'exact run', observed_at: '2026-08-31', client_version: 'cli:2.1.251' },
        },
      },
    },
    pricing: {
      'Claude 标准线路': { status: 'verified', currency: 'CNY', unit: 'per_1m_tokens', input_price: 2, output_price: 10, cache_read_price: 0.2 },
    },
  },
  verification: { official_spec_url: 'https://example.test', verified_at: '2026-08-31', limits_source: 'official', gateway_e2e: true, modalities: { text: 'verified', image: 'verified', video: 'unsupported' } },
} as unknown as ModelDocContract

const client = {
  clients: [{
    id: 'claude-code', name: 'Claude Code', slug: 'integration-claude-code', icon: '/claude.svg', one_click_status: 'prototype',
    client_protocol: { protocols: [
      { protocol: 'responses', support: 'unsupported' },
      { protocol: 'chat_completions', support: 'unsupported' },
      { protocol: 'messages', support: 'supported' },
      { protocol: 'generate_content', support: 'unsupported' },
    ] },
    client_reasoning: { control_kind: 'model_defined', level_control: { values: ['low', 'medium', 'high', 'max'] }, modes: [], fallback: { strategy: 'floor' } },
    client_config_os: {
      release: { version_key: 'cli:2.1.251', display: '2.1.251' },
      os_support: [{ os: 'macos', support: 'documented', config_files: [{ path: '~/.claude/settings.json', format: 'json', scope: 'user' }] }],
      endpoint: { base_url_rule: 'append messages', credential_location: 'env token' },
      model_discovery: '/v1/models', model_slots: ['model'], owned_fields: ['model'],
      mutation: { merge_strategy: 'upsert' },
    },
  }],
} as unknown as ClientMatrixV2Data

describe('buildModelClientMatrix', () => {
  it('only marks a model/client protocol intersection as a candidate and intersects reasoning levels', () => {
    const rows = buildModelClientMatrix([contract], client)
    const messages = rows.find(row => row.protocol === 'messages')!
    const responses = rows.find(row => row.protocol === 'responses')!

    expect(messages.candidate).toBe(true)
    expect(messages.status).toBe('verified')
    expect(messages.reasoningLevels).toEqual(['low', 'high'])
    expect(responses.candidate).toBe(false)
    expect(responses.status).toBe('unsupported')
  })

  it('keeps an otherwise compatible row blocked when an exact OS result is missing', () => {
    const withLinux = structuredClone(client)
    withLinux.clients[0]!.client_config_os.os_support.push({ os: 'linux', support: 'documented', config_files: [] })
    const messages = buildModelClientMatrix([contract], withLinux).find(row => row.protocol === 'messages')!

    expect(messages.status).toBe('blocked')
    expect(messages.osResults.map(result => [result.os, result.status])).toEqual([
      ['macos', 'verified'],
      ['linux', 'blocked'],
    ])
  })

  it('prefers public pricing readback over the contract snapshot', () => {
    const pricing: PublicModelPricingCatalog = {
      updated_at: '2026-08-31', currency: 'CNY', unit: 'per_1m_tokens', groups: [{
        group_id: 1, name: 'Claude 标准线路', platform: 'anthropic', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard',
        models: [{ model: 'claude-test', input_price: 3, output_price: 12, cache_write_price: null, cache_read_price: 0.3 }],
      }],
    }
    const messages = buildModelClientMatrix([contract], client, pricing).find(row => row.protocol === 'messages')!

    expect(messages.prices).toEqual([expect.objectContaining({ source: 'public', inputPrice: 3, outputPrice: 12 })])
  })
})
