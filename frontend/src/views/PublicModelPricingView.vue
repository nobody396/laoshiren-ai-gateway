<template>
  <div class="model-pricing-page">
    <header class="model-pricing-header">
      <router-link to="/" class="model-pricing-brand">
        <img src="/laoshirenai-icon.jpg" alt="老实人AI" />
        <span>老实人AI</span>
      </router-link>
      <nav class="model-pricing-nav">
        <router-link to="/enterprise">{{ t('modelPricing.nav.enterprise') }}</router-link>
        <router-link to="/security">{{ t('modelPricing.nav.security') }}</router-link>
        <router-link to="/status">{{ t('modelPricing.nav.status') }}</router-link>
        <router-link to="/docs">{{ t('modelPricing.nav.docs') }}</router-link>
        <router-link to="/login">{{ t('modelPricing.nav.login') }}</router-link>
      </nav>
    </header>

    <main class="model-pricing-main">
      <div class="model-pricing-intro">
        <h1>{{ t('modelPricing.title') }}</h1>
        <p class="model-pricing-lede">{{ t('modelPricing.lede') }}</p>
        <p v-if="catalog" class="model-pricing-updated">
          {{ t('modelPricing.updatedAt') }} {{ formatUpdatedAt(catalog.updated_at) }}
        </p>
      </div>

      <PricingBillingExample />

      <nav v-if="!loading && !error && blocks.length > 0" class="model-pricing-tabs" aria-label="模型厂商">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="model-pricing-tabs__item"
          :class="{ 'model-pricing-tabs__item--active': activeTab === tab.key }"
          type="button"
          @click="activeTab = tab.key"
        >
          <ModelIcon v-if="tab.iconModel" :model="tab.iconModel" size="14px" />
          <Icon v-else-if="tab.icon" :name="tab.icon" size="xs" />
          {{ t(`modelPricing.block.${tab.key}`) }}
        </button>
      </nav>

      <div v-if="loading" class="model-pricing-state">{{ t('modelPricing.loading') }}</div>

      <div v-else-if="error" class="model-pricing-state">
        <p class="model-pricing-error">{{ t('modelPricing.error') }}: {{ error }}</p>
        <button class="model-pricing-retry" @click="load">{{ t('modelPricing.retry') }}</button>
      </div>

      <div v-else-if="catalog && catalog.groups.length === 0" class="model-pricing-state">
        {{ t('modelPricing.empty') }}
      </div>

      <div v-else-if="catalog" class="model-pricing-blocks">
        <section
          v-for="b in visibleBlocks"
          :id="`block-${b.key}`"
          :key="b.key"
          class="model-pricing-block"
        >
          <h2 class="model-pricing-block__title">
            <ModelIcon v-if="blockIconModel(b.key)" :model="blockIconModel(b.key)" size="20px" />
            <Icon v-else-if="b.key === 'builderPass'" name="creditCard" size="md" />
            {{ t(`modelPricing.block.${b.key}`) }}
          </h2>
          <div class="model-pricing-block__groups">
            <ModelPricingGroupSection
              v-for="g in b.groups"
              :key="g.group_id"
              :group="g"
            />
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ModelPricingGroupSection from '@/components/pricing/ModelPricingGroupSection.vue'
import PricingBillingExample from '@/components/pricing/PricingBillingExample.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPublicModelPricing } from '@/api/publicPricing'
import type { PublicModelPricingCatalog, PublicPricingGroup } from '@/api/publicPricing'

const { t } = useI18n()
const catalog = ref<PublicModelPricingCatalog | null>(null)
const loading = ref(true)
const error = ref('')

// 厂商分块展示顺序：GPT 在前，然后 Claude，之后 Grok / GLM / DeepSeek 等；
// 月卡（Builder Pass）分块排在厂商分块之后、「其他」之前。
// 月卡组会同时出现在所属厂商分块和 Builder Pass 分块（运营要求 2026-08-19）。
const BLOCK_ORDER = ['gpt', 'claude', 'grok', 'glm', 'deepseek', 'qwen', 'minimax', 'builderPass', 'other']

// 每个厂商分块 tab / 标题用的品牌图标（ModelIcon 按模型名匹配品牌）
const BLOCK_ICON_MODEL: Record<string, string> = {
  gpt: 'gpt',
  claude: 'claude',
  grok: 'grok',
  glm: 'glm',
  deepseek: 'deepseek',
  qwen: 'qwen',
  minimax: 'minimax',
}

function blockIconModel(key: string): string {
  return BLOCK_ICON_MODEL[key] ?? ''
}

function classifyBlock(group: PublicPricingGroup): string {
  if (group.image_generation) return 'gpt'
  const joined = (group.models ?? []).map((model) => model.model).join(' ').toLowerCase()
  if (/\bgpt[-\s]/.test(joined)) return 'gpt'
  if (/\bclaude[-\s]/.test(joined)) return 'claude'
  if (/\bgrok[-\s]/.test(joined)) return 'grok'
  if (/\bglm[-\s]/.test(joined)) return 'glm'
  if (/\bdeepseek[-\s]/.test(joined)) return 'deepseek'
  if (/\bqwen[-\s]/.test(joined)) return 'qwen'
  if (/\bminimax[-\s]/.test(joined)) return 'minimax'
  return 'other'
}

