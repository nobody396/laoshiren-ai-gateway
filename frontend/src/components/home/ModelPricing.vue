<template>
  <section id="model-pricing" class="model-pricing">
    <div class="model-pricing__container mirror-reveal">
      <div class="greco-divider" aria-hidden="true"></div>
      <div class="pricing-heading">
        <p class="section-eyebrow">{{ ui.eyebrow }}</p>
        <h2 class="section-title">{{ ui.title }}</h2>
        <p class="section-lede">{{ ui.lede }}</p>
      </div>

      <div class="discount-ledger">
        <span><strong>DeepSeek</strong> {{ ui.deepseekLedger }}</span>
        <span><strong>{{ ui.sixFoldProviders }}</strong> {{ ui.sixFoldLedger }}</span>
        <span><strong>Anthropic Max</strong> {{ maxLedgerLabel }} · {{ ui.approx }} {{ maxDiscount }}</span>
        <span><strong>OpenAI Pro</strong> {{ proLedgerLabel }} · {{ ui.approx }} {{ proDiscount }}</span>
      </div>

      <div class="pricing-provider-grid" :aria-label="ui.providerGridAria">
        <article class="provider-card provider-card--gpt" :aria-label="ui.openaiAria">
          <header class="provider-card__header">
            <div class="provider-card__brand">
              <strong>OpenAI</strong>
              <span>{{ ui.openaiAlias }}</span>
            </div>
            <p>{{ ui.proGroup }} · {{ proDiscount }}</p>
          </header>

          <div class="provider-card__body">
            <div v-for="row in gptRows" :key="row.model" class="compact-price-row">
              <div class="compact-model">
                <span>
                  <strong>{{ row.model }}</strong>
                  <em>{{ row.modelId }}</em>
                </span>
                <b>Pro {{ row.discount }}</b>
              </div>
              <div class="compact-rates compact-rates--three">
                <span><strong>{{ ui.input }}</strong><em>{{ row.pro.input }}</em><small>{{ ui.official }} {{ row.official.input }}</small></span>
                <span><strong>{{ ui.cached }}</strong><em>{{ row.pro.cachedInput }}</em><small>{{ ui.official }} {{ row.official.cachedInput }}</small></span>
                <span><strong>{{ ui.output }}</strong><em>{{ row.pro.output }}</em><small>{{ ui.official }} {{ row.official.output }}</small></span>
              </div>
            </div>
          </div>
        </article>

        <article class="provider-card provider-card--claude" :aria-label="ui.anthropicAria">
          <header class="provider-card__header">
            <div class="provider-card__brand">
              <strong>Anthropic</strong>
              <span>{{ ui.anthropicAlias }}</span>
            </div>
            <p>{{ ui.maxGroup }} · {{ maxDiscount }}</p>
          </header>

          <div class="provider-card__body">
            <div v-for="row in claudeRows" :key="row.model" class="compact-price-row">
              <div class="compact-model">
                <span>
                  <strong>{{ row.model }}</strong>
                  <em>{{ row.modelId }}</em>
                </span>
                <b>{{ row.discount }}</b>
              </div>
              <div class="compact-rates">
                <span><strong>{{ ui.input }}</strong><em>{{ row.max.input }}</em><small>{{ ui.official }} {{ row.official.input }}</small></span>
                <span><strong>{{ ui.cacheWrite5m }}</strong><em>{{ row.max.cacheWrite5m }}</em><small>{{ ui.official }} {{ row.official.cacheWrite5m }}</small></span>
                <span><strong>{{ ui.read }}</strong><em>{{ row.max.cacheRead }}</em><small>{{ ui.official }} {{ row.official.cacheRead }}</small></span>
                <span><strong>{{ ui.output }}</strong><em>{{ row.max.output }}</em><small>{{ ui.official }} {{ row.official.output }}</small></span>
              </div>
            </div>
          </div>
        </article>

        <article
          v-for="group in specialProviderGroups"
          :key="group.key"
          class="provider-card"
          :class="`provider-card--${group.key}`"
          :aria-label="`${group.name} ${ui.priceAriaSuffix}`"
        >
          <header class="provider-card__header">
            <div class="provider-card__brand">
              <strong>{{ group.name }}</strong>
              <span>{{ group.alias }}</span>
            </div>
            <p>{{ group.subtitle }}</p>
          </header>

          <div class="provider-card__body provider-card__body--short">
            <div v-for="row in group.rows" :key="row.modelId" class="compact-price-row">
              <div class="compact-model">
                <span>
                  <strong>{{ row.model }}</strong>
                  <em>{{ row.modelId }}</em>
                </span>
                <b>{{ row.discount }}</b>
              </div>
              <div class="compact-rates">
                <span><strong>{{ ui.input }}</strong><em>{{ row.price.input }}</em><small>{{ ui.official }} {{ row.official.input }}</small></span>
                <span><strong>{{ ui.output }}</strong><em>{{ row.price.output }}</em><small>{{ ui.official }} {{ row.official.output }}</small></span>
                <span><strong>{{ ui.cacheRead }}</strong><em>{{ row.price.cacheRead }}</em><small>{{ ui.official }} {{ row.official.cacheRead }}</small></span>
                <span><strong>{{ ui.cacheCreate }}</strong><em>{{ row.price.cacheCreate }}</em><small>{{ ui.official }} {{ row.official.cacheCreate }}</small></span>
              </div>
            </div>
          </div>
        </article>
      </div>

      <div class="model-pricing__cta">
        <router-link
          :to="isAuthenticated ? '/dashboard' : '/login'"
          class="cta-btn"
        >
          {{ ui.cta }}
        </router-link>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 模型定价表格
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

