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

    <div v-if="contextPricingRows.length > 0" class="pricing-group__context-pricing">
      <div class="pricing-group__detail-heading">
        <div>
          <p class="pricing-group__detail-title">{{ t('modelPricing.contextPricing.title') }}</p>
          <p class="pricing-group__detail-note">{{ t('modelPricing.contextPricing.summaryNote') }}</p>
        </div>
      </div>
      <div class="pricing-group__context-list">
        <div v-for="row in contextPricingRows" :key="row.model" class="pricing-group__context-row">
          <strong>{{ row.model }}</strong>
          <div class="pricing-group__tier-list">
            <div v-for="tier in row.tiers" :key="`${row.model}-${tier.min_tokens}`" class="pricing-group__tier-pill">
              <span class="pricing-group__tier-threshold">
                {{ t('modelPricing.contextPricing.aboveThreshold', { threshold: formatTokenThreshold(tier.min_tokens) }) }}
              </span>
              <span>{{ compactPriceLabel(tier) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="timePricingRows.length > 0" class="pricing-group__time-pricing">
      <div class="pricing-group__detail-heading">
        <div>
          <p class="pricing-group__detail-title">{{ t('modelPricing.timePricing.title') }}</p>
          <p class="pricing-group__detail-note">{{ t('modelPricing.timePricing.explicitPriceNote') }}</p>
        </div>
      </div>
      <div class="pricing-group__time-list">
        <div v-for="row in timePricingRows" :key="row.key" class="pricing-group__time-model">
          <div class="pricing-group__time-model-title">
            <strong>{{ row.model }}</strong>
            <span>{{ row.timezone }}</span>
            <span v-if="row.weekdaysOnly">{{ t('modelPricing.timePricing.weekdaysOnly') }}</span>
          </div>
          <div class="pricing-group__time-tier-list">
            <div v-for="period in row.periods" :key="`${period.startTime}-${period.endTime}-${period.multiplier}`" class="pricing-group__time-tier">
              <span :class="['pricing-group__time-badge', period.multiplier < 1 ? 'is-valley' : 'is-peak']">
                {{ period.multiplier < 1 ? t('modelPricing.timePricing.valley') : t('modelPricing.timePricing.peak') }}
              </span>
              <span class="pricing-group__time-range">{{ period.label }}</span>
              <span>{{ compactPriceLabel(period) }}</span>
            </div>
          </div>
        </div>
      </div>
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

type ModelPrice = NonNullable<PublicPricingGroup['models']>[number]
type ContextTier = NonNullable<ModelPrice['context_intervals']>[number]
type PriceShape = Pick<ContextTier, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price'>

function displayBasePrice(model: ModelPrice): PriceShape {
  return model.context_intervals?.find((interval) => interval.min_tokens === 0) ?? model
}

const pricingRows = computed<PricingRow[]>(() => [
  ...models.value.map((model) => {
    const price = displayBasePrice(model)
    return {
      key: `model-${model.model}`,
      label: model.model,
      input: price.input_price,
      output: price.output_price,
      cacheWrite: price.cache_write_price,
      cacheRead: price.cache_read_price,
      disabled: model.disabled
    }
  }),
  // 生图行固定排在文本模型之后
  ...imagePricingRows.value
])

const contextPricingRows = computed(() => models.value.flatMap((model) => {
  const tiers = [...(model.context_intervals ?? [])]
    .filter((interval) => interval.min_tokens > 0)
    .sort((a, b) => a.min_tokens - b.min_tokens)
  return tiers.length > 0 ? [{ model: model.model, tiers }] : []
}))

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

const timePricingRows = computed(() => models.value
  .filter((model) => model.time_pricing?.periods?.length)
  .map((model) => ({
    key: model.model,
    model: model.model,
    timezone: model.time_pricing!.timezone,
    weekdaysOnly: model.time_pricing!.weekdays_only === true,
    periods: explicitTimePrices(model)
  })))

function formatTokenThreshold(tokens: number): string {
  if (tokens >= 1_000_000 && tokens % 1_000_000 === 0) return `${tokens / 1_000_000}M`
  if (tokens >= 1024 && tokens % 1024 === 0) return `${tokens / 1024}K`
  return tokens.toLocaleString()
}

function compactPriceLabel(price: PriceShape): string {
  const parts = [
    price.input_price == null ? '' : `${t('modelPricing.table.input')} ${formatPrice(price.input_price)}`,
    price.output_price == null ? '' : `${t('modelPricing.table.output')} ${formatPrice(price.output_price)}`,
    price.cache_write_price == null ? '' : `${t('modelPricing.table.cacheWrite')} ${formatPrice(price.cache_write_price)}`,
    price.cache_read_price == null ? '' : `${t('modelPricing.table.cacheRead')} ${formatPrice(price.cache_read_price)}`
  ]
  return parts.filter(Boolean).join(' · ')
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

.pricing-group__context-pricing,
.pricing-group__time-pricing {
  padding: 0.875rem 1.25rem;
  border-top: 1px solid #e5e7eb;
  background: #fafafa;
  color: #6b7280;
  font-size: 0.75rem;
}

.pricing-group__detail-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.pricing-group__detail-title {
  margin: 0;
  color: #111827;
  font-weight: 700;
}

.pricing-group__detail-note {
  margin: 0.25rem 0 0;
  color: #6b7280;
  line-height: 1.5;
}

.pricing-group__context-list,
.pricing-group__time-list {
  display: grid;
  gap: 0.625rem;
  margin-top: 0.75rem;
}

.pricing-group__context-row,
.pricing-group__time-model {
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #ffffff;
  padding: 0.75rem;
}

.pricing-group__context-row > strong,
.pricing-group__time-model-title strong {
  color: #111827;
  font-family: 'SFMono-Regular', 'Menlo', 'Consolas', monospace;
  font-size: 0.75rem;
}

.pricing-group__tier-list,
.pricing-group__time-tier-list {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.pricing-group__tier-pill,
.pricing-group__time-tier {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.75rem;
  border-radius: 8px;
  background: #f9fafb;
  padding: 0.5rem 0.625rem;
  font-variant-numeric: tabular-nums;
}

.pricing-group__tier-threshold,
.pricing-group__time-range {
  color: #374151;
  font-weight: 700;
}

.pricing-group__time-model-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem 0.75rem;
}

.pricing-group__time-badge {
  display: inline-flex;
  min-width: 3rem;
  justify-content: center;
  border-radius: 999px;
  padding: 0.2rem 0.55rem;
  font-weight: 700;
}

.pricing-group__time-badge.is-peak {
  background: #fff7ed;
  color: #c2410c;
}

.pricing-group__time-badge.is-valley {
  background: #ecfdf5;
  color: #047857;
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
