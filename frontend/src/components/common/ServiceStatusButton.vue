<template>
  <!-- 服务状态快捷入口：点击新标签页打开服务状态页 -->
  <a
    :href="statusUrl"
    target="_blank"
    rel="noopener noreferrer"
    class="relative flex items-center gap-1 rounded-lg px-2 py-1.5 text-xs font-medium text-gray-600 transition-all hover:scale-105 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white"
    :aria-label="t('common.serviceStatus')"
    :title="t('common.serviceStatus')"
  >
    <Icon name="activity" size="sm" />
    <span class="hidden sm:inline">{{ t('common.status') }}</span>
  </a>
</template>

<script setup lang="ts">
/**
 * 顶部栏服务状态入口按钮
 * 点击跳转到外部服务状态页面
 */
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

// 使用站内公开状态页，避免外部占位域名和跨站状态不一致。
const statusUrl = '/status'
</script>