type ClaudePriceSet = {
  input: string
  cacheWrite5m: string
  cacheRead: string
  output: string
}

type ClaudePricingRow = {
  model: string
  modelId: string
  official: ClaudePriceSet
  max: ClaudePriceSet
  discount: string
}

type GptPriceSet = {
  input: string
  cachedInput: string
  output: string
}

type GptPricingRow = {
  model: string
  modelId: string
  official: GptPriceSet
  pro: GptPriceSet
  discount: string
}

type SpecialPriceSet = {
  input: string
  output: string
  cacheRead: string
  cacheCreate: string
}

type SpecialPricingRow = {
  model: string
  modelId: string
  provider: string
  providerKey: 'deepseek' | 'qwen' | 'glm' | 'minimax'
  official: SpecialPriceSet
  price: SpecialPriceSet
  discount: string
}

const props = defineProps<{
  claudeRows: ClaudePricingRow[]
  gptRows: GptPricingRow[]
  specialRows: SpecialPricingRow[]
  maxLedgerLabel: string
  proLedgerLabel: string
  maxDiscount: string
  proDiscount: string
  isAuthenticated: boolean
}>()

const { locale } = useI18n()
const isEnglish = computed(() => locale.value === 'en')

const ui = computed(() => (isEnglish.value
  ? {
    eyebrow: 'IV · Pricing Inscription',
    title: 'Model Pricing',
    lede: 'Displayed per 1M tokens.',
    deepseekLedger: '30% of official price',
    sixFoldProviders: 'Alibaba Cloud / Zhipu AI / MiniMax',
    sixFoldLedger: '60% of official price',
    approx: 'approx.',
    providerGridAria: 'Model prices grouped by provider',
    openaiAria: 'OpenAI model pricing',
    anthropicAria: 'Anthropic model pricing',
    openaiAlias: 'GPT models',
    anthropicAlias: 'Claude models',
    proGroup: 'Pro group',
    maxGroup: 'Max group',
    priceAriaSuffix: 'pricing',
    official: 'Official',
    input: 'Input',
    output: 'Output',
    cached: 'Cached',
    cacheWrite5m: '5m write',
    read: 'Read',
    cacheRead: 'Cache read',
    cacheCreate: 'Cache create',
    cta: 'Sign up free · Start now'
  }
  : {
    eyebrow: 'IV · 价格铭文',
    title: '模型定价',
    lede: '统一按每 100 万 tokens 展示。',
    deepseekLedger: '官方价 3 折',
    sixFoldProviders: '阿里云 / 智谱 AI / MiniMax',
    sixFoldLedger: '官方价 6 折',
    approx: '约',
    providerGridAria: '按供应商分组的模型价格',
    openaiAria: 'OpenAI 模型价格',
    anthropicAria: 'Anthropic 模型价格',
    openaiAlias: 'GPT 模型',
    anthropicAlias: 'Claude 模型',
    proGroup: 'Pro 分组',
    maxGroup: 'Max 分组',
    priceAriaSuffix: '价格',
    official: '官方',
    input: '输入',
    output: '输出',
    cached: '缓存',
    cacheWrite5m: '5分钟写入',
    read: '读取',
    cacheRead: '缓存读取',
    cacheCreate: '缓存创建',
    cta: '免费注册 · 立即体验'
  }))

