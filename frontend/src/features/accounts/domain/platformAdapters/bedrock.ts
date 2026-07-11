import { createPlatformAdapter } from './shared'

export const bedrockAccountAdapter = createPlatformAdapter({
  platformKeys: [
    'auth_mode', 'aws_region', 'aws_force_global', 'aws_access_key_id',
    'aws_secret_access_key', 'aws_session_token', 'api_key'
  ],
  requiredSecrets: {},
  validate(draft, mode) {
    if (mode !== 'create') return []
    if (draft.credentials.auth_mode === 'api_key') {
      return draft.credentials.api_key ? [] : [{ field: 'credentials.api_key', code: 'required' }]
    }
    return ['aws_access_key_id', 'aws_secret_access_key']
      .filter((key) => !draft.credentials[key])
      .map((key) => ({ field: `credentials.${key}`, code: 'required' }))
  }
})
