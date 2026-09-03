import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import {
  createClientSetupTicket,
  createClientSetupTicketForAPIKey,
  createClientSetupTicketForSelection,
  type ClientSetupSelection,
} from '../resources'

vi.mock('../client', () => ({ apiClient: { post: vi.fn(), defaults: {} } }))

const mockClient = apiClient as unknown as { post: ReturnType<typeof vi.fn> }

describe('client setup ticket API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockClient.post.mockResolvedValue({ data: { ticket: 'ticket' } })
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
