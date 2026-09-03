<template>
  <section class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-3xl font-bold text-gray-950 dark:text-white">模型目录</h1>
      <p class="text-gray-600 dark:text-dark-300">
        同一个模型可以由多个分组提供。请根据预算选择对应分组。
      </p>
      <p v-if="catalog" class="text-xs text-gray-500 dark:text-dark-400">
        更新于 {{ formatUpdatedAt(catalog.updated_at) }}
      </p>
    </header>

    <p class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm leading-6 text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
      模型原生输入：点亮表示支持，灰色表示不支持。客户端抽帧或使用本地工具处理，不算原生支持。
    </p>

    <label class="block">
      <span class="sr-only">搜索模型</span>
      <input
        v-model.trim="query"
        type="search"
        class="w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm text-gray-900 outline-none transition focus:border-primary-400 focus:ring-2 focus:ring-primary-100 dark:border-dark-700 dark:bg-dark-900 dark:text-white dark:focus:border-primary-500 dark:focus:ring-primary-900/30"
        placeholder="搜索模型 ID 或可用分组"
      />
    </label>

    <nav class="flex flex-wrap gap-2" aria-label="模型分类">
      <button
        v-for="category in modelCategories"
        :key="category.key"
        type="button"
        class="inline-flex items-center gap-2 rounded-full border px-3 py-2 text-xs font-semibold transition focus:outline-none focus:ring-2 focus:ring-primary-200 dark:focus:ring-primary-900/40"
        :class="activeCategory === category.key
          ? 'border-primary-600 bg-primary-600 text-white shadow-sm dark:border-primary-500 dark:bg-primary-500'
          : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300 dark:hover:border-primary-700 dark:hover:text-primary-300'"
        :aria-pressed="activeCategory === category.key"
        @click="activeCategory = category.key"
      >
        <ModelIcon v-if="category.iconModel" :model="category.iconModel" size="16px" />
        <span v-else aria-hidden="true" class="grid h-4 w-4 grid-cols-2 gap-0.5">
          <i v-for="index in 4" :key="index" class="rounded-[2px] bg-current opacity-70" />
        </span>
        <span>{{ category.label }}</span>
        <span
          class="rounded-full px-1.5 py-0.5 font-mono text-[10px]"
          :class="activeCategory === category.key ? 'bg-white/20 text-white' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
        >{{ category.count }}</span>
      </button>
    </nav>

    <nav
      v-if="activeCategory === 'domestic'"
      class="-mt-2 flex flex-wrap gap-1.5 rounded-xl border border-gray-200 bg-gray-50 p-2 dark:border-dark-700 dark:bg-dark-900"
      aria-label="国产模型分类"
    >
      <button
        v-for="brand in domesticBrands"
        :key="brand.key"
        type="button"
        class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[11px] font-semibold transition focus:outline-none focus:ring-2 focus:ring-primary-200 dark:focus:ring-primary-900/40"
        :class="activeDomesticBrand === brand.key
          ? 'bg-white text-primary-700 shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:text-primary-300 dark:ring-dark-700'
          : 'text-gray-500 hover:bg-white hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white'"
        :aria-pressed="activeDomesticBrand === brand.key"
        @click="activeDomesticBrand = brand.key"
      >
        <ModelIcon v-if="brand.iconModel" :model="brand.iconModel" size="14px" />
        <span>{{ brand.label }}</span>
        <span class="font-mono text-[9px] opacity-60">{{ brand.count }}</span>
      </button>
    </nav>

    <div v-if="loading" class="rounded-xl border border-gray-200 p-6 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
      正在读取实时模型目录…
    </div>

    <div v-else-if="error" class="rounded-xl border border-red-200 p-6 text-sm text-red-700 dark:border-red-900/50 dark:text-red-300">
      <p>{{ error }}</p>
      <button type="button" class="mt-3 font-semibold underline" @click="load">重新加载</button>
    </div>

    <div v-else class="space-y-3">
      <DocsModelContractCard
        v-for="model in filteredModels"
        :key="model.id"
        :model-id="model.id"
        :contract="modelDocContractById[model.id]"
        :plans="model.plans"
        :candidate-protocols="model.protocols"
      />

      <p v-if="filteredModels.length === 0" class="rounded-xl border border-gray-200 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
        没有匹配的模型。
      </p>
    </div>

  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import DocsModelContractCard from './DocsModelContractCard.vue'