const specialProviderGroups = computed(() => [
  {
    key: 'deepseek',
    name: 'DeepSeek',
    alias: isEnglish.value ? 'DeepSeek models' : '深度求索',
    subtitle: isEnglish.value ? '30% of official' : '官方价 3 折',
    rows: props.specialRows.filter((row) => row.providerKey === 'deepseek')
  },
  {
    key: 'qwen',
    name: isEnglish.value ? 'Alibaba Cloud' : '阿里云',
    alias: isEnglish.value ? 'Qwen models' : '通义千问',
    subtitle: isEnglish.value ? '60% of official' : '官方价 6 折',
    rows: props.specialRows.filter((row) => row.providerKey === 'qwen')
  },
  {
    key: 'glm',
    name: isEnglish.value ? 'Zhipu AI' : '智谱 AI',
    alias: isEnglish.value ? 'GLM models' : 'GLM 模型',
    subtitle: isEnglish.value ? '60% of official' : '官方价 6 折',
    rows: props.specialRows.filter((row) => row.providerKey === 'glm')
  },
  {
    key: 'minimax',
    name: 'MiniMax',
    alias: 'MiniMax M3',
    subtitle: isEnglish.value ? '60% of official' : '官方价 6 折',
    rows: props.specialRows.filter((row) => row.providerKey === 'minimax')
  }
])
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

.discount-ledger {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin: 2rem auto 4rem;
}

.discount-ledger span {
  display: inline-flex;
  align-items: center;
  min-height: 2.25rem;
  padding: 0.45rem 0.9rem;
  border: 1px solid rgba(63, 90, 58, 0.22);
  background: rgba(250, 246, 236, 0.72);
  color: rgb(var(--color-ink));
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  letter-spacing: 0.02em;
}

.discount-ledger strong {
  margin-right: 0.4rem;
  color: rgb(var(--color-terracotta));
}

.pricing-provider-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: start;
  gap: 4.25rem 2.25rem;
  margin-top: 4.25rem;
}

.provider-card {
  position: relative;
  min-height: 20rem;
  padding: 1rem;
  background: rgb(var(--color-marble));
  border: 1px solid rgba(154, 59, 31, 0.34);
  box-shadow: 0 1rem 2.6rem rgba(63, 90, 58, 0.08), inset 0 0 0 1px rgba(250, 246, 236, 0.7);
}

.provider-card--claude {
  min-height: 24rem;
}

.provider-card--qwen,
.provider-card--glm,
.provider-card--minimax {
  min-height: 23rem;
}

.provider-card__header {
  display: flex;
  align-items: center;
  flex-direction: column;
  justify-content: center;
  min-height: 2.55rem;
  padding: 0.65rem 0.75rem 0.95rem;
  border-bottom: 1px solid rgba(63, 90, 58, 0.12);
  text-align: center;
}

