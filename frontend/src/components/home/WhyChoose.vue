<template>
  <section class="why-choose">
    <div class="why-choose__container mirror-reveal">
      <p class="section-eyebrow">{{ ui.eyebrow }}</p>
      <h2 class="section-title">{{ ui.title }}</h2>
      <p class="section-lede">{{ ui.lede }}</p>

      <div class="virtue-grid">
        <article
          v-for="(card, index) in featureCards"
          :key="card.title"
          class="virtue-card"
        >
          <div class="virtue-card__icon" v-html="icons[index]"></div>
          <p class="virtue-card__number">0{{ index + 1 }}</p>
          <h3>{{ virtueNames[index] }} · {{ card.title }}</h3>
          <p>{{ card.description }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 为什么选择区块
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  featureCards: Array<{ title: string; description: string }>
}>()

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    eyebrow: 'I · Four Virtues',
    title: 'Four Virtues',
    lede: 'Four engineering virtues: prudence, restraint, courage, and justice.'
  }
  : {
    eyebrow: 'I · 四美德',
    title: '四美德',
    lede: '古希腊智者推崇四主德：审慎、节制、勇毅、正义。我们的工程亦如是。'
  }))

const virtueNames = ['Prudentia', 'Temperantia', 'Fortitudo', 'Iustitia']

const icons = [
  '<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"><ellipse cx="24" cy="28" rx="14" ry="6"/><path d="M24 22 V14 M21 18 q3 -3 6 0 q-3 3 -6 0"/><path d="M24 12 Q22 9 24 7 Q26 9 24 12"/></svg>',
  '<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"><rect x="10" y="14" width="28" height="22" rx="1"/><path d="M14 20 H34 M14 26 H30 M14 32 H26"/><circle cx="36" cy="14" r="6"/><path d="M34 14 L36 16 L40 12"/></svg>',
  '<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"><path d="M24 8 L34 14 V26 Q34 36 24 42 Q14 36 14 26 V14 Z"/><path d="M20 24 L23 27 L29 21"/></svg>',
  '<svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"><path d="M24 8 V40"/><path d="M14 14 H34"/><path d="M10 14 Q14 26 18 14 Q14 18 10 14"/><path d="M30 14 Q34 26 38 14 Q34 18 30 14"/></svg>',
]
</script>

<style scoped>
.why-choose {
  padding: 5rem 0 6rem;
  background: rgb(var(--color-papyrus));
}

.why-choose__container {
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
  max-width: 100%;
  margin: 1.125rem auto 3.75rem;
  color: rgb(var(--color-ink));
  font-size: 1.18rem;
  font-style: italic;
  line-height: 1.65;
  text-align: center;
  white-space: nowrap;
}

.virtue-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1.5rem;
}

.virtue-card {
  min-height: 18rem;
  padding: 2.25rem 1.65rem;
  background: rgb(var(--color-stone));
  border: 1px solid rgba(63, 90, 58, 0.16);
  transition: background 0.25s ease, border-color 0.25s ease, transform 0.25s ease;
}

.virtue-card:hover {
  background: rgb(var(--color-marble));
  border-color: rgba(63, 90, 58, 0.34);
  transform: translateY(-2px);
}

.virtue-card__icon {
  width: 3rem;
  height: 3rem;
  margin-bottom: 1.35rem;
  color: rgb(var(--color-terracotta));
}

.virtue-card__icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.virtue-card__number {
  margin: 0 0 0.5rem;
  color: rgb(var(--color-muted));
  font-family: 'Cinzel', serif;
  font-size: 0.85rem;
  letter-spacing: 0.12em;
}

.virtue-card h3 {
  margin: 0 0 0.875rem;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.15rem;
  font-weight: 500;
  line-height: 1.35;
}

.virtue-card p:last-child {
  margin: 0;
  color: rgb(var(--color-ink));
  font-size: 1rem;
  line-height: 1.65;
}

@media (max-width: 1024px) {
  .virtue-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .why-choose__container {
    width: min(100% - 2rem, 1200px);
  }

  .virtue-grid {
    grid-template-columns: 1fr;
  }
}
</style>
