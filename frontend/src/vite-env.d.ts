/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_SITE_URL?: string
  readonly VITE_GA4_DISABLED?: string
  readonly VITE_GA4_FORCE_ENABLE?: string
  readonly VITE_GA4_MEASUREMENT_ID?: string
  readonly BASE_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
