<template>
  <section class="vip-tiers">
    <div class="vip-tiers__container mirror-reveal">
      <div class="seal-row">
        <img src="/laoshirenai-icon.jpg" :alt="ui.brandAlt" />
      </div>
      <h3 class="trusted-title">{{ ui.title }}</h3>
      <div class="stats-list">
        <template v-for="(stat, index) in ui.stats" :key="stat.label">
          <div v-if="index > 0" class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-num">{{ stat.value }}</span>
            <span class="stat-label">{{ stat.label }}</span>
          </div>
        </template>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 数据指标区块
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')
const ui = computed(() => (isEnglish.value
  ? {
    brandAlt: 'LaoshirenAI',
    title: 'Widely used by developers',
    stats: [
      { value: '2000+', label: 'Developers served' },
      { value: '1M+', label: 'Total calls' },
      { value: '6.3s', label: 'Average latency' }
    ]
  }
  : {
    brandAlt: '老实人AI',
    title: '已被开发者广泛使用',
    stats: [
      { value: '2000+', label: '服务开发者' },
      { value: '100万+', label: '累计调用次数' },
      { value: '6.3s', label: '平均耗时' }
    ]
  }))
</script>

<style scoped>
.vip-tiers {
  padding: 5rem 0 6rem;
  background:
    linear-gradient(rgb(var(--color-ink) / 0.02), rgb(var(--color-ink) / 0.02)),
    rgb(var(--color-marble));
}

.vip-tiers__container {
  width: min(100% - 4rem, 1200px);
  margin: 0 auto;
  text-align: center;
}

.seal-row {
  display: flex;
  justify-content: center;
  margin-bottom: 1.5rem;
}

.seal-row img {
  width: 4rem;
  height: 4rem;
  object-fit: cover;
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px rgb(var(--color-marble) / 0.42), 0 0 0 1px rgb(var(--color-terracotta-dark));
}

.trusted-title {
  margin: 0 0 3.5rem;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: clamp(2rem, 3.4vw, 2.75rem);
  font-weight: 500;
  line-height: 1.2;
}

.stats-list {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 3.75rem;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.stat-num {
  color: rgb(var(--color-terracotta));
  font-family: 'Cinzel', serif;
  font-size: clamp(2.5rem, 4vw, 3.2rem);
  font-weight: 600;
  line-height: 1;
}

.stat-label {
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.stat-divider {
  width: 1px;
  height: 3.75rem;
  background: rgb(var(--color-laurel) / 0.18);
}

@media (max-width: 760px) {
  .vip-tiers__container {
    width: min(100% - 2rem, 1200px);
  }

  .stats-list {
    flex-direction: column;
    gap: 2rem;
  }

  .stat-divider {
    width: 6rem;
    height: 1px;
  }
}
</style>
