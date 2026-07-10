import { createPlatformAdapter } from './shared'

export const antigravityAccountAdapter = createPlatformAdapter({
  platformKeys: [
    'api_key', 'access_token', 'refresh_token', 'id_token', 'project_id',
    'subscription_type', 'token_type', 'expires_at'
  ],
  requiredSecrets: { apikey: ['api_key'], upstream: ['api_key'] }
})