import { getPublicModelPricing } from '@/api/publicPricing'
import type { PublicImageGenerationPricing, PublicModelPricingCatalog } from '@/api/publicPricing'
import { modelDocContractById } from '@/generated/modelDocContracts'

interface AvailabilityPlan {
  groupId: number
  name: string
  multiplier: number
  isExclusive: boolean
  subscriptionType: string
  imageGeneration?: PublicImageGenerationPricing
}

interface LogicalModel {
  id: string
  protocols: string[]
  plans: AvailabilityPlan[]
}

type ModelCategoryKey = 'all' | 'domestic' | 'gpt' | 'claude' | 'grok' | 'gemini' | 'image'
type DomesticBrandKey = 'all' | 'glm' | 'qwen' | 'deepseek' | 'kimi' | 'minimax'

interface ModelCategory {
  key: ModelCategoryKey
  label: string
  iconModel?: string
  count: number
}

interface DomesticBrand {
  key: DomesticBrandKey
  label: string
  iconModel?: string
  count: number
}

const catalog = ref<PublicModelPricingCatalog | null>(null)
const loading = ref(true)
const error = ref('')
const query = ref('')
const activeCategory = ref<ModelCategoryKey>('all')
const activeDomesticBrand = ref<DomesticBrandKey>('all')
const models = computed<LogicalModel[]>(() => {
  const byID = new Map<string, { protocols: Set<string>; plans: AvailabilityPlan[] }>()
  for (const group of catalog.value?.groups ?? []) {
    const addPlan = (modelID: string, protocol: string) => {
      const current = byID.get(modelID) ?? { protocols: new Set<string>(), plans: [] }
      current.protocols.add(protocol)
      if (!current.plans.some((plan) => plan.groupId === group.group_id)) {
        current.plans.push({
          groupId: group.group_id,
          name: group.name,
          multiplier: group.rate_multiplier,
          isExclusive: group.is_exclusive,
          subscriptionType: group.subscription_type,
          imageGeneration: group.image_generation,
        })
      }
      byID.set(modelID, current)
    }

    for (const model of group.models ?? []) {
      if (model.disabled) continue
      for (const protocol of protocolLabels(model.model, group.platform)) {
        addPlan(model.model, protocol)
      }
    }

    if (group.image_generation && (group.models ?? []).length === 0) {
      addPlan('gpt-image-2', 'Images API')
    }
  }

  return [...byID.entries()]
    .map(([id, value]) => ({
      id,
      protocols: [...value.protocols],
      plans: value.plans.sort((a, b) => a.groupId - b.groupId),
    }))
    .sort((a, b) => compareModelDisplayOrder(a.id, b.id))
})

const modelDisplayOrder = [
  'gpt-5.6-sol',
  'gpt-5.6-terra',
  'gpt-daybreak-blue-latest',
  'gpt-5.6-luna',
  'gpt-5.5',
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-5.3-codex-spark',
  'claude-opus-5',
  'claude-sonnet-5',
  'claude-fable-5',
  'claude-opus-4-8',
  'claude-opus-4-7',
  'claude-opus-4-6',
  'claude-sonnet-4-6',
  'claude-opus-4-5',
  'claude-sonnet-4-5',
  'claude-haiku-4-5',
  'glm-5.3',
  'glm-5.2',
  'qwen3.8-max',
  'qwen3.7-max',
  'qwen3.7-plus',
  'qwen3.7-flash',
  'qwen3.6-plus',
  'qwen3.6-flash',
  'deepseek-v4-pro-0813',
  'deepseek-v4-flash-0731',
  'kimi-k3',
  'kimi-k2.7-code',
  'minimax-m3',
  'grok-4.6',
  'grok-4.5',
  'gemini-3.7-flash',
  'gemini-3.1-pro',
  'gpt-image-2',
]

const modelDisplayRank = new Map(modelDisplayOrder.map((model, index) => [model, index]))

function compareModelDisplayOrder(left: string, right: string): number {
  const leftValue = left.toLocaleLowerCase()
  const rightValue = right.toLocaleLowerCase()
  const leftRank = modelDisplayRank.get(leftValue)
  const rightRank = modelDisplayRank.get(rightValue)
  if (leftRank != null && rightRank != null) return leftRank - rightRank
  if (leftRank != null) return -1
  if (rightRank != null) return 1
  return rightValue.localeCompare(leftValue, undefined, { numeric: true })
}

