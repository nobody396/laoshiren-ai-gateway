<template>
  <section class="pricing-group">
    <header class="pricing-group__header">
      <div class="pricing-group__title">
        <ModelIcon :model="iconModel" size="18px" />
        <h3>{{ group.name }}</h3>
        <span v-if="protocolLabelText" class="pricing-group__badge">{{ protocolLabelText }}</span>
      </div>
      <div class="pricing-group__meta">
        <span class="pricing-group__rate" :title="t('modelPricing.rateMultiplier')">
          ×{{ group.rate_multiplier }}
        </span>
        <span v-if="group.is_exclusive" class="pricing-group__badge">
          {{ t('modelPricing.exclusive') }}
        </span>
        <span v-if="group.subscription_type === 'credit'" class="pricing-group__badge">
          {{ t('modelPricing.subscription') }}
        </span>
      </div>
    </header>

    <div v-if="pricingRows.length === 0" class="pricing-group__empty">
      {{ t('modelPricing.noModels') }}
    </div>

    <div v-if="pricingRows.length > 0" class="pricing-group__table-wrap">
      <table class="pricing-group__table">
        <thead>
          <tr>
            <th>{{ t('modelPricing.table.model') }}</th>
            <th>{{ t('modelPricing.table.input') }}</th>
            <th>{{ t('modelPricing.table.output') }}</th>
            <th>{{ t('modelPricing.table.cacheWrite') }}</th>
            <th>{{ t('modelPricing.table.cacheRead') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in pricingRows"
            :key="row.key"
            :class="{ 'pricing-group__row--disabled': row.disabled }"
          >
            <td class="pricing-group__model">
              <span class="pricing-group__value">{{ row.label }}</span>
              <span v-if="row.disabled" class="pricing-group__disabled-badge">
                {{ t('modelPricing.disabled') }}
              </span>
            </td>
            <td><span class="pricing-group__value">{{ formatPrice(row.input) }}</span></td>
            <td>
              <span class="pricing-group__value">{{ formatPrice(row.output) }}</span>
              <span v-if="row.outputUnit" class="pricing-group__unit">{{ row.outputUnit }}</span>
            </td>
            <td><span class="pricing-group__value">{{ formatPrice(row.cacheWrite) }}</span></td>
            <td><span class="pricing-group__value">{{ formatPrice(row.cacheRead) }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="timePricingRows.length > 0" class="pricing-group__time-pricing">
      <p class="pricing-group__time-title">{{ t('modelPricing.timePricing.title') }}</p>
      <p v-for="row in timePricingRows" :key="row.key" class="pricing-group__time-row">
        <strong>{{ row.model }}</strong>
        <span>{{ row.timezone }}</span>
        <span v-if="row.weekdaysOnly">{{ t('modelPricing.timePricing.weekdaysOnly') }}</span>
        <span v-for="period in row.periods" :key="`${period.start_time}-${period.end_time}`">
          {{ period.start_time }}–{{ period.end_time }} ×{{ period.multiplier }}
        </span>
      </p>
      <p class="pricing-group__time-note">{{ t('modelPricing.timePricing.basePriceNote') }}</p>
    </div>

  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { PublicPricingGroup } from '@/api/publicPricing'

const props = defineProps<{
  group: PublicPricingGroup
}>()

const { t } = useI18n()

const models = computed(() => props.group.models ?? [])

// 分组品牌图标：取第一个模型的品牌；纯生图分组回退到 gpt-image-2。
const iconModel = computed(() => {
  const first = models.value[0]?.model
  if (first) return first
  if (props.group.image_generation) return 'gpt-image-2'
  return props.group.platform || 'gpt'
})

const isImageOnlyGroup = computed(
  () => Boolean(props.group.image_generation) && models.value.length === 0
)

// 分组头部的协议徽标：写厂商官方 API 全名（Anthropic Messages API、
// OpenAI Responses/Chat Completions API、Gemini v1beta、OpenAI Images API），
// 品牌专有名不做 i18n。
const GROUP_PROTOCOL: Record<string, string> = {
  anthropic: 'Messages API',
  openai: 'Responses / Chat Completions',
  gemini: 'Gemini v1beta',
  antigravity: 'v1beta / Messages API',
  'gpt-image': 'Images API',
  grok: 'Responses / Chat Completions'
}

const protocolLabelText = computed(() => {
  if (isImageOnlyGroup.value) return GROUP_PROTOCOL['gpt-image']
  return GROUP_PROTOCOL[props.group.platform] ?? ''
})

interface PricingRow {
  key: string
  label: string
  input: number | null | undefined
  output: number | null | undefined
  cacheWrite: number | null | undefined
  cacheRead: number | null | undefined
  outputUnit?: string
  disabled?: boolean
}

const imagePricingRows = computed<PricingRow[]>(() => {
  const image = props.group.image_generation
  if (!image) return []

  if (image.mode === 'fixed_per_image') {
    return [
      {
        key: 'image-fixed',
        label: 'GPT Image 2',
        input: null,
        output: image.price_per_image,
        outputUnit: t('modelPricing.image.perImageUnit'),
        cacheWrite: null,
        cacheRead: null
      }
    ]
  }

  return [
    {
      key: 'image-token-text',
      label: `GPT Image 2 · ${t('modelPricing.image.textModality')}`,
      input: image.text_input_price,
      output: null,
      cacheWrite: null,
      cacheRead: image.text_cached_input_price
    },
    {
      key: 'image-token-image',
      label: `GPT Image 2 · ${t('modelPricing.image.imageModality')}`,
      input: image.image_input_price,
      output: image.image_output_price,
      cacheWrite: null,
      cacheRead: image.image_cached_input_price
    }
  ]
})

const pricingRows = computed<PricingRow[]>(() => [
  ...models.value.flatMap((model) => {
    const base: PricingRow = {
      key: `model-${model.model}`,
      label: model.context_intervals?.length ? `${model.model} · ${t('modelPricing.contextPricing.base')}` : model.model,
      input: model.input_price,
      output: model.output_price,
      cacheWrite: model.cache_write_price,
      cacheRead: model.cache_read_price,
      disabled: model.disabled
    }
    const intervals = (model.context_intervals || []).map((interval, index): PricingRow => ({
      key: `model-${model.model}-interval-${index}`,
      label: `${model.model} · ${formatContextRange(interval.min_tokens, interval.max_tokens)}`,
      input: interval.input_price,
      output: interval.output_price,
      cacheWrite: interval.cache_write_price,
      cacheRead: interval.cache_read_price,
      disabled: model.disabled
    }))
    return [base, ...intervals]
  }),
  // 生图行固定排在文本模型之后
  ...imagePricingRows.value
])

const timePricingRows = computed(() => models.value
  .filter(model => model.time_pricing?.periods?.length)
  .map(model => ({
    key: model.model,
    model: model.model,
    timezone: model.time_pricing!.timezone,
    weekdaysOnly: model.time_pricing!.weekdays_only === true,
    periods: model.time_pricing!.periods
  })))

function formatContextRange(min: number, max?: number): string {
  const lower = min.toLocaleString()
  return max == null
    ? t('modelPricing.contextPricing.above', { min: lower })
    : t('modelPricing.contextPricing.range', { min: lower, max: max.toLocaleString() })
}

function formatPrice(v: number | null | undefined): string {
  if (v === null || v === undefined) return '—'
  const digits = v >= 1 ? 2 : 4
  return `¥${v.toFixed(digits)}`
}
</script>

<style scoped>
.pricing-group {
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #ffffff;
  overflow: hidden;
}

.pricing-group__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1.25rem;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
}

.pricing-group__title {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  min-width: 0;
}

.pricing-group__title h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: #111827;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pricing-group__platform {
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 600;
}

.pricing-group__meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.pricing-group__rate {
  padding: 0.125rem 0.5rem;
  border-radius: 6px;
  background: #eef2ff;
  color: #4338ca;
  font-size: 0.75rem;
  font-weight: 700;
}

.pricing-group__badge {
  padding: 0.125rem 0.5rem;
  border-radius: 6px;
  background: #f3f4f6;
  color: #6b7280;
  font-size: 0.7rem;
  font-weight: 600;
}

.pricing-group__empty {
  padding: 1.25rem;
  color: #9ca3af;
  font-size: 0.875rem;
  text-align: center;
}

.pricing-group__table-wrap {
  overflow-x: auto;
}

.pricing-group__time-pricing {
  padding: 0.875rem 1.25rem;
  border-top: 1px solid rgb(var(--color-gray-200));
  color: rgb(var(--color-muted));
  font-size: 0.75rem;
}

.pricing-group__time-title {
  margin: 0 0 0.375rem;
  color: rgb(var(--color-ink));
  font-weight: 700;
}

.pricing-group__time-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem 0.75rem;
  margin: 0.25rem 0;
}

