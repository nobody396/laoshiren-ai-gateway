<template>
  <BaseDialog :show="show" title="还原官方配置" width="normal" @close="$emit('close')">
    <div class="space-y-5 text-sm">
      <p class="leading-6 text-gray-600 dark:text-gray-300">
        先完全退出 Codex，再选择系统并复制命令，在本机终端执行。目前仅支持还原由老实人AI一键配置接入的 Codex。
      </p>
      <div class="flex gap-2" role="group" aria-label="选择系统">
        <button v-for="system in systems" :key="system" type="button"
          :aria-pressed="os === system" :class="['btn', os === system ? 'btn-primary' : 'btn-secondary']"
          @click="os = system; selected = false; copyFailed = false">{{ system }}</button>
      </div>
      <button type="button" class="btn btn-secondary w-full" data-client="codex" @click="selectCodex">
        Codex · 复制还原命令
      </button>
      <template v-if="selected">
        <p role="status">{{ copyFailed ? '自动复制失败，请手动复制下方命令' : '已复制，请在终端执行' }}</p>
        <textarea :value="command" readonly aria-label="Codex 还原命令"
          class="input w-full font-mono text-xs" rows="5" @focus="selectCommand" />
        <p class="text-xs text-gray-500">{{ os === 'Windows' ? '使用 Windows PowerShell 5.1 或 PowerShell 7，不要粘贴到 CMD' : '使用终端（bash / zsh）' }}</p>
      </template>
      <ul class="list-disc space-y-2 pl-5 leading-6 text-gray-500 dark:text-gray-400">
        <li>自动备份配置与需要修改的登录文件，再移除本站 provider、模型设置及接入 Key；无需填写 API Key</li>
        <li>保留会话、MCP、其他设置与已有官方登录信息；TOML 的注释和排版会重新整理</li>
        <li>完成后重启 Codex，必要时运行 codex login 或选择 Sign in with ChatGPT，登录自己的官方账号</li>
        <li>这不是恢复到出厂设置，也不会代办官方订阅；CC Switch、环境变量或命令行参数仍可能覆盖配置</li>
      </ul>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import { buildCodexRestoreCommand } from '@/utils/codexRestore'
const props = defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()
const systems = ['macOS', 'Linux', 'Windows'] as const
const os = ref<typeof systems[number]>(/windows/i.test(navigator.userAgent) ? 'Windows' : /linux/i.test(navigator.userAgent) ? 'Linux' : 'macOS')
const selected = ref(false)
const copyFailed = ref(false)
const command = computed(() => buildCodexRestoreCommand(os.value === 'Windows'))
const { copyToClipboard } = useClipboard()
watch(() => props.show, () => { selected.value = false; copyFailed.value = false })
async function selectCodex() {
  selected.value = true
  copyFailed.value = !(await copyToClipboard(command.value))
}
function selectCommand(event: FocusEvent) {
  (event.target as HTMLTextAreaElement).select()
}
</script>
