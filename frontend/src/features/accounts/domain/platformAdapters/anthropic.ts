import { createPlatformAdapter } from './shared'

export const anthropicAccountAdapter = createPlatformAdapter({
  platformKeys: [
    'api_key', 'access_token', 'refresh_token', 'setup_token', 'subscription_type',
    'auth_mode', 'aws_region', 'aws_force_global', 'aws_access_key_id',
    'aws_secret_access_key', 'aws_session_token'
  ],
  requiredSecrets: {
    apikey: ['api_key'],
    upstream: ['api_key'],
    'setup-token': ['setup_token']
  },
  validate(draft, mode) {
    if (mode !== 'create' || draft.type !== 'bedrock') return []
    if (draft.credentials.auth_mode === 'api_key') {
      return draft.credentials.api_key ? [] : [{ field: 'credentials.api_key', code: 'required' }]
    }
    return ['aws_access_key_id', 'aws_secret_access_key']
      .filter((key) => !draft.credentials[key])
      .map((key) => ({ field: `credentials.${key}`, code: 'required' }))
  }
})
