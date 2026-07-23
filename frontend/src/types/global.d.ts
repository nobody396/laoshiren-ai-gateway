import type { PublicSettings } from '@/types'

declare global {
  interface AppLoadState {
    fail: (reason?: string) => void
    markSlow: () => void
    succeed: () => void
  }

  interface Window {
    __APP_CONFIG__?: PublicSettings
    __APP_LOAD_STATE__?: AppLoadState
  }
}

export {}
