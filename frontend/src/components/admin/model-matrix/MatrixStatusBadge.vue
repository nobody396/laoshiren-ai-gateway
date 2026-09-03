<template>
  <span :class="classes" :data-status="status">
    <span class="h-1.5 w-1.5 rounded-full bg-current" />
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MatrixStatus } from './matrix'

const props = defineProps<{ status: MatrixStatus; compact?: boolean }>()
const label = computed(() => ({ verified: '已验证', unsupported: '不支持', blocked: '待补证据' })[props.status])
const classes = computed(() => [
  'inline-flex items-center gap-1.5 rounded-full font-semibold ring-1 ring-inset',
  props.compact ? 'px-2 py-0.5 text-[11px]' : 'px-2.5 py-1 text-xs',
  props.status === 'verified' && 'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-950/40 dark:text-emerald-300',
  props.status === 'unsupported' && 'bg-slate-100 text-slate-600 ring-slate-500/20 dark:bg-slate-800 dark:text-slate-300',
  props.status === 'blocked' && 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-950/40 dark:text-amber-300',
])
</script>
