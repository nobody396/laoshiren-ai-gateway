<template>
  <section class="claude-combinations">
    <div class="claude-combinations__container mirror-reveal">
      <p class="section-eyebrow">{{ ui.eyebrow }}</p>
      <h2 class="section-title">{{ ui.title }}</h2>
      <p class="section-lede">{{ ui.lede }}</p>

      <div class="pillars-wrap">
        <div class="pillars-architrave" aria-hidden="true"></div>
        <div class="pillars">
          <article v-for="model in models" :key="model.name" class="pillar">
            <div class="pillar-capital" aria-hidden="true"></div>
            <p class="pillar-name">{{ model.name }}</p>
            <p class="pillar-role">{{ model.eyebrow }} · {{ model.subtitle }}</p>
            <p class="pillar-desc">{{ model.description }}</p>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 模型矩阵区块
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    eyebrow: 'II · Three Pillars',
    title: 'Three Model Pillars',
    lede: 'Three model families, each in its place, holding one roof.'
  }
  : {
    eyebrow: 'II · 三柱',
    title: '三大模型支柱',
    lede: '三大模型，各司其位，如帕特农神庙之三柱，共承一檐。'
  }))

defineProps<{
  models: Array<{
    eyebrow: string
    name: string
    subtitle: string
    description: string
  }>
}>()
</script>

<style scoped>
.claude-combinations {
  padding: 6rem 0;
  background: rgb(var(--color-marble));
}

.claude-combinations__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
}

.section-eyebrow {
  margin: 0 0 0.875rem;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-align: center;
  text-transform: uppercase;
}

.section-title {
  margin: 0;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2.2rem, 4vw, 3rem);
  font-weight: 500;
  line-height: 1.15;
  text-align: center;
}

.section-lede {
  max-width: 58ch;
  margin: 1.125rem auto 4rem;
  color: rgb(var(--color-ink));
  font-size: 1.18rem;
  font-style: italic;
  line-height: 1.65;
  text-align: center;
}

.pillars-wrap {
  position: relative;
  padding: 0 1.25rem;
}

.pillars-architrave {
  height: 0.5rem;
  margin: 0 -1.25rem;
  background: rgb(var(--color-ink));
  position: relative;
}

.pillars-architrave::before,
.pillars-architrave::after {
  content: '';
  position: absolute;
  top: -0.25rem;
  width: 2.5rem;
  height: 1rem;
  background: rgb(var(--color-ink));
}

.pillars-architrave::before {
  left: 0;
}

.pillars-architrave::after {
  right: 0;
}

.pillars {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border: 1px solid rgba(63, 90, 58, 0.16);
  border-top: 0;
}

.pillar {
  min-height: 25rem;
  padding: 3rem 2rem 3.25rem;
  text-align: center;
  background: rgb(var(--color-marble));
  border-right: 1px solid rgba(63, 90, 58, 0.18);
}

.pillar:nth-child(even) {
  background: rgb(var(--color-stone));
}

.pillar:last-child {
  border-right: 0;
}

.pillar-capital {
  width: 4rem;
  height: 2rem;
  margin: 0 auto 1.75rem;
  position: relative;
}

.pillar-capital::before,
.pillar-capital::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  background: rgb(var(--color-ink));
}

.pillar-capital::before {
  top: 0;
  height: 0.25rem;
}

.pillar-capital::after {
  top: 0.875rem;
  left: 0.375rem;
  right: 0.375rem;
  height: 0.125rem;
}

.pillar-name {
  margin: 0 0 0.5rem;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', serif;
  font-size: 2rem;
  font-weight: 600;
  line-height: 1.2;
}

.pillar-role {
  margin: 0 0 1.125rem;
  color: rgb(var(--color-terracotta));
  font-size: 1rem;
  font-style: italic;
}

.pillar-desc {
  max-width: 22rem;
  margin: 0 auto;
  color: rgb(var(--color-ink));
  font-size: 1rem;
  line-height: 1.65;
}

@media (max-width: 900px) {
  .claude-combinations__container {
    width: min(100% - 2rem, 1200px);
  }

  .pillars {
    grid-template-columns: 1fr;
  }

  .pillar {
    min-height: auto;
    border-right: 0;
    border-bottom: 1px solid rgba(63, 90, 58, 0.18);
  }

  .pillar:last-child {
    border-bottom: 0;
  }
}
</style>
