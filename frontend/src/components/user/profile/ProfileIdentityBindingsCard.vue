<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.identity.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
        {{ t('profile.identity.description') }}
      </p>
    </div>

    <div class="px-6 py-6">
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="provider in providerConfigs"
          :key="provider.provider"
          class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700"
        >
          <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div class="space-y-2">
              <div class="flex items-center gap-3">
                <div class="rounded-xl bg-gray-100 px-3 py-2 text-sm font-semibold text-gray-800 dark:bg-dark-700 dark:text-gray-100">
                  {{ provider.label }}
                </div>
                <span
                  :class="[
                    'badge',
                    bindingFor(provider.provider) ? 'badge-green' : 'badge-gray'
                  ]"
                >
                  {{ bindingFor(provider.provider) ? t('profile.identity.connected') : t('profile.identity.notConnected') }}
                </span>
              </div>

              <p class="text-sm text-gray-600 dark:text-gray-300">
                {{ provider.description }}
              </p>

              <div v-if="bindingFor(provider.provider)" class="space-y-1 text-sm text-gray-500 dark:text-dark-400">
                <p>
                  {{ t('profile.identity.accountLabel') }}:
                  <span class="font-medium text-gray-800 dark:text-gray-200">
                    {{
                      bindingFor(provider.provider)?.display_name ||
                      bindingFor(provider.provider)?.email ||
                      bindingFor(provider.provider)?.provider_user_id
                    }}
                  </span>
                </p>
                <p>
                  {{ t('profile.identity.boundAt') }}:
                  <span class="text-gray-700 dark:text-gray-300">
                    {{ formatDateTime(bindingFor(provider.provider)?.bound_at || '') }}
                  </span>
                </p>
                <p>
                  {{ t('profile.identity.lastLoginAt') }}:
                  <span class="text-gray-700 dark:text-gray-300">
                    {{
                      bindingFor(provider.provider)?.last_login_at
                        ? formatDateTime(bindingFor(provider.provider)?.last_login_at || '')
                        : t('common.time.never')
                    }}
                  </span>
                </p>
              </div>
            </div>

            <div class="flex justify-end">
              <button
                v-if="bindingFor(provider.provider)"
                class="btn btn-secondary"
                :disabled="actionLoading"
                @click="confirmProvider = provider.provider"
              >
                {{ actionLoading ? t('common.processing') : t('profile.identity.disconnect') }}
              </button>

              <button
                v-else
                class="btn btn-primary"
                :disabled="actionLoading || !provider.enabled"
                @click="handleProviderBind(provider.provider)"
              >
                {{ actionLoading ? t('profile.identity.connecting') : t('profile.identity.connect') }}
              </button>
            </div>
          </div>

          <p v-if="!provider.enabled" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('profile.identity.providerDisabled', { provider: provider.label }) }}
          </p>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="confirmProvider !== null"
      :title="t('profile.identity.disconnectConfirmTitle')"
      :message="t('profile.identity.disconnectConfirmMessage', { provider: providerLabel(confirmProvider) })"
      danger
      @confirm="handleConfirmUnbind"
      @cancel="confirmProvider = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { userAPI, type UserIdentityBinding } from '@/api/user'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const actionLoading = ref(false)
const linuxDoEnabled = ref(false)
const googleEnabled = ref(false)
const googleProviderName = ref('Google')
const githubEnabled = ref(false)
const bindings = ref<UserIdentityBinding[]>([])
const confirmProvider = ref<string | null>(null)

type IdentityProvider = 'linuxdo' | 'google' | 'github'

const providerConfigs = computed(() => [
  {
    provider: 'linuxdo' as IdentityProvider,
    label: 'Linux.do',
    enabled: linuxDoEnabled.value,
    description: t('profile.identity.linuxdoDescription')
  },
  {
    provider: 'google' as IdentityProvider,
    label: googleProviderName.value || 'Google',
    enabled: googleEnabled.value,
    description: t('profile.identity.googleDescription')
  },
  {
    provider: 'github' as IdentityProvider,
    label: 'GitHub',
    enabled: githubEnabled.value,
    description: t('profile.identity.githubDescription')
  }
])

function bindingFor(provider: string): UserIdentityBinding | null {
  return bindings.value.find((item) => item.provider === provider) || null
}

function providerLabel(provider: string | null): string {
  if (provider === 'linuxdo') return 'Linux.do'
  if (provider === 'google') return googleProviderName.value || 'Google'
  if (provider === 'github') return 'GitHub'
  return provider || ''
}

function extractReason(error: unknown): string {
  const err = error as {
    reason?: string
    response?: { data?: { reason?: string; message?: string } }
    message?: string
  }
  return err.response?.data?.reason || err.reason || ''
}

function extractMessage(error: unknown, fallback: string): string {
  const reason = extractReason(error)
  if (reason === 'AUTH_IDENTITY_ORPHAN_UNBIND') {
    return t('profile.identity.errors.orphanUnbind')
  }
  if (reason === 'AUTH_IDENTITY_CONFLICT') {
    return t('profile.identity.errors.bindingConflict')
  }
  const err = error as {
    response?: { data?: { message?: string } }
    message?: string
  }
  return err.response?.data?.message || err.message || fallback
}

async function loadBindings() {
  loading.value = true
  try {
    const [settings, items] = await Promise.all([
      appStore.fetchPublicSettings(),
      userAPI.listIdentityBindings()
    ])
    linuxDoEnabled.value = settings?.linuxdo_oauth_enabled ?? false
    googleEnabled.value = settings?.oidc_oauth_enabled ?? false
    googleProviderName.value = settings?.oidc_oauth_provider_name || 'Google'
    githubEnabled.value = settings?.github_oauth_enabled ?? false
    bindings.value = items
  } catch (error) {
    appStore.showError(extractMessage(error, t('profile.identity.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function handleProviderBind(provider: IdentityProvider) {
  const config = providerConfigs.value.find((item) => item.provider === provider)
  if (!config?.enabled) return
  actionLoading.value = true
  try {
    const result = await userAPI.startIdentityBind(provider, '/profile')
    window.location.href = result.auth_url
  } catch (error) {
    appStore.showError(extractMessage(error, t('profile.identity.startBindFailed')))
    actionLoading.value = false
  }
}

async function handleConfirmUnbind() {
  if (!confirmProvider.value) return
  const provider = confirmProvider.value
  confirmProvider.value = null
  actionLoading.value = true
  try {
    await userAPI.unbindIdentity(provider)
    appStore.showSuccess(t('profile.identity.disconnectSuccess'))
    await loadBindings()
  } catch (error) {
    appStore.showError(extractMessage(error, t('profile.identity.disconnectFailed')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(() => {
  void loadBindings()
})
</script>
