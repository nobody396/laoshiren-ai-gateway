<template>
  <div class="docs-category-home">
    <router-link to="/docs" class="docs-category-back">← 返回文档首页</router-link>

    <header>
      <p>DOCUMENTATION SECTION</p>
      <h1>{{ category.title }}</h1>
      <span>{{ summary }}</span>
    </header>

    <section v-if="category.key === 'integrations'" class="docs-protocol-matrix" aria-labelledby="client-protocol-heading">
      <div class="docs-protocol-matrix__intro">
        <h2 id="client-protocol-heading">客户端原生协议</h2>
        <p>这里只表示客户端能否配置该协议；模型、分组、工具调用和流式续轮仍需在模型卡片中单独验收。</p>
      </div>

      <div class="docs-protocol-matrix__table-wrap">
        <table>
          <thead>
            <tr>
              <th>客户端</th>
              <th>Responses</th>
              <th>Chat Completions</th>
              <th>Anthropic Messages</th>
              <th>Gemini GenerateContent</th>
              <th>集成状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="client in clientProtocols" :key="client.slug">
              <td>
                <router-link :to="`/docs/${client.slug}`">{{ client.name }}</router-link>
                <small>{{ client.version }}</small>
              </td>
              <td v-for="protocol in protocolKeys" :key="protocol">
                <span :class="client[protocol] ? 'is-supported' : 'is-unsupported'">
                  {{ client[protocol] ? '支持' : '—' }}
                </span>
              </td>
              <td><span class="matrix-status" :class="`is-${client.status}`">{{ statusLabel(client.status) }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="docs-protocol-matrix__note">
        例：Codex 当前只接受 Responses；Kimi Code 和 OpenCode 虽能配置四类协议，也不代表任意模型在四类协议下都能完成 Agent 任务。
      </p>
    </section>

    <div class="docs-category-list">
      <router-link v-for="(item, index) in category.items" :key="item.slug" :to="`/docs/${item.slug}`">
        <small>{{ String(index + 1).padStart(2, '0') }}</small>
        <div>
          <h2>{{ item.title }}</h2>
          <p>{{ item.description }}</p>
        </div>
        <b aria-hidden="true">→</b>
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DocCategory } from '@/docs/config'
import { clientMatrix } from '@/generated/clientMatrix'
import { simpleClientGuideById } from '@/docs/guides/simpleClientGuides'

const props = defineProps<{ category: DocCategory }>()

const summaries: Record<string, string> = {
  start: '创建 API Key，查询当前 Key 可用模型，再完成一个最小真实请求。',
  api: 'Responses、Chat Completions、Messages、Gemini、Models 与 Images 的开发参考。',
  integrations: '查看全部客户端的原生协议、配置基线与导入开放状态。',
  models: '按逻辑模型查看支持协议、可用分组、倍率和推荐工具。',
}

type ProtocolKey = 'responses' | 'chat' | 'messages' | 'gemini'
type ClientProtocol = Record<ProtocolKey, boolean> & {
  name: string
  slug: string
  version: string
  status: 'ready' | 'prototype' | 'disabled'
}

const protocolKeys: ProtocolKey[] = ['responses', 'chat', 'messages', 'gemini']

const clientProtocols: ClientProtocol[] = clientMatrix
  .filter(client => simpleClientGuideById[client.id])
  .map(client => ({
    name: client.name,
    slug: client.slug,
    version: client.version,
    status: client.one_click_status,
    responses: client.protocols.includes('responses'),
    chat: client.protocols.includes('chat_completions'),
    messages: client.protocols.includes('messages'),
    gemini: client.protocols.includes('generate_content'),
  }))

function statusLabel(status: ClientProtocol['status']): string {
  if (status === 'ready') return '一键导入可用'
  if (status === 'prototype') return '手动配置'
  return '未开放'
}

const summary = computed(() => summaries[props.category.key] ?? props.category.title)
</script>

<style scoped>
.docs-category-home {
  max-width: 58rem;
  margin: 0 auto;
  padding: 3.5rem 1rem 5rem;
  color: #18181b;
}

