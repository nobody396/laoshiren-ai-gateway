<template>
  <!-- 顶部公告条:自动赔付上线,可关闭 -->
  <div v-if="visible" class="zen-banner">
    <Icon name="sparkles" size="sm" class="zen-banner__sparkle" />
    <span class="zen-banner__text">{{ ui.text }}</span>
    <a href="#compensation" class="zen-banner__link" @click="onDetailsClick">{{ ui.details }}</a>
    <button class="zen-banner__close" :aria-label="ui.close" @click="visible = false">
      <Icon name="x" size="sm" />
    </button>
  </div>
</template>

<script setup lang="ts">
/**
 * 落地页顶部公告条(自动赔付上线,可 dismiss)
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { scrollToHash } from './scrollToHash'

const { locale } = useI18n()

const visible = ref(true)
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    text: 'Automatic compensation is live — upstream failure, no ticket needed, credit lands automatically',
    details: 'Learn more',
    close: 'Dismiss announcement'
  }
  : {
    text: '自动赔付已上线 — 调用异常，无需申请，赔付自动到账',
    details: '了解详情',
    close: '关闭公告'
  }))

function onDetailsClick(event: MouseEvent): void {
  event.preventDefault()
  scrollToHash('#compensation')
}
</script>

<style scoped>
.zen-banner {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 9px 40px 9px 16px;
  background: rgb(var(--zen-bg-soft));
  border-bottom: 1px solid rgb(var(--zen-line));
  font-size: 13px;
  color: rgb(var(--zen-text));
}

.zen-banner__sparkle {
  color: rgb(var(--zen-violet));
  flex: none;
}

.zen-banner__text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.zen-banner__link {
  font-weight: 600;
  color: rgb(var(--zen-ink));
  border-bottom: 1px solid rgb(var(--zen-ink));
  line-height: 1.1;
  white-space: nowrap;
  text-decoration: none;
}

.zen-banner__close {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 6px;
  background: none;
  color: rgb(var(--zen-muted-light));
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease;
}

.zen-banner__close:hover {
  background: rgb(var(--zen-line));
  color: rgb(var(--zen-ink));
}
</style>
