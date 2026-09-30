<template>
  <BaseDialog :show="show" title="还原官方配置" width="normal" @close="$emit('close')">
    <div class="space-y-5 text-sm">
      <p class="leading-6 text-gray-600 dark:text-gray-300">
        先完全退出要还原的工具，再选择系统、点击工具复制命令，在本机终端执行。仅移除老实人AI一键配置或 CC Switch 导入的本站接入设置。
      </p>
      <div class="flex flex-wrap gap-2" role="group" aria-label="选择系统">
        <button v-for="system in systems" :key="system" type="button"
          :aria-pressed="os === system" :class="['btn', os === system ? 'btn-primary' : 'btn-secondary']"
          @click="os = system; resetCopy()">{{ system }}</button>
      </div>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2" role="group" aria-label="选择要还原的工具">
        <button v-for="tool in tools" :key="tool.id" type="button" class="btn btn-secondary flex items-center gap-3 text-left"
          :data-client="tool.id" @click="copyCommand(tool.id)">
          <CcsClientIcon :client="tool.id" class="h-10 w-10" />
          <span><span class="block">{{ tool.name }}</span><span class="block text-xs font-normal text-gray-500 dark:text-gray-400">复制还原命令</span></span>
        </button>
      </div>
      <template v-if="copyStatus !== 'idle'">
        <p role="status">{{ selectedName }} · {{ copyStatus === 'copying' ? '正在复制…' : copyStatus === 'failed' ? '自动复制失败，请手动复制下方命令' : '已复制，请在终端执行' }}</p>
        <textarea :value="command" readonly :aria-label="`${selectedName} 还原命令`"
          class="input w-full font-mono text-xs" rows="5" @focus="selectCommand" />
        <p class="text-xs text-gray-500">{{ os === 'Windows' ? '使用 Windows PowerShell 5.1 或 PowerShell 7，不要粘贴到 CMD' : '使用终端（bash / zsh）' }}</p>
        <p class="leading-6 text-gray-600 dark:text-gray-300">{{ client === 'codex' ? '完成后重启 Codex，必要时运行 codex login 或选择 Sign in with ChatGPT。TOML 注释和排版会重新整理。' : '完成后重启 Claude Code，必要时运行 /login，再用 /status 检查登录状态。仅修改用户 settings.json，不修改官方登录文件。' }}</p>
      </template>
      <ul class="list-disc space-y-2 pl-5 leading-6 text-gray-500 dark:text-gray-400">
        <li>自动备份需要修改的文件，再移除本站端点、模型设置及接入 Key；无需填写 API Key</li>
        <li>保留会话、MCP、权限、Hooks、其他设置与已有官方登录信息；遇到其他服务商或冲突配置会停止，不强行覆盖</li>
        <li>这不是恢复到出厂设置，也不会代办官方订阅；项目/组织设置、CC Switch、环境变量或命令行参数仍可能覆盖配置</li>
      </ul>
      <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
        其他接入工具：Grok Build、Gemini CLI、OpenCode、Kimi Code、ZCode、WorkBuddy，以及 CC Switch 的 OpenClaw、Hermes 暂未支持还原。移除本站 provider 与切回官方账号的方式不同，验证后再开放。
      </p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import CcsClientIcon from './CcsClientIcon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { buildOfficialRestoreCommand } from '@/utils/officialConfigRestore'
const props = defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()
const systems = ['macOS', 'Linux', 'Windows'] as const
const tools = [{ id: 'codex', name: 'Codex' }, { id: 'claude', name: 'Claude Code' }] as const
const os = ref<typeof systems[number]>(/windows/i.test(navigator.userAgent) ? 'Windows' : /linux/i.test(navigator.userAgent) ? 'Linux' : 'macOS')
const client = ref<'codex' | 'claude'>('codex')
const selectedName = computed(() => tools.find(tool => tool.id === client.value)!.name)
const copyStatus = ref<'idle' | 'copying' | 'copied' | 'failed'>('idle')
const command = computed(() => buildOfficialRestoreCommand(client.value, os.value === 'Windows'))
const { copyToClipboard } = useClipboard()
let copyRequest = 0
function resetCopy() {
  copyRequest++
  copyStatus.value = 'idle'
}
watch(() => props.show, resetCopy)
async function copyCommand(target: 'codex' | 'claude') {
  client.value = target
  const request = ++copyRequest
  copyStatus.value = 'copying'
  let copied = false
  try { copied = await copyToClipboard(command.value) } catch { /* Manual copy remains available. */ }
  if (request === copyRequest) copyStatus.value = copied ? 'copied' : 'failed'
}
function selectCommand(event: FocusEvent) {
  (event.target as HTMLTextAreaElement).select()
}
</script>
