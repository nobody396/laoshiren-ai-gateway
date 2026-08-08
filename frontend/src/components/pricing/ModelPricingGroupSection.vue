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

    <div v-if="group.models.length === 0" class="pricing-group__empty">
      {{ t('modelPricing.noModels') }}
    </div>

    <table v-else class="pricing-group__table">
      <thead>
        <tr>
          <th>{{ t('modelPricing.table.model') }}</th>
          <th>{{ t('modelPricing.table.input') }}</th>
          <th>{{ t('modelPricing.table.output') }}</th>
          <th>{{ t('modelPricing.table.cacheRead') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in group.models" :key="m.model">
          <td class="pricing-group__model">{{ m.model }}</td>
          <td>{{ formatPrice(m.input_price) }}</td>
          <td>{{ formatPrice(m.output_price) }}</td>
          <td>{{ formatPrice(m.cache_read_price) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'
import type { PublicPricingGroup } from '@/api/publicPricing'

defineProps<{
  group: PublicPricingGroup
}>()

const { t } = useI18n()

function formatPrice(v: number | null): string {
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
</style>
