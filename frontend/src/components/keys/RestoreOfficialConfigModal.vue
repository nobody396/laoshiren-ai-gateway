<template>
  <BaseDialog :show="show" title="还原官方配置" width="normal" @close="$emit('close')">
    <div class="space-y-4 text-sm">
      <p class="text-gray-600 dark:text-gray-300">退出工具后，在终端粘贴执行。</p>
      <div class="flex flex-wrap gap-2" role="group" aria-label="选择系统">
        <button v-for="system in systems" :key="system" type="button" :disabled="copying"
          :aria-pressed="os === system" :class="['btn', os === system ? 'btn-primary' : 'btn-secondary']"
          @click="os = system">{{ system }}</button>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2" role="group" aria-label="选择要还原的工具">
        <button v-for="tool in tools" :key="tool.id" type="button" :disabled="copying"
          class="btn btn-secondary flex items-center justify-start gap-3 text-left"
          :data-client="tool.id" @click="copyCommand(tool.id)">
          <CcsClientIcon :client="tool.id" class="h-10 w-10" />
          <span><span class="block">{{ tool.name }}</span><span class="block text-xs font-normal text-gray-500 dark:text-gray-400">复制还原命令</span></span>
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import CcsClientIcon from './CcsClientIcon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { buildOfficialRestoreCommand } from '@/utils/officialConfigRestore'
defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()
const systems = ['macOS', 'Linux', 'Windows'] as const
const tools = [{ id: 'codex', name: 'Codex' }, { id: 'claude', name: 'Claude Code' }] as const
const os = ref<typeof systems[number]>(/windows/i.test(navigator.userAgent) ? 'Windows' : /linux/i.test(navigator.userAgent) ? 'Linux' : 'macOS')
const copying = ref(false)
const { copyToClipboard } = useClipboard()
async function copyCommand(target: 'codex' | 'claude') {
  if (copying.value) return
  copying.value = true
  try {
    await copyToClipboard(buildOfficialRestoreCommand(target, os.value === 'Windows'), '已复制到剪贴板，请到终端执行')
  } finally {
    copying.value = false
  }
}
</script>
