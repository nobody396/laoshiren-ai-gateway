import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { modelDocContracts } from '@/generated/modelDocContracts'
import ModelMatrixDetail from '../ModelMatrixDetail.vue'
import { buildModelClientMatrix, type ClientMatrixV2Data } from '../matrix'

describe('ModelMatrixDetail', () => {
  it('does not turn an unpublished maximum output limit into zero', () => {
    const matrix = {
      clients: [{
        id: 'codex', name: 'Codex', slug: 'integration-codex', icon: '/codex.svg', one_click_status: 'prototype',
        client_protocol: { protocols: [
          { protocol: 'responses', support: 'supported' },
          { protocol: 'chat_completions', support: 'unsupported' },
          { protocol: 'messages', support: 'unsupported' },
          { protocol: 'generate_content', support: 'unsupported' },
        ] },
        client_reasoning: { control_kind: 'model_defined', level_control: { values: [] } },
        client_config_os: {
          release: { version_key: 'cli:test', display: 'test' },
          os_support: [{ os: 'macos', support: 'documented', config_files: [] }],
          endpoint: { base_url_rule: '', credential_location: '' }, model_discovery: '', model_slots: [], owned_fields: [], mutation: { merge_strategy: '' },
        },
      }],
    } as unknown as ClientMatrixV2Data
    const rows = buildModelClientMatrix(
      modelDocContracts,
      matrix,
    )
    const row = rows.find(item => item.model.model.id === 'gpt-5.3-codex-spark')
    expect(row).toBeTruthy()

    const wrapper = mount(ModelMatrixDetail, { props: { row: row! } })
    expect(wrapper.text()).toContain('待确认 / 未公开')
    expect(wrapper.text()).not.toContain('最大输出0')
  })

  it('opens the immutable receipt summary and evidence hash from a matrix cell', async () => {
    const matrix = {
      clients: [{
        id: 'codex', name: 'Codex', slug: 'integration-codex', icon: '/codex.svg', one_click_status: 'ready',
        client_protocol: { protocols: [{ protocol: 'responses', support: 'supported' }] },
        client_reasoning: { control_kind: 'model_defined', level_control: { values: [] } },
        client_config_os: {
          release: { version_key: 'cli:test', display: 'test' },
          os_support: [{ os: 'macos', support: 'verified', config_files: [] }],
          endpoint: { base_url_rule: '', credential_location: '' }, model_discovery: '', model_slots: [], owned_fields: [], mutation: { merge_strategy: '' },
          verification_commands: [{ operating_systems: ['macos'], commands: ['codex --version'] }],
        },
      }],
    } as unknown as ClientMatrixV2Data
    const row = buildModelClientMatrix(modelDocContracts, matrix)
      .find(item => item.model.model.id === 'gpt-5.3-codex-spark' && item.protocol === 'responses')!
    row.osResults = [{ ...row.osResults[0], status: 'verified', evidence: 'M8 receipt evidence-1234567890abcdef12345678' }]
    const wrapper = mount(ModelMatrixDetail, {
      props: {
        row,
        evidenceIndex: {
          'evidence-1234567890abcdef12345678': {
            result: 'pass', summary: 'tool loop completed', artifact_uri: 'artifacts/test.json', artifact_sha256: 'a'.repeat(64), observed_at: '2026-09-01T00:00:00Z',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('codex --version')
    await wrapper.get('.receipt-button').trigger('click')
    expect(wrapper.get('[data-testid="receipt-detail"]').text()).toContain('tool loop completed')
    expect(wrapper.get('[data-testid="receipt-detail"]').text()).toContain('a'.repeat(64))
  })
})
