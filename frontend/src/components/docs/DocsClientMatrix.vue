<template>
  <section class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-3xl font-bold text-gray-950 dark:text-white">客户端能力矩阵</h1>
      <p class="text-gray-600 dark:text-dark-300">
        每个客户端在四种协议上的验证状态、推理强度控制和一键配置状态。本页由能力矩阵直接生成，不手写维护。
        模型对应表见<router-link to="/docs/model-matrix" class="font-semibold text-primary-700 underline dark:text-primary-300">模型能力矩阵</router-link>。
      </p>
      <p class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm leading-6 text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
        <span class="font-bold text-green-500 dark:text-green-400">✓</span> 已验证，通过真实 Agent 闭环；
        <span class="font-bold text-red-500 dark:text-red-400">✗</span> 实测不支持；
        <span class="font-bold text-gray-400 dark:text-dark-500">○</span> 未验证，还没跑过闭环。
        只有带 ✓ 协议的客户端才会生成写入命令。
      </p>
    </header>

    <section class="space-y-3" aria-labelledby="client-matrix">
      <h2 id="client-matrix" class="sr-only">客户端能力矩阵</h2>
      <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
        <table class="w-full table-fixed divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs font-semibold text-gray-500 dark:bg-dark-900 dark:text-dark-400">
            <tr>
              <th scope="col" class="w-[16%] px-4 py-3">客户端</th>
              <th scope="col" class="w-[15%] px-3 py-3">版本</th>
              <th v-for="column in PROTOCOL_COLUMNS" :key="column.key" scope="col" class="w-[7%] px-1 py-3 text-center text-[10px] leading-tight tracking-tight">
                {{ column.label }}
              </th>
              <th scope="col" class="w-[29%] px-3 py-3">推理强度</th>
              <th scope="col" class="w-[12%] px-3 py-3">一键配置</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-950">
            <tr v-for="row in clientRows" :key="row.id">
              <th scope="row" class="px-4 py-2 text-left text-xs font-semibold leading-5 text-gray-900 dark:text-white">
                {{ row.name }}
              </th>
              <td class="px-3 py-2 font-mono text-[11px] leading-5 text-gray-600 dark:text-dark-300">{{ row.version }}</td>
              <td v-for="cell in row.protocols" :key="cell.name" class="px-1 py-2 text-center">
                <span
                  :class="['text-base font-bold', cell.class]"
                  :title="cell.title"
                  :aria-label="`${row.name} ${cell.name}: ${cell.title}`"
                >{{ cell.mark }}</span>
              </td>
              <td class="px-3 py-2 text-xs leading-5 text-gray-600 dark:text-dark-300">{{ row.reasoning }}</td>
              <td class="px-3 py-2 text-xs font-semibold leading-5" :class="row.oneClickClass">{{ row.oneClick }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { clientMatrix, type ClientMatrixEntry } from '@/generated/clientMatrix'
import { formatMatrixLevels } from '@/utils/matrixDisplay'

const PROTOCOL_COLUMNS = [
  { key: 'responses', label: 'Responses' },
  { key: 'chat_completions', label: 'Chat Completions' },
  { key: 'messages', label: 'Messages' },
  { key: 'generate_content', label: 'Generate Content' },
] as const

interface ProtocolCell {
  name: string
  mark: string
  title: string
  class: string
}

function protocolCell(client: ClientMatrixEntry, protocol: string): ProtocolCell {
  const detail = (client.protocol_details ?? []).find(row => row.protocol === protocol)
  if (detail?.support === 'supported') {
    return { name: protocol, mark: '✓', title: '已验证：真实 Agent 闭环通过', class: 'text-green-500 dark:text-green-400' }
  }
  if (detail?.support === 'unsupported') {
    return { name: protocol, mark: '✗', title: '实测不支持', class: 'text-red-500 dark:text-red-400' }
  }
  return { name: protocol, mark: '○', title: '未验证：还没跑过真实 Agent 闭环', class: 'text-gray-400 dark:text-dark-500' }
}

function clientReasoningText(client: ClientMatrixEntry): string {
  const reasoning = client.reasoning
  const parts: string[] = []
  if (reasoning.levels.length) parts.push(formatMatrixLevels(reasoning.levels))
  if (reasoning.modes?.length) parts.push(`模式：${reasoning.modes.join(' / ')}`)
  if (!parts.length) return '跟随模型目录'
  return parts.join('；')
}

const clientRows = computed(() => clientMatrix.map((client) => {
  const ready = client.one_click_status === 'ready'
  return {
    id: client.id,
    name: client.name,
    version: client.version,
    protocols: PROTOCOL_COLUMNS.map(column => protocolCell(client, column.key)),
    reasoning: clientReasoningText(client),
    oneClick: ready ? '可用' : '手动配置',
    oneClickClass: ready
      ? 'text-green-600 dark:text-green-400'
      : 'text-gray-500 dark:text-dark-400',
  }
}))
</script>
