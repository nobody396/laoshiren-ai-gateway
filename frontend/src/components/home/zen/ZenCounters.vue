<template>
  <div class="zen-counters">
    <article class="zen-counters__item zen-reveal">
      <label class="zen-counters__label">{{ ui.tokensLabel }}</label>
      <ZenFlipNumber :value="tokensMillions" :pad="5" :intro="intro" />
    </article>
    <article class="zen-counters__item zen-reveal" style="--zen-reveal-i: 1">
      <label class="zen-counters__label">{{ ui.compensationLabel }}</label>
      <ZenFlipNumber :value="compensationCny" prefix="¥" :pad="4" :intro="intro" />
    </article>
  </div>
</template>

<script setup lang="ts">
/**
 * 落地页数据计数器(TOKEN 处理量 / 累计赔付金额)
 * 数值由 HomeView 轮询 /public/stats 后下发,本组件只负责展示。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ZenFlipNumber from './ZenFlipNumber.vue'

withDefaults(defineProps<{
  /** token 处理量(百万) */
  tokensMillions: number
  /** 累计赔付金额(元) */
  compensationCny: number
  /** 首次入场:从 0 减速滚动到真值 */
  intro?: boolean
}>(), {
  intro: false
})

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? { tokensLabel: 'TOKENS PROCESSED (MILLIONS)', compensationLabel: 'TOTAL COMPENSATION PAID (CNY)' }
  : { tokensLabel: 'TOKEN 处理量（百万）', compensationLabel: '累计赔付金额（元）' }))
</script>

<style scoped>
.zen-counters {
  width: min(100% - 4rem, 1200px);
  margin: 72px auto 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
}

.zen-counters__label {
  display: block;
  margin-bottom: 14px;
  font-size: 13px;
  font-weight: 500;
  color: rgb(var(--zen-muted-light));
  letter-spacing: 0.02em;
}

@media (max-width: 900px) {
  .zen-counters {
    width: min(100% - 2.5rem, 1200px);
    grid-template-columns: 1fr;
    gap: 36px;
    margin-top: 56px;
  }
}
</style>
