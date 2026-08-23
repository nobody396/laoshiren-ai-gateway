<template>
  <section class="family-block">
    <h3>{{ family.display_name }}</h3>
    <div class="product-list">
      <article
        v-for="product in family.products"
        :key="product.code"
        class="product-row"
        :class="`service-status-tone-${displayStatus(product)}`"
        :data-status="displayStatus(product)"
      >
        <div class="product-main">
          <span class="product-dot" aria-hidden="true" />
          <div class="product-copy">
            <strong>{{ product.display_name }}</strong>
            <span>
              {{ productIsStale(product) ? '数据更新延迟' : 'HTTP' }}
              · 更新于 {{ formatServiceStatusTime(product.computed_at) }}
            </span>
          </div>
        </div>
        <span class="status-label" :class="{ stale: productIsStale(product) }">
          {{ statusLabels[displayStatus(product)] }}
        </span>
        <div v-if="affectedComponents(product).length" class="affected-components">
          <span>受影响范围</span>
          <strong v-for="component in affectedComponents(product)" :key="component.code">
            {{ component.display_name }}
          </strong>
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { ServiceStatusFamily, ServiceStatusProduct } from '@/api/serviceStatus'
import { formatServiceStatusTime, isServiceStatusProductStale, serviceStatusForPresentation, serviceStatusLabels as statusLabels } from '@/features/service-status/presentation'

const props = defineProps<{ family: ServiceStatusFamily; nowMs: number }>()
function productIsStale(product: ServiceStatusProduct): boolean {
  return isServiceStatusProductStale(product, props.nowMs)
}

function displayStatus(product: ServiceStatusProduct) {
  return serviceStatusForPresentation(product, props.nowMs)
}

function affectedComponents(product: ServiceStatusProduct) {
  if (displayStatus(product) !== 'partial_outage' || product.components.length < 2) return []
  return product.components.filter((component) => component.status !== 'operational')
}
</script>

<style scoped>
.family-block h3 { margin: 0 0 .6rem; color: rgb(var(--color-muted)); font-size: .78rem; font-weight: 750; letter-spacing: .12em; text-transform: uppercase; }
.product-list { border-top: 1px solid rgb(var(--color-gray-200)); }
.product-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: .8rem 1.5rem; border-bottom: 1px solid rgb(var(--color-gray-200)); padding: 1.25rem .25rem; }
.product-main { display: flex; min-width: 0; align-items: center; gap: .9rem; }
.product-dot { flex: 0 0 auto; width: .55rem; height: .55rem; border-radius: 999px; background: currentColor; }
.product-copy { display: grid; min-width: 0; gap: .25rem; }
.product-copy strong { overflow: hidden; color: rgb(var(--color-ink-deep)); font-size: 1rem; text-overflow: ellipsis; white-space: nowrap; }
.product-copy span { color: rgb(var(--color-muted)); font-size: .79rem; font-variant-numeric: tabular-nums; }
.status-label { border: 1px solid color-mix(in srgb, currentColor 25%, transparent); border-radius: 999px; padding: .35rem .7rem; background: color-mix(in srgb, currentColor 7%, transparent); font-size: .78rem; font-weight: 750; }
.status-label.stale { border-style: dashed; }
.affected-components { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; gap: .45rem .75rem; margin-left: 1.45rem; color: rgb(var(--color-muted)); font-size: .78rem; }
.affected-components strong { color: currentColor; }
@media (max-width: 720px) {
  .product-row { gap: .75rem; }
  .status-label { padding-inline: .55rem; }
}
</style>