const filteredModels = computed(() => {
  const needle = query.value.toLocaleLowerCase()
  return models.value.filter((model) => {
    if (!matchesModelCategory(model.id, activeCategory.value)) return false
    if (activeCategory.value === 'domestic' && !matchesDomesticBrand(model.id, activeDomesticBrand.value)) return false
    if (!needle) return true
    return model.id.toLocaleLowerCase().includes(needle)
      || model.plans.some((plan) => plan.name.toLocaleLowerCase().includes(needle))
  })
})

const modelCategories = computed<ModelCategory[]>(() => [
  { key: 'all', label: '全部', count: models.value.length },
  { key: 'domestic', label: '国产模型', iconModel: 'glm-5.3', count: countCategory('domestic') },
  { key: 'gpt', label: 'GPT', iconModel: 'gpt-5.6-sol', count: countCategory('gpt') },
  { key: 'claude', label: 'Claude', iconModel: 'claude-sonnet-5', count: countCategory('claude') },
  { key: 'grok', label: 'Grok', iconModel: 'grok-4.6', count: countCategory('grok') },
  { key: 'gemini', label: 'Gemini', iconModel: 'gemini-3.7-flash', count: countCategory('gemini') },
  { key: 'image', label: '生图', iconModel: 'gpt-image-2', count: countCategory('image') },
])

const domesticBrands = computed<DomesticBrand[]>(() => [
  { key: 'all', label: '全部国产', count: countDomesticBrand('all') },
  { key: 'glm', label: 'GLM', iconModel: 'glm-5.3', count: countDomesticBrand('glm') },
  { key: 'qwen', label: 'Qwen', iconModel: 'qwen3.8-max', count: countDomesticBrand('qwen') },
  { key: 'deepseek', label: 'DeepSeek', iconModel: 'deepseek-v4-flash-0731', count: countDomesticBrand('deepseek') },
  { key: 'kimi', label: 'Kimi', iconModel: 'kimi-k3', count: countDomesticBrand('kimi') },
  { key: 'minimax', label: 'MiniMax', iconModel: 'minimax-m3', count: countDomesticBrand('minimax') },
])

function countCategory(category: ModelCategoryKey): number {
  return models.value.filter(model => matchesModelCategory(model.id, category)).length
}

function countDomesticBrand(brand: DomesticBrandKey): number {
  return models.value.filter(model => matchesDomesticBrand(model.id, brand)).length
}

function matchesDomesticBrand(model: string, brand: DomesticBrandKey): boolean {
  if (brand === 'all') return matchesModelCategory(model, 'domestic')
  const value = model.toLocaleLowerCase()
  if (brand === 'glm') return value.startsWith('glm-')
  if (brand === 'qwen') return value.startsWith('qwen')
  if (brand === 'deepseek') return value.startsWith('deepseek-')
  if (brand === 'kimi') return value.startsWith('kimi-')
  return value.startsWith('minimax-')
}

function matchesModelCategory(model: string, category: ModelCategoryKey): boolean {
  if (category === 'all') return true
  const value = model.toLocaleLowerCase()
  if (category === 'domestic') {
    return value.startsWith('glm-')
      || value.startsWith('qwen')
      || value.startsWith('deepseek-')
      || value.startsWith('kimi-')
      || value.startsWith('minimax-')
  }
  if (category === 'gpt') return (value.startsWith('gpt-') || value.includes('codex')) && !value.startsWith('gpt-image')
  if (category === 'claude') return value.startsWith('claude-')
  if (category === 'grok') return value.startsWith('grok-')
  if (category === 'gemini') return value.startsWith('gemini-')
  return value.startsWith('gpt-image')
}

function protocolLabels(model: string, platform: string): string[] {
  const value = model.toLocaleLowerCase()
  if (value === 'gpt-image-2') return ['Images API']
  if (value.startsWith('claude-') || platform === 'anthropic') return ['Anthropic Messages']
  if (value.startsWith('gemini-') || platform === 'gemini') return ['Gemini GenerateContent']
  if (value.startsWith('grok-') || platform === 'grok') return ['Chat Completions']
  if (value.startsWith('qwen') || value.startsWith('deepseek-')) {
    return ['OpenAI Responses', 'Chat Completions']
  }
  if (value.startsWith('glm-') || value.startsWith('kimi-') || value.startsWith('minimax-')) {
    return ['Chat Completions']
  }
  if (value.startsWith('gpt-') || value.includes('codex')) return ['OpenAI Responses']
  if (platform === 'antigravity') return ['Gemini GenerateContent']
  return ['OpenAI compatible（待逐模型验证）']
}

function formatUpdatedAt(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    catalog.value = await getPublicModelPricing()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
