<template>
  <div class="admin-chart-card card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.dashboard.tokenUsageTrend') }}
    </h3>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="normalizedTrendData.length > 0 && chartData" class="h-48">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { useChartPalette, useChartInk } from '@/utils/chartPalette'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { TrendDataPoint } from '@/types'
import { fillUsageTrendBuckets, type TrendGranularity } from '@/utils/trendBuckets'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const props = defineProps<{
  trendData: TrendDataPoint[]
  loading?: boolean
  startDate?: string
  granularity?: TrendGranularity
}>()

const normalizedTrendData = computed(() =>
  fillUsageTrendBuckets(props.trendData, props.startDate, props.granularity)
)

// The four series are fixed and meaningful (input / output / cache write /
// cache read), so they take the first four chart tokens by position rather
// than a lookup — series 1 is always the primary terracotta. Light and dark
// values both come from --chart-1…4, so the local dark-mode MutationObserver
// this component used to run is gone; useChartPalette owns that now, with one
// observer shared by every chart.
const seriesColors = useChartPalette()
// Area fills, at the same 12.5% the old `${color}20` hex-alpha suffix gave.
// That suffix cannot be used any more: the palette now returns rgb() strings,
// and "rgb(154 59 31)20" is not a color — the fill would silently vanish.
const fillColors = useChartPalette(0.125)
const ink = useChartInk()

const chartColors = computed(() => ({
  text: ink.value.text,
  grid: ink.value.grid,
  input: seriesColors.value[0],
  output: seriesColors.value[1],
  cacheCreation: seriesColors.value[2],
  cacheRead: seriesColors.value[3]
}))

const chartFills = computed(() => ({
  input: fillColors.value[0],
  output: fillColors.value[1],
  cacheCreation: fillColors.value[2],
  cacheRead: fillColors.value[3]
}))

const chartData = computed(() => {
  if (!normalizedTrendData.value.length) return null

  return {
    labels: normalizedTrendData.value.map((d) => d.date),
    datasets: [
      {
        label: 'Input',
        data: normalizedTrendData.value.map((d) => d.input_tokens),
        borderColor: chartColors.value.input,
        backgroundColor: chartFills.value.input,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Output',
        data: normalizedTrendData.value.map((d) => d.output_tokens),
        borderColor: chartColors.value.output,
        backgroundColor: chartFills.value.output,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Cache Creation',
        data: normalizedTrendData.value.map((d) => d.cache_creation_tokens),
        borderColor: chartColors.value.cacheCreation,
        backgroundColor: chartFills.value.cacheCreation,
        fill: true,
        tension: 0.3
      },
      {
        label: 'Cache Read',
        data: normalizedTrendData.value.map((d) => d.cache_read_tokens),
        borderColor: chartColors.value.cacheRead,
        backgroundColor: chartFills.value.cacheRead,
        fill: true,
        tension: 0.3
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        },
        footer: (tooltipItems: any) => {
          const dataIndex = tooltipItems[0]?.dataIndex
          if (dataIndex !== undefined && normalizedTrendData.value[dataIndex]) {
            const data = normalizedTrendData.value[dataIndex]
            return `Actual: $${formatCost(data.actual_cost)} | Standard: $${formatCost(data.cost)}`
          }
          return ''
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatCost = (value: number): string => {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}
</script>