type PricingBlock = { key: string; groups: PublicPricingGroup[] }
type PricingTab = { key: string; iconModel?: string; icon?: 'grid' | 'creditCard' }

const blocks = computed<PricingBlock[]>(() => {
  if (!catalog.value) return []
  const map = new Map<string, PublicPricingGroup[]>()
  const push = (key: string, g: PublicPricingGroup) => {
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(g)
  }
  for (const g of catalog.value.groups) {
    push(classifyBlock(g), g)
    // 月卡（credit 订阅）分组额外归入 Builder Pass 分块，与厂商分块同时展示。
    if (g.subscription_type === 'credit') push('builderPass', g)
  }
  return BLOCK_ORDER.filter((k) => map.has(k)).map((k) => ({ key: k, groups: map.get(k)! }))
})

// 顶部 tab：默认「全部」展示所有分块；选中某个厂商/Builder Pass 时只显示对应分块。
// 支持 ?tab=<key> 直达某个分块（便于分享链接，如 ?tab=builderPass）。
const VALID_TABS = new Set(['all', ...BLOCK_ORDER])
const initialTab = new URLSearchParams(window.location.search).get('tab') ?? 'all'
const activeTab = ref(VALID_TABS.has(initialTab) ? initialTab : 'all')

const tabs = computed<PricingTab[]>(() => [
  { key: 'all', icon: 'grid' },
  ...blocks.value.map((b): PricingTab => {
    if (b.key === 'builderPass') return { key: b.key, icon: 'creditCard' }
    const iconModel = blockIconModel(b.key)
    return iconModel ? { key: b.key, iconModel } : { key: b.key }
  }),
])

const visibleBlocks = computed<PricingBlock[]>(() => {
  if (activeTab.value === 'all') return blocks.value
  return blocks.value.filter((b) => b.key === activeTab.value)
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    catalog.value = await getPublicModelPricing()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function formatUpdatedAt(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

onMounted(load)
</script>

<style scoped>
.model-pricing-page {
  min-height: 100vh;
  background: #ffffff;
  color: #374151;
  color-scheme: light;
}

.model-pricing-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  max-width: 1080px;
  margin: 0 auto;
  padding: 1rem 1.25rem;
}

.model-pricing-brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 700;
  color: #111827;
  text-decoration: none;
}

.model-pricing-brand img {
  width: 28px;
  height: 28px;
  border-radius: 6px;
}

.model-pricing-nav {
  display: flex;
  align-items: center;
  gap: 1.25rem;
}

.model-pricing-nav a {
  color: #6b7280;
  font-size: 0.875rem;
  text-decoration: none;
}

.model-pricing-nav a:hover {
  color: #111827;
}

.model-pricing-main {
  max-width: 1080px;
  margin: 0 auto;
  padding: 2rem 1.25rem 4rem;
}

.model-pricing-intro {
  margin-bottom: 1.5rem;
}

.model-pricing-intro h1 {
  margin: 0 0 0.5rem;
  color: #111827;
  font-size: 1.875rem;
  font-weight: 700;
}

.model-pricing-lede {
  margin: 0;
  color: #6b7280;
}

.model-pricing-updated {
  margin: 0.5rem 0 0;
  color: #9ca3af;
  font-size: 0.8125rem;
}

.model-pricing-tabs {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 2rem;
  padding: 0.75rem 1rem;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(6px);
}

.model-pricing-tabs__item {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.4rem 0.9rem;
  border: 1px solid #d1d5db;
  border-radius: 999px;
  background: #ffffff;
  color: #374151;
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.model-pricing-tabs__item:hover {
  border-color: #111827;
  color: #111827;
}

.model-pricing-tabs__item--active,
.model-pricing-tabs__item--active:hover {
  border-color: #111827;
  color: #111827;
  box-shadow: inset 0 0 0 1px #111827;
}

.model-pricing-block {
  scroll-margin-top: 96px;
  margin-bottom: 2.5rem;
}

.model-pricing-block__title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0 0 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #e5e7eb;
  color: #111827;
  font-size: 1.375rem;
  font-weight: 700;
}

.model-pricing-block__groups {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.model-pricing-state {
  padding: 3rem 1rem;
  color: #6b7280;
  text-align: center;
}

.model-pricing-error {
  margin: 0 0 1rem;
  color: #b91c1c;
}

.model-pricing-retry {
  padding: 0.5rem 1.25rem;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: #ffffff;
  color: #111827;
  font-size: 0.875rem;
  cursor: pointer;
}

.model-pricing-retry:hover {
  background: #f9fafb;
}
</style>