.docs-category-back {
  color: #6b7280;
  font-size: 0.82rem;
  text-decoration: none;
}
.docs-category-back:hover { color: #1267d6; }

.docs-category-home header {
  max-width: 42rem;
  margin: 3rem 0 2.5rem;
}
.docs-category-home header p {
  margin: 0 0 0.8rem;
  color: #1267d6;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.7rem;
  font-weight: 700;
  letter-spacing: 0.15em;
}
.docs-category-home header h1 {
  margin: 0;
  font-size: clamp(2.4rem, 5vw, 3.6rem);
  font-weight: 800;
  letter-spacing: -0.045em;
  line-height: 1.08;
}
.docs-category-home header span {
  display: block;
  margin-top: 1rem;
  color: #6b7280;
  font-size: 1rem;
  line-height: 1.7;
}

.docs-category-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.docs-protocol-matrix {
  margin: 0 0 2.5rem;
  border: 1px solid #e4e4e7;
  border-radius: 0.75rem;
  background: #fff;
  overflow: hidden;
}
.docs-protocol-matrix__intro { padding: 1.25rem 1.35rem 1rem; }
.docs-protocol-matrix__intro h2 { margin: 0; font-size: 1.1rem; }
.docs-protocol-matrix__intro p,
.docs-protocol-matrix__note { margin: 0.5rem 0 0; color: #6b7280; font-size: 0.8rem; line-height: 1.6; }
.docs-protocol-matrix__table-wrap { overflow-x: auto; border-top: 1px solid #e4e4e7; border-bottom: 1px solid #e4e4e7; }
.docs-protocol-matrix table { width: 100%; min-width: 760px; border-collapse: collapse; font-size: 0.78rem; }
.docs-protocol-matrix th,
.docs-protocol-matrix td { padding: 0.75rem 0.8rem; border-bottom: 1px solid #f1f1f3; text-align: center; }
.docs-protocol-matrix th:first-child,
.docs-protocol-matrix td:first-child { text-align: left; }
.docs-protocol-matrix th { background: #fafafa; color: #52525b; font-weight: 700; }
.docs-protocol-matrix td a { color: #18181b; font-weight: 700; text-decoration: none; }
.docs-protocol-matrix td a:hover { color: #1267d6; }
.docs-protocol-matrix td small { display: block; margin-top: 0.2rem; color: #a1a1aa; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.docs-protocol-matrix .is-supported { display: inline-block; border-radius: 999px; background: #dcfce7; color: #166534; padding: 0.18rem 0.5rem; font-weight: 700; }
.docs-protocol-matrix .is-unsupported { color: #a1a1aa; }
.docs-protocol-matrix .matrix-status { display:inline-block;border-radius:999px;padding:.18rem .5rem;font-size:.67rem;font-weight:700;white-space:nowrap }
.docs-protocol-matrix .matrix-status.is-ready { background:#dcfce7;color:#166534 }
.docs-protocol-matrix .matrix-status.is-prototype { background:#fef3c7;color:#92400e }
.docs-protocol-matrix .matrix-status.is-disabled { background:#f4f4f5;color:#71717a }
.docs-protocol-matrix__note { margin: 0; padding: 0.8rem 1.35rem 1rem; }
.docs-category-list > a {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 1rem;
  min-height: 10.5rem;
  align-items: start;
  border: 1px solid #e4e4e7;
  border-radius: 0.6rem;
  background: white;
  color: inherit;
  padding: 1.35rem;
  text-decoration: none;
  transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}
.docs-category-list > a:hover {
  transform: translateY(-1px);
  border-color: #1267d6;
  box-shadow: 0 14px 34px rgb(31 31 32 / 8%);
}
.docs-category-list small {
  color: #1267d6;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.7rem;
  font-weight: 700;
}
.docs-category-list h2 { margin: 0; font-size: 1.05rem; }
.docs-category-list p { margin: 0.55rem 0 0; color: #6b7280; font-size: 0.86rem; line-height: 1.6; }
.docs-category-list b { color: #a1a1aa; font-weight: 500; }
.docs-category-list > a:hover b { color: #1267d6; }

.dark .docs-category-home { color: #f4f4f5; }
.dark .docs-category-home header span,
.dark .docs-category-list p,
.dark .docs-category-back { color: #a1a1aa; }
.dark .docs-category-list > a { border-color: #3f3f46; background: #18181b; }
.dark .docs-protocol-matrix { border-color: #3f3f46; background: #18181b; }
.dark .docs-protocol-matrix__table-wrap { border-color: #3f3f46; }
.dark .docs-protocol-matrix th { background: #27272a; color: #d4d4d8; }
.dark .docs-protocol-matrix th,
.dark .docs-protocol-matrix td { border-color: #27272a; }
.dark .docs-protocol-matrix td a { color: #f4f4f5; }
.dark .docs-protocol-matrix .is-supported { background: rgb(20 83 45 / 45%); color: #86efac; }

@media (max-width: 720px) {
  .docs-category-home { padding-top: 2.5rem; }
  .docs-category-list { grid-template-columns: 1fr; }
}

@media (prefers-reduced-motion: reduce) {
  .docs-category-list > a { transition: none; }
}
</style>
