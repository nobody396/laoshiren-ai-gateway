<template>
  <section id="model-pricing" class="model-pricing">
    <div class="model-pricing__container mirror-reveal">
      <div class="greco-divider" aria-hidden="true"></div>
      <div class="pricing-heading">
        <p class="section-eyebrow">{{ ui.eyebrow }}</p>
        <h2 class="section-title">{{ ui.title }}</h2>
        <p class="section-lede">{{ ui.lede }}</p>
        <RouterLink to="/models" class="pricing-heading__action">{{ ui.viewFullPrice }}</RouterLink>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')

const ui = computed(() => (isEnglish.value
  ? {
    eyebrow: 'IV · Pricing Inscription',
    title: 'Model Pricing',
    lede: 'Per-group prices for every model, computed live from official prices.',
    viewFullPrice: 'View full pricing'
  }
  : {
    eyebrow: 'IV · 价格铭文',
    title: '模型定价',
    lede: '按分组展示每个模型的价格,随官方价与分组倍率实时计算。',
    viewFullPrice: '查看模型价格'
  }))
</script>

<style scoped>
.model-pricing {
  padding: 6rem 0 5.5rem;
  background: rgb(var(--color-papyrus));
  scroll-margin-top: 88px;
}

.model-pricing__container {
  width: min(100% - 4rem, 1320px);
  margin: 0 auto;
}

.greco-divider {
  width: 15rem;
  height: 1.125rem;
  margin: 0 auto 3.75rem;
  opacity: 0.42;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 40 18'><path d='M0,9 L8,9 L8,3 L16,3 L16,15 L24,15 L24,3 L32,3 L32,9 L40,9' stroke='%233F5A3A' fill='none' stroke-width='1.5'/></svg>");
  background-repeat: repeat-x;
  background-size: 40px 18px;
}

.pricing-heading {
  text-align: center;
}

.section-eyebrow {
  margin: 0 0 0.875rem;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.section-title {
  margin: 0;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2.5rem, 4.4vw, 3.75rem);
  font-weight: 500;
  line-height: 1.1;
}

.section-lede {
  margin: 1rem auto 0;
  color: rgb(var(--color-ink));
  font-size: 1.18rem;
  font-style: italic;
  line-height: 1.65;
}

.pricing-heading__action {
  display: inline-block;
  margin-top: 1.75rem;
  padding: 0.75rem 1.75rem;
  border: 1.5px solid rgb(var(--color-ink));
  border-radius: 999px;
  color: rgb(var(--color-ink));
  font-family: 'Inter', sans-serif;
  font-size: 0.875rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-decoration: none;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.pricing-heading__action:hover {
  background: rgb(var(--color-ink));
  color: rgb(var(--color-papyrus));
  transform: translateY(-1px);
}

@media (max-width: 768px) {
  .model-pricing__container {
    width: min(100% - 2rem, 1320px);
  }
}
</style>
