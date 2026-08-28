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

    <div v-if="pricingModelGroups.length === 0" class="pricing-group__empty">
      {{ t('modelPricing.noModels') }}
    </div>

    <div v-if="pricingModelGroups.length > 0" class="pricing-group__table-wrap">
      <table class="pricing-group__table">
        <colgroup>
          <col class="pricing-group__col-model" />
          <col class="pricing-group__col-tier" />
          <col class="pricing-group__col-price" />
          <col class="pricing-group__col-price" />
          <col class="pricing-group__col-price" />
          <col class="pricing-group__col-price" />
        </colgroup>
        <thead>
          <tr>
            <th class="pricing-group__model-header">{{ t('modelPricing.table.model') }}</th>
            <th class="pricing-group__tier-header">{{ t('modelPricing.table.tier') }}</th>
            <th class="pricing-group__price-header">{{ t('modelPricing.table.input') }}</th>
            <th class="pricing-group__price-header">{{ t('modelPricing.table.output') }}</th>
            <th class="pricing-group__price-header">{{ t('modelPricing.table.cacheWrite') }}</th>
            <th class="pricing-group__price-header">{{ t('modelPricing.table.cacheRead') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="modelGroup in pricingModelGroups" :key="modelGroup.key">
            <tr
              v-for="(row, rowIndex) in modelGroup.rows"
              :key="`${modelGroup.key}-${rowIndex}`"
              :class="[
                { 'pricing-group__row--disabled': modelGroup.disabled },
                rowIndex > 0 ? 'pricing-group__subrow' : ''
              ]"
            >
              <td v-if="rowIndex === 0" :rowspan="modelGroup.rows.length" class="pricing-group__model">
                <span class="pricing-group__value">{{ modelGroup.label }}</span>
                <span v-if="modelGroup.disabled" class="pricing-group__disabled-badge">
                  {{ t('modelPricing.disabled') }}
                </span>
              </td>
              <td class="pricing-group__tier-label">{{ row.tierLabel }}</td>
              <td class="pricing-group__price-cell"><span class="pricing-group__value">{{ formatPrice(row.input) }}</span></td>
              <td class="pricing-group__price-cell">
                <span class="pricing-group__value">{{ formatPrice(row.output) }}</span>
                <span v-if="row.outputUnit" class="pricing-group__unit">{{ row.outputUnit }}</span>
              </td>
              <td class="pricing-group__price-cell"><span class="pricing-group__value">{{ formatPrice(row.cacheWrite) }}</span></td>
              <td class="pricing-group__price-cell"><span class="pricing-group__value">{{ formatPrice(row.cacheRead) }}</span></td>
            </tr>
          </template>
        </tbody>
      </table>
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

interface PricingValueRow {
  tierLabel: string
  input: number | null | undefined
  output: number | null | undefined
  cacheWrite: number | null | undefined
  cacheRead: number | null | undefined
  outputUnit?: string
}

interface PricingModelGroup {
  key: string
  label: string
  rows: PricingValueRow[]
  disabled?: boolean
}

const imagePricingGroup = computed<PricingModelGroup | null>(() => {
  const image = props.group.image_generation
  if (!image) return null

  if (image.mode === 'fixed_per_image') {
    return {
      key: 'image-fixed',
      label: 'GPT Image 2',
      rows: [{
        tierLabel: t('modelPricing.table.perImage'),
        input: null,
        output: image.price_per_image,
        outputUnit: t('modelPricing.image.perImageUnit'),
        cacheWrite: null,
        cacheRead: null
      }]
    }
  }

  return {
    key: 'image-token',
    label: 'GPT Image 2',
    rows: [
      {
        tierLabel: t('modelPricing.image.textModality'),
        input: image.text_input_price,
        output: null,
        cacheWrite: null,
        cacheRead: image.text_cached_input_price
      },
      {
        tierLabel: t('modelPricing.image.imageModality'),
        input: image.image_input_price,
        output: image.image_output_price,
        cacheWrite: null,
        cacheRead: image.image_cached_input_price
      }
    ]
  }
})

type ModelPrice = NonNullable<PublicPricingGroup['models']>[number]
type ContextTier = NonNullable<ModelPrice['context_intervals']>[number]
type PriceShape = Pick<ContextTier, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price'>

function displayBasePrice(model: ModelPrice): PriceShape {
  return model.context_intervals?.find((interval) => interval.min_tokens === 0) ?? model
}

interface TimePricePeriod extends PriceShape {
  startTime: number
  endTime: number
  multiplier: number
  wrapsMidnight: boolean
  label: string
}

const SECONDS_PER_DAY = 24 * 60 * 60

function timeToSeconds(value: string, isEnd = false): number {
  const [hours = 0, minutes = 0, seconds = 0] = value.split(':').map(Number)
  if (isEnd && hours === 0 && minutes === 0 && seconds === 0) return SECONDS_PER_DAY
  return hours * 3600 + minutes * 60 + seconds
}

function formatClock(seconds: number): string {
  const normalized = seconds === SECONDS_PER_DAY ? 0 : seconds
  const hours = Math.floor(normalized / 3600).toString().padStart(2, '0')
  const minutes = Math.floor((normalized % 3600) / 60).toString().padStart(2, '0')
  return `${hours}:${minutes}`
}

function multiplyPrice(value: number | null | undefined, multiplier: number): number | null {
  return value === null || value === undefined ? null : value * multiplier
}

function withPeriodPrices(base: PriceShape, startTime: number, endTime: number, multiplier: number, wrapsMidnight = false): TimePricePeriod {
  const endLabel = wrapsMidnight
    ? `${t('modelPricing.timePricing.nextDay')}${formatClock(endTime)}`
    : formatClock(endTime)
  return {
    startTime,
    endTime,
    multiplier,
    wrapsMidnight,
    label: `${formatClock(startTime)}–${endLabel}`,
    input_price: multiplyPrice(base.input_price, multiplier),
    output_price: multiplyPrice(base.output_price, multiplier),
    cache_write_price: multiplyPrice(base.cache_write_price, multiplier),
    cache_read_price: multiplyPrice(base.cache_read_price, multiplier)
  }
}

function explicitTimePrices(model: ModelPrice): TimePricePeriod[] {
  const configured = (model.time_pricing?.periods ?? [])
    .map((period) => ({
      startTime: timeToSeconds(period.start_time),
      endTime: timeToSeconds(period.end_time, true),
      multiplier: period.multiplier
    }))
    .sort((a, b) => a.startTime - b.startTime)
  if (configured.length === 0) return []

  const segments: Array<{ startTime: number; endTime: number; multiplier: number }> = []
  let cursor = 0
  for (const period of configured) {
    if (period.startTime > cursor) segments.push({ startTime: cursor, endTime: period.startTime, multiplier: 1 })
    segments.push(period)
    cursor = period.endTime
  }
  if (cursor < SECONDS_PER_DAY) segments.push({ startTime: cursor, endTime: SECONDS_PER_DAY, multiplier: 1 })

  const merged: typeof segments = []
  for (const segment of segments) {
    const previous = merged[merged.length - 1]
    if (previous && previous.endTime === segment.startTime && previous.multiplier === segment.multiplier) {
      previous.endTime = segment.endTime
    } else {
      merged.push({ ...segment })
    }
  }

  const first = merged[0]
  const last = merged[merged.length - 1]
  const wraps = merged.length > 1 && first.startTime === 0 && last.endTime === SECONDS_PER_DAY && first.multiplier === last.multiplier
  const normalized: Array<{ startTime: number; endTime: number; multiplier: number; wrapsMidnight: boolean }> = wraps
    ? [
        ...merged.slice(1, -1).map((period) => ({ ...period, wrapsMidnight: false })),
        { startTime: last.startTime, endTime: first.endTime, multiplier: first.multiplier, wrapsMidnight: true }
      ]
    : merged.map((period) => ({ ...period, wrapsMidnight: false }))

  const base = displayBasePrice(model)
  return normalized
    .map((period) => withPeriodPrices(base, period.startTime, period.endTime, period.multiplier, period.wrapsMidnight))
    .sort((a, b) => b.multiplier - a.multiplier || a.startTime - b.startTime)
}

function formatTokenThreshold(tokens: number): string {
  if (tokens >= 1_000_000 && tokens % 1_000_000 === 0) return `${tokens / 1_000_000}M`
  if (tokens >= 1024 && tokens % 1024 === 0) return `${tokens / 1024}K`
  return tokens.toLocaleString()
}

function contextTierLabel(tier: ContextTier): string {
  const max = tier.max_tokens == null ? '' : formatTokenThreshold(tier.max_tokens)
  if (tier.min_tokens === 0) return `≤${max}`
  const min = formatTokenThreshold(tier.min_tokens)
  return max ? `>${min}–${max}` : `>${min}`
}

function valueRow(label: string, price: PriceShape, outputUnit?: string): PricingValueRow {
  return {
    tierLabel: label,
    input: price.input_price,
    output: price.output_price,
    cacheWrite: price.cache_write_price,
    cacheRead: price.cache_read_price,
    outputUnit
  }
}

const pricingModelGroups = computed<PricingModelGroup[]>(() => {
  const textGroups = models.value.map((model): PricingModelGroup => {
    const timePrices = explicitTimePrices(model)
    if (timePrices.length > 0) {
      return {
        key: `model-${model.model}`,
        label: model.model,
        disabled: model.disabled,
        rows: timePrices.map((period) => valueRow(
          `${period.multiplier < 1 ? t('modelPricing.timePricing.valley') : t('modelPricing.timePricing.peak')} · ${period.label}`,
          period
        ))
      }
    }

    const contextPrices = [...(model.context_intervals ?? [])].sort((a, b) => a.min_tokens - b.min_tokens)
    if (contextPrices.length > 0) {
      return {
        key: `model-${model.model}`,
        label: model.model,
        disabled: model.disabled,
        rows: contextPrices.map((tier) => valueRow(contextTierLabel(tier), tier))
      }
    }

    return {
      key: `model-${model.model}`,
      label: model.model,
      disabled: model.disabled,
      rows: [valueRow(t('modelPricing.table.standard'), model)]
    }
  })
  return imagePricingGroup.value ? [...textGroups, imagePricingGroup.value] : textGroups
})

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

.pricing-group__table {
  width: 100%;
  min-width: 880px;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.pricing-group__col-model,
.pricing-group__col-tier {
  width: 25%;
}

.pricing-group__col-price {
  width: 12.5%;
}

.pricing-group__table th {
  padding: 0.625rem 1.25rem;
  text-align: left;
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
  background: #ffffff;
}

.pricing-group__model-header,
.pricing-group__tier-header,
.pricing-group__model,
.pricing-group__tier-label {
  text-align: left;
}

.pricing-group__price-header,
.pricing-group__price-cell {
  text-align: left;
}

.pricing-group__table td {
  padding: 0.625rem 1.25rem;
  text-align: left;
  color: #111827;
  border-bottom: 1px solid #f3f4f6;
  font-variant-numeric: tabular-nums;
}

.pricing-group__subrow td {
  border-top: 1px dashed #f3f4f6;
}

.pricing-group__tier-label {
  color: #4b5563;
  font-size: 0.75rem;
  font-weight: 600;
  white-space: nowrap;
}

.pricing-group__table tr:last-child td {
  border-bottom: none;
}

.pricing-group__model {
  font-family: 'SFMono-Regular', 'Menlo', 'Consolas', monospace;
  font-size: 0.8125rem;
  vertical-align: middle;
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
