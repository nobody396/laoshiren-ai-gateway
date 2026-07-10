import { createPlatformAdapter } from './shared'

export const geminiAccountAdapter = createPlatformAdapter({
  platformKeys: [
    'api_key', 'access_token', 'refresh_token', 'id_token', 'project_id',
    'oauth_type', 'tier_id', 'token_type', 'scope', 'expires_at'
  ],
  requiredSecrets: { apikey: ['api_key'], upstream: ['api_key'] }
})
