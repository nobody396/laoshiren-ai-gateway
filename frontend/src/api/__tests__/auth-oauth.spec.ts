import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { completeOAuthRegistration } from '../auth'

vi.mock('../client', () => ({
  apiClient: {
    post: vi.fn()
  }
}))

const mockClient = apiClient as unknown as {
  post: ReturnType<typeof vi.fn>
}

beforeEach(() => {
  vi.clearAllMocks()
  mockClient.post.mockResolvedValue({
    data: {
      access_token: 'access-token',
      refresh_token: 'refresh-token',
      expires_in: 3600,
      token_type: 'Bearer'
    }
  })
})

describe('OAuth registration API', () => {
  it('sends referral_code when completing OAuth registration', async () => {
    await completeOAuthRegistration('google', 'pending-token', 'INV123', 'REF456')

    expect(mockClient.post).toHaveBeenCalledWith('/auth/oauth/google/complete-registration', {
      pending_oauth_token: 'pending-token',
      invitation_code: 'INV123',
      referral_code: 'REF456'
    })
  })

  it('omits empty optional registration codes', async () => {
    await completeOAuthRegistration('github', 'pending-token')

    expect(mockClient.post).toHaveBeenCalledWith('/auth/oauth/github/complete-registration', {
      pending_oauth_token: 'pending-token'
    })
  })
})
