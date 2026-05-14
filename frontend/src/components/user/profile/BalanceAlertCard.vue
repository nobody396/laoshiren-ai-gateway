<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.balanceAlert.title') }}
      </h2>
    </div>
    <div class="px-6 py-6">
      <div v-if="loadingConfig" class="flex items-center justify-center py-8">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <form v-else class="space-y-5" @submit.prevent="handleSave">
        <div class="flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('profile.balanceAlert.enabled') }}</label>
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('profile.balanceAlert.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.enabled" />
        </div>

        <div v-if="form.enabled" class="space-y-4">
          <div>
            <label for="alert-threshold" class="input-label">
              {{ t('profile.balanceAlert.threshold') }}
            </label>
            <input
              id="alert-threshold"
              v-model.number="form.threshold"
              type="number"
              step="0.01"
              min="0.10"
              class="input"
              :placeholder="effectiveThreshold ? `${t('profile.balanceAlert.default')}: $${effectiveThreshold}` : '$5.00'"
            />
            <p class="input-hint">{{ t('profile.balanceAlert.thresholdHint') }}</p>
          </div>

          <div>
            <label for="alert-email" class="input-label">
              {{ t('profile.balanceAlert.email') }}
            </label>
            <input
              id="alert-email"
              v-model="form.email"
              type="email"
              class="input"
              :placeholder="effectiveEmail || t('profile.balanceAlert.emailPlaceholder')"
            />
            <p class="input-hint">{{ t('profile.balanceAlert.emailHint') }}</p>
          </div>
        </div>

        <div class="flex justify-end pt-2">
          <button type="submit" :disabled="saving" class="btn btn-primary">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { balanceAlertAPI, buildUpdateBalanceAlertRequest } from '@/api/balanceAlert'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loadingConfig = ref(true)
const saving = ref(false)
const effectiveThreshold = ref('')
const effectiveEmail = ref('')

const form = ref({
  enabled: true,
  threshold: null as number | null,
  email: ''
})

onMounted(async () => {
  try {
    const cfg = await balanceAlertAPI.getConfig()
    form.value.enabled = cfg.enabled
    form.value.threshold = cfg.threshold ? parseFloat(cfg.threshold) : null
    form.value.email = cfg.email
    effectiveThreshold.value = cfg.effective_threshold
    effectiveEmail.value = cfg.effective_email
  } catch {
    // silently fall back to defaults
  } finally {
    loadingConfig.value = false
  }
})

const handleSave = async () => {
  saving.value = true
  try {
    await balanceAlertAPI.updateConfig(buildUpdateBalanceAlertRequest(form.value))
    appStore.showSuccess(t('profile.balanceAlert.saveSuccess'))
  } catch (error: unknown) {
    const msg = error instanceof Error ? error.message : t('profile.balanceAlert.saveFailed')
    appStore.showError(msg)
  } finally {
    saving.value = false
  }
}
</script>
