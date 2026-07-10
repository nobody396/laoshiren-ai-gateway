import { createPlatformAdapter } from './shared'

export const openAIAccountAdapter = createPlatformAdapter({
  platformKeys: [
    'api_key', 'access_token', 'refresh_token', 'id_token', 'client_id',
    'account_id', 'organization_id'
  ],
  requiredSecrets: { apikey: ['api_key'], upstream: ['api_key'] }
})
