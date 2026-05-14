<template>
  <div class="space-y-4">
    <div class="grid gap-3" :class="providers.length > 1 ? 'sm:grid-cols-2' : 'grid-cols-1'">
      <button
        v-for="provider in providers"
        :key="provider.provider"
        type="button"
        :disabled="disabled"
        class="btn btn-secondary w-full justify-center"
        @click="startLogin(provider.provider)"
      >
        <span class="mr-2 inline-flex h-5 w-5 items-center justify-center" aria-hidden="true">
          <svg v-if="provider.provider === 'linuxdo'" viewBox="0 0 16 16" class="h-5 w-5">
            <path
              d="m7.44,0s.09,0,.13,0c.09,0,.19,0,.28,0,.14,0,.29,0,.43,0,.09,0,.18,0,.27,0q.12,0,.25,0t.26.08c.15.03.29.06.44.08,1.97.38,3.78,1.47,4.95,3.11.04.06.09.12.13.18.67.96,1.15,2.11,1.3,3.28q0,.19.09.26c0,.15,0,.29,0,.44,0,.04,0,.09,0,.13,0,.09,0,.19,0,.28,0,.14,0,.29,0,.43,0,.09,0,.18,0,.27,0,.08,0,.17,0,.25q0,.19-.08.26c-.03.15-.06.29-.08.44-.38,1.97-1.47,3.78-3.11,4.95-.06.04-.12.09-.18.13-.96.67-2.11,1.15-3.28,1.3q-.19,0-.26.09c-.15,0-.29,0-.44,0-.04,0-.09,0-.13,0-.09,0-.19,0-.28,0-.14,0-.29,0-.43,0-.09,0-.18,0-.27,0-.08,0-.17,0-.25,0q-.19,0-.26-.08c-.15-.03-.29-.06-.44-.08-1.97-.38-3.78-1.47-4.95-3.11q-.07-.09-.13-.18c-.67-.96-1.15-2.11-1.3-3.28q0-.19-.09-.26c0-.15,0-.29,0-.44,0-.04,0-.09,0-.13,0-.09,0-.19,0-.28,0-.14,0-.29,0-.43,0-.09,0-.18,0-.27,0-.08,0-.17,0-.25q0-.19.08-.26c.03-.15.06-.29.08-.44.38-1.97,1.47-3.78,3.11-4.95.06-.04.12-.09.18-.13C4.42.73,5.57.26,6.74.1,7,.07,7.15,0,7.44,0Z"
              fill="#EFEFEF"
            />
            <path d="m1.27,11.33h13.45c-.94,1.89-2.51,3.21-4.51,3.88-1.99.59-3.96.37-5.8-.57-1.25-.7-2.67-1.9-3.14-3.3Z" fill="#FEB005" />
            <path d="m12.54,1.99c.87.7,1.82,1.59,2.18,2.68H1.27c.87-1.74,2.33-3.13,4.2-3.78,2.44-.79,5-.47,7.07,1.1Z" fill="#1D1D1F" />
          </svg>
          <svg v-else-if="provider.provider === 'google'" viewBox="0 0 24 24" class="h-5 w-5">
            <path fill="#4285F4" d="M21.6 12.23c0-.78-.07-1.53-.2-2.23H12v4.22h5.38a4.6 4.6 0 0 1-2 3.02v2.51h3.23c1.89-1.74 2.99-4.31 2.99-7.52z" />
            <path fill="#34A853" d="M12 22c2.7 0 4.96-.9 6.61-2.25l-3.23-2.51c-.9.6-2.04.95-3.38.95-2.6 0-4.8-1.76-5.58-4.12H3.08v2.59A10 10 0 0 0 12 22z" />
            <path fill="#FBBC05" d="M6.42 14.07A6 6 0 0 1 6.1 12c0-.72.12-1.42.32-2.07V7.34H3.08A10 10 0 0 0 2 12c0 1.61.39 3.14 1.08 4.66l3.34-2.59z" />
            <path fill="#EA4335" d="M12 5.81c1.47 0 2.79.51 3.83 1.5l2.87-2.87C16.96 2.82 14.7 2 12 2a10 10 0 0 0-8.92 5.34l3.34 2.59C7.2 7.57 9.4 5.81 12 5.81z" />
          </svg>
          <svg v-else viewBox="0 0 24 24" class="h-5 w-5 fill-current">
            <path d="M12 .5a12 12 0 0 0-3.79 23.39c.6.11.82-.26.82-.58v-2.03c-3.34.73-4.04-1.61-4.04-1.61-.55-1.39-1.33-1.76-1.33-1.76-1.09-.74.08-.73.08-.73 1.2.09 1.83 1.24 1.83 1.24 1.07 1.83 2.8 1.3 3.49.99.11-.78.42-1.3.76-1.6-2.66-.3-5.46-1.33-5.46-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.54-1.52.12-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.66 1.66.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.8 5.62-5.47 5.92.43.37.81 1.1.81 2.22v3.3c0 .32.22.7.82.58A12 12 0 0 0 12 .5z" />
          </svg>
        </span>
        {{ provider.label }}
      </button>
    </div>

    <div class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.oauth.orContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

interface LoginOAuthProvider {
  provider: 'linuxdo' | 'google' | 'github'
  label: string
}

const props = defineProps<{
  providers: LoginOAuthProvider[]
  disabled?: boolean
  invitationCode?: string
  referralCode?: string
}>()

const route = useRoute()
const { t } = useI18n()

function startLogin(provider: LoginOAuthProvider['provider']): void {
  const redirectTo = (route.query.redirect as string) || '/dashboard'
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const params = new URLSearchParams({ redirect: redirectTo })
  const invitationCode = (props.invitationCode || route.query.invitation_code || '').toString().trim()
  const referralCode = (
    props.referralCode ||
    route.query.referral_code ||
    route.query.aff ||
    route.query.ref ||
    ''
  ).toString().trim()
  if (invitationCode) {
    params.set('invitation_code', invitationCode)
  }
  if (referralCode) {
    params.set('referral_code', referralCode)
  }
  const startURL = `${normalized}/auth/oauth/${provider}/start?${params.toString()}`
  window.location.href = startURL
}
</script>
