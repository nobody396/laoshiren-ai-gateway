<template>
  <div class="docs-terminal-command" :class="{ empty: !command }">
    <div class="terminal-bar">
      <span class="terminal-dots" aria-hidden="true"><i /><i /><i /></span>
      <b>{{ label }}</b>
      <span class="terminal-actions"><button v-if="runnable" type="button" class="run-button" :disabled="running || !command" @click="$emit('run')">{{ running ? '运行中…' : '运行' }}</button><button type="button" :disabled="!command" @click="copyCommand">{{ copied ? '已复制' : '复制' }}</button></span>
    </div>
    <pre><code>{{ displayCommand || command || emptyText }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const props = withDefaults(defineProps<{
  label?: string
  command?: string
  displayCommand?: string
  emptyText?: string
  runnable?: boolean
  running?: boolean
}>(), {
  label: 'Terminal',
  command: '',
  displayCommand: '',
  emptyText: '',
  runnable: false,
  running: false,
})

defineEmits<{ run: [] }>()

const copied = ref(false)

async function copyCommand(): Promise<void> {
  if (!props.command) return
  await navigator.clipboard.writeText(props.command)
  copied.value = true
  window.setTimeout(() => { copied.value = false }, 1600)
}
</script>

<style scoped>
.docs-terminal-command{width:100%;max-width:100%;min-width:0;box-sizing:border-box;overflow:hidden;border:1px solid #3f3f46;border-radius:.75rem;background:#1f1f1f}.terminal-bar{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:.7rem;border-bottom:1px solid #3f3f46;background:#292929;padding:.5rem .65rem}.terminal-bar b{min-width:0;overflow:hidden;color:#d4d4d8;font:600 .62rem ui-monospace,monospace;text-overflow:ellipsis;white-space:nowrap}.terminal-dots{display:flex;gap:.28rem}.terminal-dots i{width:.48rem;height:.48rem;border-radius:999px;background:#ef4444}.terminal-dots i:nth-child(2){background:#f59e0b}.terminal-dots i:nth-child(3){background:#22c55e}.terminal-actions{display:flex;gap:.35rem}pre{width:100%;max-width:100%;min-width:0;max-height:13rem;box-sizing:border-box;margin:0;overflow-x:auto;overflow-y:auto;padding:.9rem 1rem}code{display:block;width:max-content;min-width:100%;box-sizing:border-box;color:#f4f4f5;font-size:.66rem;line-height:1.7;white-space:pre;tab-size:2}button{border:0;border-radius:.42rem;background:#fff;color:#27272a;padding:.34rem .6rem;font-size:.62rem;font-weight:700;cursor:pointer}.run-button{background:#2383e2;color:#fff}button:disabled{cursor:not-allowed;background:#e4e4e7;color:#52525b;opacity:1}.empty code{color:#d4d4d8}
@media(max-width:700px){pre{padding:.75rem}}
</style>