.provider-card__brand {
  position: absolute;
  top: -2.55rem;
  left: 50%;
  display: grid;
  justify-items: center;
  gap: 0.18rem;
  min-width: max-content;
  color: #c5482a;
  line-height: 1;
  transform: translateX(-50%);
}

.provider-card__brand strong {
  font-family: 'Inter', sans-serif;
  font-size: 0.95rem;
  font-weight: 900;
  letter-spacing: 0.04em;
  line-height: 1;
  white-space: nowrap;
}

.provider-card__brand span {
  color: rgb(var(--color-muted));
  font-family: 'Inter', 'Noto Sans SC', sans-serif;
  font-size: 0.56rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  line-height: 1.1;
  white-space: nowrap;
}

.provider-card__header p {
  margin: 0;
  color: #6f634f;
  font-family: 'Inter', sans-serif;
  font-size: 0.76rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  line-height: 1.35;
  text-transform: uppercase;
}

.provider-card__body {
  display: grid;
  gap: 0.75rem;
  max-height: 32rem;
  overflow-y: auto;
  padding: 0.95rem 0.1rem 0.1rem;
  scrollbar-color: rgba(154, 59, 31, 0.42) transparent;
  scrollbar-width: thin;
}

.provider-card__body--short {
  max-height: none;
}

.compact-price-row {
  display: grid;
  gap: 0.72rem;
  padding: 0.82rem;
  background: rgb(var(--color-papyrus));
  border: 1px solid rgba(63, 90, 58, 0.14);
}

