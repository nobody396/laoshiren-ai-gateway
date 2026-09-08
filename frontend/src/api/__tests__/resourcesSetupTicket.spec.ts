import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import {
  createClientSetupTicket,
  createClientSetupTicketForAPIKey,
  createClientSetupTicketForSelection,
  createClientSetupTicketForOption,
  getClientSetupOptions,
  type ClientSetupSelection,
} from '../resources'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), defaults: {} } }))

const mockClient = apiClient as unknown as { get: ReturnType<typeof vi.fn>; post: ReturnType<typeof vi.fn> }

describe('client setup ticket API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockClient.post.mockResolvedValue({ data: { ticket: 'ticket' } })
  })

  it('reads simple options and issues a ticket without exposing version protocol or model fields', async () => {
    mockClient.get.mockResolvedValueOnce({ data: [{ client_id: 'codex', name: 'Codex' }] })
    await getClientSetupOptions(42, 'windows')
    await createClientSetupTicketForOption(42, 'codex', 'windows')
    await createClientSetupTicketForOption(43, 'claude-code', 'macos')

    expect(mockClient.get).toHaveBeenCalledWith('/resources/setup-options', {
      params: { api_key_id: 42, os: 'windows' }
    })
    expect(mockClient.post).toHaveBeenCalledWith('/resources/setup-ticket', {
      api_key_id: 42,
      client_id: 'codex',
      os: 'windows'
    })
    expect(mockClient.post).toHaveBeenCalledWith('/resources/setup-ticket', {
      api_key_id: 43,
      client_id: 'claude-code',
      os: 'macos'
    })
  })

  it('preserves both legacy target and API-key request shapes', async () => {
    await createClientSetupTicket('codex')
    await createClientSetupTicketForAPIKey(42)

    expect(mockClient.post).toHaveBeenNthCalledWith(1, '/resources/setup-ticket', { target: 'codex' })
    expect(mockClient.post).toHaveBeenNthCalledWith(2, '/resources/setup-ticket', { api_key_id: 42 })
  })

  it('sends the exact explicit selection without deriving it from a group platform', async () => {
    const selection: ClientSetupSelection = {
      api_key_id: 42,
      client_id: 'codex',
      client_version_key: 'cli:0.151.0',
      protocol: 'responses',
      model_id: 'gpt-5.6-sol',
      os: 'macos',
    }

    await createClientSetupTicketForSelection(selection)

    expect(mockClient.post).toHaveBeenCalledWith('/resources/setup-ticket', selection)
  })
})
