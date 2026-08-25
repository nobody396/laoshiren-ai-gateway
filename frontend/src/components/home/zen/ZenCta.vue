<template>
  <section class="zen-cta">
    <div class="zen-cta__inner zen-reveal">
      <small class="zen-cta__greek">γνῶθι σεαυτόν</small>
      <h2 class="zen-cta__title">{{ ui.title }}</h2>
      <a class="zen-cta__btn" :href="ctaTarget" @click="onCtaClick">
        {{ ui.button }} <Icon name="arrowRight" size="sm" />
      </a>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 收尾 CTA:黑色横带 + EB Garamond 斜体希腊文 + 白色药丸按钮
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  /** 是否已认证 */
  isAuthenticated: boolean
  /** 控制台路径 */
  dashboardPath: string
}>()

const $router = useRouter()
const { locale } = useI18n()

const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? { title: 'Make every call simpler.', button: 'Get started' }
  : { title: '让每一次调用，都更简单。', button: '开始使用' }))

const ctaTarget = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))

function onCtaClick(event: MouseEvent): void {
  event.preventDefault()
  void $router.push(ctaTarget.value)
}
</script>

<style scoped>
.zen-cta {
  background: rgb(var(--zen-ink));
  color: rgb(var(--zen-bg));
}

.zen-cta__inner {
  width: min(100% - 4rem, 1200px);
  margin: auto;
  padding: 104px 0;
  text-align: center;
}

.zen-cta__greek {
  display: block;
  font-family: 'EB Garamond', 'Times New Roman', serif;
  font-style: italic;
  font-size: 17px;
  letter-spacing: 0.06em;
  color: rgb(var(--color-dark-200));
}

.zen-cta__title {
  margin: 18px 0 0;
  font-size: clamp(36px, 4vw, 56px);
  font-weight: 800;
  letter-spacing: -0.02em;
}

.zen-cta__btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 44px;
  padding: 0 22px;
  margin-top: 34px;
  border-radius: 11px;
  background: rgb(var(--zen-bg));
  color: rgb(var(--zen-ink));
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}

.zen-cta__btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 20px rgb(var(--lacquer-base) / 0.3);
}

.zen-cta__btn:active {
  transform: scale(0.97);
}

@media (max-width: 900px) {
  .zen-cta__inner {
    width: min(100% - 2.5rem, 1200px);
    padding: 80px 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .zen-cta__btn {
    transition-duration: 1ms;
  }
}
</style>
