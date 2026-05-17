<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const consoleLinks = [
  {
    key: 'edgeone',
    icon: 'cloud',
    labelKey: 'admin.ops.externalConsoles.edgeone.title',
    descriptionKey: 'admin.ops.externalConsoles.edgeone.description',
    url: 'https://console.cloud.tencent.com/edgeone'
  },
  {
    key: 'hostinger',
    icon: 'server',
    labelKey: 'admin.ops.externalConsoles.hostinger.title',
    descriptionKey: 'admin.ops.externalConsoles.hostinger.description',
    url: 'https://hpanel.hostinger.com/vps'
  },
  {
    key: 'dokploy',
    icon: 'terminal',
    labelKey: 'admin.ops.externalConsoles.dokploy.title',
    descriptionKey: 'admin.ops.externalConsoles.dokploy.description',
    url: 'http://187.77.143.86:3000'
  }
] as const
</script>

<template>
  <section class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div>
        <h2 class="text-base font-black text-gray-900 dark:text-white">
          {{ t('admin.ops.externalConsoles.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.externalConsoles.description') }}
        </p>
      </div>
      <div class="text-xs font-medium text-gray-400 dark:text-gray-500">
        {{ t('admin.ops.externalConsoles.openInNewTab') }}
      </div>
    </div>

    <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-3">
      <a
        v-for="item in consoleLinks"
        :key="item.key"
        :href="item.url"
        target="_blank"
        rel="noopener noreferrer"
        class="group flex min-h-[88px] items-center justify-between gap-3 rounded-xl border border-gray-100 bg-gray-50 px-4 py-3 transition hover:border-primary-200 hover:bg-primary-50/60 hover:shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:hover:border-primary-800 dark:hover:bg-primary-950/30"
      >
        <span class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white text-primary-600 ring-1 ring-gray-900/5 transition group-hover:bg-primary-600 group-hover:text-white dark:bg-dark-800 dark:text-primary-400 dark:ring-dark-700">
            <Icon :name="item.icon" size="md" :stroke-width="2" />
          </span>
          <span class="min-w-0">
            <span class="block text-sm font-bold text-gray-900 dark:text-white">
              {{ t(item.labelKey) }}
            </span>
            <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">
              {{ t(item.descriptionKey) }}
            </span>
          </span>
        </span>
        <Icon name="externalLink" size="sm" class="shrink-0 text-gray-400 transition group-hover:text-primary-600 dark:group-hover:text-primary-400" :stroke-width="2" />
      </a>
    </div>
  </section>
</template>