.compact-model {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.compact-model span {
  min-width: 0;
}

.compact-model strong {
  display: block;
  color: rgb(var(--color-ink-deep));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  font-size: 1.02rem;
  font-weight: 700;
  line-height: 1.18;
}

.compact-model em {
  display: block;
  margin-top: 0.2rem;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.62rem;
  font-style: normal;
  font-weight: 700;
  letter-spacing: 0.01em;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.compact-model b {
  flex: 0 0 auto;
  padding: 0.22rem 0.48rem;
  background: rgb(var(--color-terracotta));
  color: rgb(var(--color-marble));
  font-family: 'Inter', sans-serif;
  font-size: 0.58rem;
  font-weight: 800;
  letter-spacing: 0.09em;
  line-height: 1.15;
  text-transform: uppercase;
}

.compact-rates {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.46rem;
}

.compact-rates--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.compact-rates span {
  display: grid;
  align-content: start;
  gap: 0.18rem;
  min-height: 4.35rem;
  padding: 0.52rem 0.48rem;
  background: rgba(250, 246, 236, 0.88);
  border: 1px solid rgba(63, 90, 58, 0.1);
}

.compact-rates strong,
.compact-rates small {
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.54rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  line-height: 1.2;
  text-transform: uppercase;
}

.compact-rates em {
  color: rgb(var(--color-ink-deep));
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  font-size: 1.02rem;
  font-style: normal;
  font-weight: 700;
  line-height: 1.1;
}

.compact-rates small {
  font-size: 0.5rem;
  font-weight: 700;
  letter-spacing: 0.03em;
  text-transform: none;
}

.pricing-stack {
  display: grid;
  gap: 2.25rem;
}

.pricing-frame {
  padding: 0.5rem;
  background: rgb(var(--color-marble));
  border: 1px solid rgba(63, 90, 58, 0.18);
  box-shadow: 0 1px 0 rgba(63, 90, 58, 0.08), 0 0 0 1px rgba(63, 90, 58, 0.04);
}

.pricing-frame__header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.25rem 1.25rem 1rem;
  border-bottom: 1px solid rgba(63, 90, 58, 0.14);
}

.pricing-frame__header h3 {
  margin: 0;
  color: rgb(var(--color-ink-deep));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 1.35rem;
  font-weight: 600;
  line-height: 1.25;
}

.pricing-frame__header p {
  margin: 0.35rem 0 0;
  color: #6f634f;
  font-size: 0.98rem;
  line-height: 1.45;
}

.table-wrapper {
  overflow-x: auto;
}

.pricing-table {
  width: 100%;
  border-collapse: collapse;
  font-family: 'EB Garamond', 'Noto Serif SC', serif;
}

.pricing-table--claude,
.pricing-table--gpt {
  min-width: 760px;
}

.pricing-table--special {
  min-width: 920px;
}

.pricing-frame--special {
  margin-bottom: 2.25rem;
}

.pricing-table th {
  padding: 1.125rem 0.875rem;
  border-bottom: 1.5px solid rgb(var(--color-ink));
  color: rgb(var(--color-muted));
  font-family: 'Cinzel', 'Noto Serif SC', serif;
  font-size: 0.7rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-align: center;
  text-transform: uppercase;
  white-space: nowrap;
}

.pricing-table td {
  padding: 1.125rem 0.875rem;
  border-bottom: 1px solid rgba(63, 90, 58, 0.1);
  color: rgb(var(--color-ink));
  font-size: 1rem;
  text-align: center;
  vertical-align: middle;
}

.pricing-table tr:nth-child(even) td {
  background: rgb(var(--color-stone));
}

.pricing-table tr:hover td {
  background: rgb(var(--color-parchment));
}

.pricing-table tr:last-child td {
  border-bottom: 0;
}

.model-name {
  color: rgb(var(--color-ink-deep));
  font-feature-settings: 'lnum' 1, 'tnum' 1;
  min-width: 11.5rem;
  white-space: nowrap;
}

.model-name strong {
  display: block;
  font-size: 1.04rem;
  font-weight: 700;
  line-height: 1.25;
}

.model-name span {
  display: block;
  margin-top: 0.22rem;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.25;
}

.rate-stack {
  display: inline-grid;
  justify-items: center;
  gap: 0.48rem;
}

.rate-stack span {
  display: grid;
  justify-items: center;
  gap: 0.16rem;
  color: rgb(var(--color-ink-deep));
  font-feature-settings: 'lnum' 1, 'tnum' 1;
}

.rate-stack strong {
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.rate-stack em {
  color: rgb(var(--color-ink-deep));
  font-size: 1.05rem;
  font-style: normal;
  font-weight: 600;
  line-height: 1.2;
}

.pricing-footnote {
  margin: 1.25rem 0 0;
  color: rgb(var(--color-muted));
  font-family: 'Inter', sans-serif;
  font-size: 0.78rem;
  line-height: 1.6;
  text-align: center;
}

.model-pricing__cta {
  display: flex;
  justify-content: center;
  margin-top: 4rem;
}

.cta-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 3.5rem;
  padding: 1rem 3rem;
  background: rgb(var(--color-ink));
  border: 1.5px solid rgb(var(--color-ink));
  color: rgb(var(--color-papyrus));
  font-family: 'Inter', sans-serif;
  font-size: 0.85rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-decoration: none;
  text-transform: uppercase;
  transition: background 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.cta-btn:hover {
  background: rgb(var(--color-terracotta));
  border-color: rgb(var(--color-terracotta));
  transform: translateY(-1px);
}

@media (max-width: 768px) {
  .model-pricing__container {
    width: min(100% - 2rem, 1320px);
  }

  .pricing-frame__header {
    flex-direction: column;
  }

  .pricing-provider-grid {
    grid-template-columns: 1fr;
    gap: 3.75rem;
    margin-top: 3.5rem;
  }

  .provider-card,
  .provider-card--claude,
  .provider-card--qwen,
  .provider-card--glm,
  .provider-card--minimax {
    min-height: auto;
  }

  .provider-card__body {
    max-height: none;
    overflow: visible;
  }

  .compact-rates,
  .compact-rates--three {
    grid-template-columns: 1fr;
  }

  .cta-btn {
    width: 100%;
    padding-inline: 1.25rem;
  }
}

@media (min-width: 769px) and (max-width: 1180px) {
  .pricing-provider-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

}
</style>
