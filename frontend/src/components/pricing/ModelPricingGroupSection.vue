<template>
  <section class="pricing-group">
    <header class="pricing-group__header">
      <div class="pricing-group__title">
        <h3>{{ group.name }}</h3>
        <span
          v-if="group.platform"
          class="pricing-group__platform"
          :class="platformBadgeClass(group.platform)"
        >
          {{ platformLabel(group.platform) }}
        </span>
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

    <div v-if="group.image_generation" class="pricing-group__image">
      <div class="pricing-group__image-heading">
        <div>
          <span class="pricing-group__image-title">{{ t('modelPricing.image.title') }}</span>
          <span class="pricing-group__image-unit">
            {{
              group.image_generation.mode === 'fixed_per_image'
                ? t('modelPricing.image.successfulImage')
                : t('modelPricing.image.perMillionTokens')
            }}
          </span>
        </div>
        <span class="pricing-group__image-badge">GPT Image 2</span>
      </div>

      <div
        v-if="group.image_generation.mode === 'fixed_per_image'"
        class="pricing-group__fixed-price"
      >
        <strong>{{ formatPrice(group.image_generation.price_per_image) }}</strong>
        <span>{{ t('modelPricing.image.perSuccessfulImage') }}</span>
      </div>
      <div v-else class="pricing-group__image-prices">
        <div v-for="item in imageTokenPrices" :key="item.label" class="pricing-group__image-price">
          <span>{{ item.label }}</span>
          <strong>{{ formatPrice(item.value) }}</strong>
        </div>
      </div>

      <p v-if="group.description" class="pricing-group__description">
        <strong>{{ t('modelPricing.image.parameters') }}：</strong>{{ group.description }}
      </p>
    </div>

    <div v-if="group.models.length === 0 && !group.image_generation" class="pricing-group__empty">
      {{ t('modelPricing.noModels') }}
    </div>

    <table v-if="group.models.length > 0" class="pricing-group__table">
      <thead>
        <tr>
          <th>{{ t('modelPricing.table.model') }}</th>
          <th>{{ t('modelPricing.table.input') }}</th>
          <th>{{ t('modelPricing.table.output') }}</th>
          <th>{{ t('modelPricing.table.cacheRead') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in group.models" :key="m.model" :class="{ 'pricing-group__row--disabled': m.disabled }">
          <td class="pricing-group__model">
            <span class="pricing-group__value">{{ m.model }}</span>
            <span v-if="m.disabled" class="pricing-group__disabled-badge">
              {{ t('modelPricing.disabled') }}
            </span>
          </td>
          <td><span class="pricing-group__value">{{ formatPrice(m.input_price) }}</span></td>
          <td><span class="pricing-group__value">{{ formatPrice(m.output_price) }}</span></td>
          <td><span class="pricing-group__value">{{ formatPrice(m.cache_read_price) }}</span></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'
import type { PublicPricingGroup } from '@/api/publicPricing'

const props = defineProps<{
  group: PublicPricingGroup
}>()

const { t } = useI18n()

const imageTokenPrices = computed(() => {
  const image = props.group.image_generation
  if (!image || image.mode !== 'token') return []
  return [
    { label: t('modelPricing.image.textInput'), value: image.text_input_price },
    { label: t('modelPricing.image.textCachedInput'), value: image.text_cached_input_price },
    { label: t('modelPricing.image.imageInput'), value: image.image_input_price },
    { label: t('modelPricing.image.imageCachedInput'), value: image.image_cached_input_price },
    { label: t('modelPricing.image.imageOutput'), value: image.image_output_price }
  ]
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

.pricing-group__image {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid #e5e7eb;
  background: linear-gradient(135deg, #faf5ff 0%, #eff6ff 100%);
}

.pricing-group__image-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.pricing-group__image-title {
  color: #111827;
  font-size: 0.875rem;
  font-weight: 700;
}

.pricing-group__image-unit {
  margin-left: 0.5rem;
  color: #6b7280;
  font-size: 0.75rem;
}

.pricing-group__image-badge {
  padding: 0.2rem 0.55rem;
  border: 1px solid #c4b5fd;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.8);
  color: #6d28d9;
  font-size: 0.7rem;
  font-weight: 700;
}

.pricing-group__fixed-price {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}

.pricing-group__fixed-price strong {
  color: #5b21b6;
  font-size: 1.35rem;
  font-variant-numeric: tabular-nums;
}

.pricing-group__fixed-price span {
  color: #6b7280;
  font-size: 0.75rem;
}

.pricing-group__image-prices {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.5rem;
}

.pricing-group__image-price {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding: 0.625rem;
  border: 1px solid rgba(196, 181, 253, 0.65);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.75);
}

.pricing-group__image-price span {
  color: #6b7280;
  font-size: 0.7rem;
}

.pricing-group__image-price strong {
  color: #111827;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}

.pricing-group__description {
  margin: 0.75rem 0 0;
  color: #4b5563;
  font-size: 0.75rem;
  line-height: 1.6;
}

.pricing-group__table {
  width: 100%;
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

@media (max-width: 760px) {
  .pricing-group__image-prices {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
