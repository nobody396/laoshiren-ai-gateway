<template>
  <span :class="badgeClass">
    <slot>{{ label }}</slot>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label?: string
  tone?: 'gray' | 'info' | 'success' | 'warning' | 'danger'
}>(), {
  label: '',
  tone: 'gray',
})

const badgeClass = computed(() => {
  switch (props.tone) {
    case 'info':
      return 'badge badge-primary'
    case 'success':
      return 'badge badge-success'
    case 'warning':
      return 'badge badge-warning'
    case 'danger':
      return 'badge badge-danger'
    default:
      return 'badge badge-gray'
  }
})
</script>
