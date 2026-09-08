<template>
  <BaseDialog :show="show" title="一键配置" width="normal" @close="$emit('close')">
    <div class="space-y-5 text-sm">
      <p class="leading-6 text-gray-600 dark:text-gray-300">
        使用「{{ keyName }}」的 {{ groupName }} 分组。选择系统，再点击工具，命令会自动复制。
      </p>

      <div class="grid grid-cols-3 rounded-xl bg-gray-100 p-1 dark:bg-dark-700" role="tablist" aria-label="选择系统">
        <button
          v-for="system in systems"
          :key="system.id"
          type="button"
          role="tab"
          :aria-selected="os === system.id"
          :class="[
            'rounded-lg px-3 py-2 font-medium transition-colors',
            os === system.id
              ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-800 dark:text-primary-400'
              : 'text-gray-500 dark:text-gray-400'
          ]"
          @click="os = system.id"
        >
          {{ system.name }}
        </button>
      </div>

      <p v-if="loading" role="status" class="text-gray-500">正在检查当前分组…</p>
      <p v-else-if="error" role="alert" class="rounded-xl bg-red-50 p-3 text-red-700 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </p>
      <p v-else-if="options.length === 0" role="status" class="rounded-xl bg-amber-50 p-3 leading-6 text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
        当前分组在这个系统上还没有完成全部模型的一键配置验收，请先使用接入文档手动配置。
      </p>

      <button
        v-for="option in options"
        :key="option.client_id"
        type="button"
        class="flex w-full items-center gap-3 rounded-2xl border border-gray-200 bg-white p-4 text-left transition hover:border-primary-400 hover:shadow-sm disabled:cursor-wait disabled:opacity-60 dark:border-dark-600 dark:bg-dark-800"
        :disabled="copying"
        :data-client="option.client_id"
        @click="copy(option)"
      >
        <img :src="clientIcon(option)" alt="" class="h-10 w-10 rounded-lg object-contain" />
        <span class="min-w-0 flex-1">
          <strong class="block text-base text-gray-900 dark:text-white">{{ option.name }}</strong>
          <small class="mt-1 block text-gray-500 dark:text-gray-400">
            {{ copying ? '正在生成一次性命令…' : '点击后自动复制配置命令' }}
          </small>
        </span>
        <Icon :name="copied ? 'check' : 'copy'" size="sm" />
      </button>

      <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">
        命令十分钟内有效且只能使用一次，不包含 API Key。它会安装缺失的客户端、备份原配置，并接入当前分组的全部模型。
      </p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { resourcesAPI } from '@/api'
import type { ClientSetupOption, ClientSetupOS } from '@/api/resources'
import { useClipboard } from '@/composables/useClipboard'
import { buildClientAutoConfigCommand } from '@/utils/clientAutoConfig'
import type { ClientAutoConfigTarget } from '@/utils/clientAutoConfig'

const props = defineProps<{
  show: boolean
  apiKeyId: number
  keyName: string
  groupName: string
}>()

defineEmits<{ close: [] }>()

const systems: Array<{ id: ClientSetupOS; name: string }> = [
  { id: 'macos', name: 'macOS' },
  { id: 'linux', name: 'Linux' },
  { id: 'windows', name: 'Windows' }
]
const detectedOS: ClientSetupOS = typeof navigator !== 'undefined' && /windows/i.test(navigator.userAgent)
  ? 'windows'
  : typeof navigator !== 'undefined' && /linux/i.test(navigator.userAgent)
    ? 'linux'
    : 'macos'
const os = ref<ClientSetupOS>(detectedOS)
const options = ref<ClientSetupOption[]>([])
const loading = ref(false)
const copying = ref(false)
const error = ref('')
const { copied, copyToClipboard } = useClipboard()
let requestID = 0

async function load() {
  const current = ++requestID
  options.value = []
  error.value = ''
  if (!props.show || props.apiKeyId <= 0) return
  loading.value = true
  try {
    const result = await resourcesAPI.getClientSetupOptions(props.apiKeyId, os.value)
    if (current === requestID) options.value = result
  } catch (cause: any) {
    if (current === requestID) error.value = cause?.message || '读取一键配置选项失败'
  } finally {
    if (current === requestID) loading.value = false
  }
}

async function copy(option: ClientSetupOption) {
  if (copying.value) return
  const selectedOS = os.value
  copying.value = true
  error.value = ''
  try {
    const ticket = await resourcesAPI.createClientSetupTicketForOption(props.apiKeyId, option.client_id, selectedOS)
    const target = clientTarget(option)
    const command = buildClientAutoConfigCommand({
      target,
      ticket: ticket.ticket,
      isWindows: selectedOS === 'windows',
      installMissing: target !== 'zcode' && target !== 'workbuddy',
      installCodexApp: target === 'codex' && selectedOS !== 'linux'
    })
    await copyToClipboard(command, `${option.name} 一键配置命令已复制`)
  } catch (cause: any) {
    error.value = cause?.message || '生成一键配置命令失败，请重试'
  } finally {
    copying.value = false
  }
}

function clientIcon(option: ClientSetupOption): string {
  if (option.client_id === 'claude-code') return '/brand/client-tools/claude.svg'
  if (option.client_id === 'grok-build') return '/brand/client-tools/grok.svg'
  if (option.client_id === 'kimi-code') return '/brand/client-tools/kimi.svg'
  if (option.client_id === 'opencode') return '/brand/client-tools/opencode.svg'
  if (option.client_id === 'zcode') return '/tool-icons/zcode.png'
  if (option.client_id === 'workbuddy') return '/tool-icons/workbuddy.svg'
  return '/brand/client-tools/codex-light.png'
}

function clientTarget(option: ClientSetupOption): ClientAutoConfigTarget {
  if (option.client_id === 'claude-code') return 'claude'
  if (option.client_id === 'grok-build') return 'grok'
  if (option.client_id === 'kimi-code') return 'kimi'
  if (option.client_id === 'opencode') return 'opencode'
  if (option.client_id === 'zcode') return 'zcode'
  if (option.client_id === 'workbuddy') return 'workbuddy'
  return 'codex'
}

watch([() => props.show, () => props.apiKeyId, os], load, { immediate: true })
</script>
