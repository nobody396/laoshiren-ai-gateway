<template>
  <router-link
    to="/changelog"
    data-testid="changelog-header-link"
    class="relative flex shrink-0 items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-gray-600 transition-all hover:scale-105 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white"
    :class="{ 'text-blue-600 dark:text-blue-400': hasNewChangelog }"
    :aria-label="hasNewChangelog ? t('changelog.newUpdate') : t('nav.changelog')"
    :title="hasNewChangelog ? t('changelog.newUpdate') : t('nav.changelog')"
  >
    <span class="relative">
      <Icon name="sparkles" size="sm" />
      <span
        v-if="hasNewChangelog"
        data-testid="changelog-new-dot"
        class="motion-safe:animate-pulse absolute -right-1 -top-1 h-2 w-2 rounded-full bg-red-500 shadow-sm ring-2 ring-white dark:ring-dark-900"
        aria-hidden="true"
      ></span>
    </span>
    <span data-testid="changelog-header-label" class="hidden sm:inline">{{
      t('nav.changelog')
    }}</span>
  </router-link>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChangelogFreshness } from '@/composables/useChangelogFreshness'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const { hasNewChangelog, refreshChangelogFreshness } = useChangelogFreshness()

onMounted(() => {
  void refreshChangelogFreshness()
})
</script>