.pricing-group__time-note {
  margin: 0.5rem 0 0;
}

.pricing-group__table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.pricing-group__table th {
  padding: 0.625rem 1.25rem;
  text-align: right;
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
  background: #ffffff;
}

.pricing-group__table th:first-child,
.pricing-group__table td:first-child {
  text-align: left;
}

.pricing-group__table td {
  padding: 0.625rem 1.25rem;
  text-align: right;
  color: #111827;
  border-bottom: 1px solid #f3f4f6;
  font-variant-numeric: tabular-nums;
}

.pricing-group__table tr:last-child td {
  border-bottom: none;
}

.pricing-group__model {
  font-family: 'SFMono-Regular', 'Menlo', 'Consolas', monospace;
  font-size: 0.8125rem;
  word-break: break-all;
}

.pricing-group__row--disabled .pricing-group__value {
  color: #9ca3af;
  text-decoration: line-through;
  text-decoration-thickness: 1.5px;
}

.pricing-group__disabled-badge {
  display: inline-flex;
  margin-left: 0.5rem;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: #f3f4f6;
  color: #6b7280;
  font-family: inherit;
  font-size: 0.65rem;
  font-weight: 700;
  text-decoration: none;
}

.pricing-group__unit {
  margin-left: 0.25rem;
  color: #6b7280;
  font-size: 0.75rem;
}
</style>
