<template>
  <section class="space-y-6">
    <header class="space-y-2">
      <h1 class="text-3xl font-bold text-gray-950 dark:text-white">模型能力矩阵</h1>
      <p class="text-gray-600 dark:text-dark-300">
        每个模型支持哪些协议、哪些推理强度。本页由能力矩阵直接生成，不手写维护。
        客户端对应表见<router-link to="/docs/client-matrix" class="font-semibold text-primary-700 underline dark:text-primary-300">客户端能力矩阵</router-link>。
      </p>
      <p class="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm leading-6 text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
        <span class="font-bold text-blue-600 dark:text-blue-400">★</span> 推荐，首选接入方式；
        <span class="font-bold text-green-500 dark:text-green-400">✓</span> 支持；
        <span class="font-bold text-red-500 dark:text-red-400">✗</span> 不支持。鼠标悬停符号可看具体原因。
      </p>
    </header>

    <section class="space-y-3" aria-labelledby="model-matrix">
      <h2 id="model-matrix" class="sr-only">模型能力矩阵</h2>
      <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
        <table class="w-full table-fixed divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-left text-xs font-semibold text-gray-500 dark:bg-dark-900 dark:text-dark-400">
            <tr>
              <th scope="col" class="w-[22%] px-3 py-3">模型</th>
              <th v-for="column in PROTOCOL_COLUMNS" :key="column.key" scope="col" class="w-[10%] px-1 py-3 text-center text-[10px] leading-tight tracking-tight">
                {{ column.label }}
            </th>
              <th scope="col" class="w-[28%] px-3 py-3">推理强度</th>
              <th scope="col" class="w-[10%] px-3 py-3 text-right">上下文</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-800 dark:bg-dark-950">
            <tr v-for="row in modelRows" :key="row.id">
              <th scope="row" class="whitespace-nowrap px-3 py-2 text-left font-mono text-xs font-semibold text-gray-900 dark:text-white">
                {{ row.id }}
              </th>
              <td v-for="cell in row.protocols" :key="cell.name" class="px-1.5 py-2 text-center">
                <span
                  :class="['text-base font-bold', cell.class]"
                  :title="cell.title"
                  :aria-label="`${row.id} ${cell.name}: ${cell.title}`"
                >{{ cell.mark }}</span>
              </td>
              <td class="whitespace-nowrap px-3 py-2 text-[11px] text-gray-600 dark:text-dark-300">
                {{ row.reasoning }}
              </td>
              <td class="whitespace-nowrap px-3 py-2 text-right font-mono text-[11px] text-gray-600 dark:text-dark-300">
                {{ row.context }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
        特殊档位：关闭 = 可以显式关掉推理，最快最省；常开 = 推理始终开启、不可调；自适应 = 模型按问题难度自行决定思考深度，无需配置；禁用 = 推理被完全关闭。
      </p>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { modelDocContracts } from '@/generated/modelDocContracts'
import { modelReasoningProfiles } from '@/generated/clientMatrix'
import { formatMatrixContext, formatMatrixLevels } from '@/utils/matrixDisplay'

const PROTOCOL_COLUMNS = [
  { key: 'responses', label: 'Responses' },
  { key: 'chat_completions', label: 'Chat Completions' },
  { key: 'messages', label: 'Messages' },
  { key: 'generate_content', label: 'Generate Content' },
] as const

const reasoningProfileById = Object.fromEntries(
  modelReasoningProfiles.map(profile => [profile.model_id, profile]),
)

interface ProtocolCell {
  name: string
  mark: string
  title: string
  class: string
}

const modelRows = computed(() => {
  return [...modelDocContracts]
    .sort((left, right) => left.model.id.localeCompare(right.model.id))
    .map((contract) => {
      const statusByName = new Map(
        (contract.protocols ?? []).map(row => [row.name, row.status] as const),
      )
      const protocols: ProtocolCell[] = PROTOCOL_COLUMNS.map((column) => {
        const status = statusByName.get(column.key)
        if (status === 'verified' && contract.recommended_protocol === column.key) {
          return { name: column.key, mark: '★', title: '推荐：该模型的首选接入协议', class: 'text-blue-600 dark:text-blue-400' }
        }
        if (status === 'verified') {
          return { name: column.key, mark: '✓', title: '支持：真实请求闭环通过', class: 'text-green-500 dark:text-green-400' }
        }
        if (status === 'unsupported' || status === 'blocked') {
          return { name: column.key, mark: '✗', title: '不支持：实测负向', class: 'text-red-500 dark:text-red-400' }
        }
        return { name: column.key, mark: '✗', title: '不支持：该模型未在此协议上开放', class: 'text-red-500 dark:text-red-400' }
      })
      return {
        id: contract.model.id,
        protocols,
        reasoning: formatMatrixLevels(reasoningProfileById[contract.model.id]?.model_levels ?? []),
        context: formatMatrixContext(contract.model.context_window),
      }
    })
})
</script>
