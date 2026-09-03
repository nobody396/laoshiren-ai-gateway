<template>
  <div class="docs-home">
    <section class="docs-home-hero">
      <p class="docs-home-kicker">DOCUMENTATION</p>
      <h1>老实人AI 开发文档</h1>
      <p class="docs-home-subtitle">一个 API，聚合并管理主流 AI 模型</p>
      <p class="docs-home-description">
        支持 OpenAI、Anthropic、Gemini 和 GPT-Image 兼容协议，从快速接入到完整 API 开发都可以在这里找到。
      </p>

      <div class="docs-home-actions">
        <router-link to="/docs/quickstart" class="docs-home-button docs-home-button-primary">
          快速开始 <span aria-hidden="true">→</span>
        </router-link>
        <router-link to="/docs/api-overview" class="docs-home-button docs-home-button-secondary">
          API 参考
        </router-link>
      </div>

      <label class="docs-home-search">
        <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="7" />
          <path d="m20 20-3.5-3.5" />
        </svg>
        <span class="sr-only">搜索文档</span>
        <input v-model.trim="query" type="search" placeholder="搜索文档…" />
        <kbd>⌘ K</kbd>
      </label>
    </section>

    <section v-if="query" class="docs-home-results" aria-live="polite">
      <div class="docs-home-section-title">
        <h2>搜索结果</h2>
        <span>{{ searchResults.length }} 篇</span>
      </div>
      <div v-if="searchResults.length" class="docs-home-result-list">
        <router-link v-for="item in searchResults" :key="item.slug" :to="`/docs/${item.slug}`">
          <div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.description }}</p>
          </div>
          <span aria-hidden="true">→</span>
        </router-link>
      </div>
      <div v-else class="docs-home-empty">没有找到对应文档，请换一个模型名、客户端或错误码。</div>
    </section>

    <template v-else>
      <section class="docs-home-protocols">
        <div class="docs-home-section-title docs-home-section-title-centered">
          <div>
            <h2>多种协议，统一接入</h2>
            <p>按照请求协议选择对应入口。Grok / xAI 模型使用 Responses 或 Chat Completions，不需要额外协议。</p>
          </div>
        </div>

        <div class="docs-home-protocol-shell">
          <div class="docs-home-protocol-tabs" role="tablist" aria-label="API 协议示例">
            <button
              v-for="protocol in protocols"
              :key="protocol.id"
              type="button"
              role="tab"
              :aria-selected="activeProtocolId === protocol.id"
              :class="{ active: activeProtocolId === protocol.id }"
              @click="activeProtocolId = protocol.id"
            >
              {{ protocol.label }}
            </button>
          </div>

          <div class="docs-home-code-panel">
            <div class="docs-home-code-toolbar">
              <div aria-hidden="true"><i></i><i></i><i></i></div>
              <span>{{ activeProtocol.endpoint }}</span>
              <button type="button" @click="copyProtocolExample">
                {{ copied ? '已复制' : '复制' }}
              </button>
            </div>
            <pre><code>{{ activeProtocol.example }}</code></pre>
          </div>
        </div>
      </section>

      <section class="docs-home-catalog">
        <div class="docs-home-section-title docs-home-section-title-centered">
          <div>
            <h2>选择文档板块</h2>
            <p>先选你要完成的任务，进入后再通过左侧目录继续浏览。</p>
          </div>
        </div>

        <div class="docs-home-card-grid">
          <router-link
            v-for="category in categoryCards"
            :key="category.title"
            :to="category.key === 'start' ? '/docs/quickstart' : `/docs/category/${category.key}`"
            class="docs-home-card"
          >
            <span class="docs-home-card-icon" v-html="category.icon"></span>
            <div>
              <h3>{{ category.title }}</h3>
              <p>{{ category.summary }}</p>
            </div>
            <span class="docs-home-card-arrow" aria-hidden="true">→</span>
          </router-link>
        </div>
      </section>

    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { docsConfig, type DocItem } from '@/docs/config'
import { useClipboard } from '@/composables/useClipboard'

const query = ref('')
const activeProtocolId = ref('responses')
const { copied, copyToClipboard } = useClipboard()

const protocols = [
  {
    id: 'responses',
    label: 'Responses',
    endpoint: 'POST /v1/responses',
    example: `curl 'https://api.laoshirenai.com/v1/responses' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "YOUR_MODEL_ID",
    "input": "只回复 OK"
  }'`,
  },
  {
    id: 'chat',
    label: 'Chat Completions',
    endpoint: 'POST /v1/chat/completions',
    example: `curl 'https://api.laoshirenai.com/v1/chat/completions' \\
  -H 'Authorization: Bearer YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role": "user", "content": "只回复 OK"}]
  }'`,
  },
  {
    id: 'messages',
    label: 'Messages',
    endpoint: 'POST /v1/messages',
    example: `curl 'https://api.laoshirenai.com/v1/messages' \\
  -H 'x-api-key: YOUR_API_KEY' \\
  -H 'anthropic-version: 2023-06-01' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "YOUR_MODEL_ID",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "只回复 OK"}]
  }'`,
  },
  {
    id: 'generate-content',
    label: 'GenerateContent',
    endpoint: 'POST /v1beta/models/{model}:generateContent',
    example: `curl 'https://api.laoshirenai.com/v1beta/models/YOUR_MODEL_ID:generateContent' \\
  -H 'x-goog-api-key: YOUR_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "contents": [{"parts": [{"text": "只回复 OK"}]}]
  }'`,
  },
  {
    id: 'images',
    label: 'Images',
    endpoint: 'POST /gpt-image/v1/images/generations',
    example: `curl 'https://api.laoshirenai.com/gpt-image/v1/images/generations' \\
  -H 'Authorization: Bearer YOUR_IMAGE_API_KEY' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "A clean product illustration"
  }'`,
  },
]

const activeProtocol = computed(() => protocols.find(item => item.id === activeProtocolId.value) ?? protocols[0])

const categoryMeta: Record<string, { summary: string; icon: string }> = {
  快速开始: {
    summary: '创建 API Key、查询模型并完成最小真实请求。',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="9"/><path d="m15.5 8.5-2 5-5 2 2-5 5-2Z"/></svg>',
  },
  工具集成: {
    summary: 'Claude Code、Codex、Grok Build、Gemini CLI、Kimi Code、OpenCode。',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M12 15h5"/></svg>',
  },
  'API 参考': {
    summary: 'Responses、Chat Completions、Messages、Gemini 与 Images。',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m8 4-5 8 5 8M16 4l5 8-5 8M14 3l-4 18"/></svg>',
  },
  模型目录: {
    summary: '按模型查看协议、可用分组、倍率和推荐工具。',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m12 3 8 4.5-8 4.5-8-4.5L12 3Z"/><path d="m4 12 8 4.5 8-4.5M4 16.5 12 21l8-4.5"/></svg>',
  },
}

const categoryCards = computed(() =>
  docsConfig.map(category => ({ ...category, ...categoryMeta[category.title] })),
)
const allDocs = computed<DocItem[]>(() => docsConfig.flatMap(category => category.items))
const searchResults = computed(() => {
  const needle = query.value.toLocaleLowerCase()
  if (!needle) return []
  return allDocs.value.filter(item => `${item.title} ${item.description}`.toLocaleLowerCase().includes(needle))
})

async function copyProtocolExample(): Promise<void> {
  await copyToClipboard(activeProtocol.value.example, '代码示例已复制')
}

function focusSearch(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    document.querySelector<HTMLInputElement>('.docs-home-search input')?.focus()
  }
}

onMounted(() => window.addEventListener('keydown', focusSearch))
onBeforeUnmount(() => window.removeEventListener('keydown', focusSearch))
</script>

<style scoped>
.docs-home {
  --docs-accent: #1267d6;
  --docs-accent-hover: #0d55b4;
  --docs-text: #18181b;
  --docs-muted: #6b7280;
  --docs-border: #e4e4e7;
  max-width: 64rem;
  margin: 0 auto;
  color: var(--docs-text);
  padding: 3.5rem 1.5rem 5rem;
}

.docs-home-hero { max-width: 52rem; margin: 0 auto 4rem; text-align: center; }
.docs-home-kicker {
  margin: 0 0 1rem;
  color: var(--docs-accent);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.16em;
}
.docs-home-hero h1 {
  margin: 0;
  color: var(--docs-text);
  font-size: clamp(2.8rem, 6vw, 4.2rem);
  font-weight: 800;
  letter-spacing: -0.045em;
  line-height: 1.06;
}
.docs-home-subtitle { margin: 1.25rem auto 0; color: var(--docs-text); font-size: clamp(1.2rem, 2.4vw, 1.55rem); font-weight: 650; }
.docs-home-description { max-width: 44rem; margin: 0.8rem auto 0; color: var(--docs-muted); font-size: 1rem; line-height: 1.7; }
.docs-home-actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.8rem; margin-top: 2rem; }
.docs-home-button {
  display: inline-flex;
  min-height: 2.8rem;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  border-radius: 0.55rem;
  font-size: 0.92rem;
  font-weight: 650;
  padding: 0 1.25rem;
  text-decoration: none;
  transition: border-color 160ms ease, background 160ms ease, transform 160ms ease;
}
.docs-home-button:hover { transform: translateY(-1px); }
.docs-home-button-primary { background: var(--docs-accent); color: white; }
.docs-home-button-primary:hover { background: var(--docs-accent-hover); }
.docs-home-button-secondary { border: 1px solid var(--docs-border); background: white; color: var(--docs-text); }
.docs-home-button-secondary:hover { border-color: var(--docs-accent); }

.docs-home-search {
  display: flex;
  width: min(100%, 32rem);
  align-items: center;
  gap: 0.7rem;
  margin: 1.4rem auto 0;
  border: 1px solid var(--docs-border);
  border-radius: 0.55rem;
  background: #f8f8f8;
  color: #71717a;
  padding: 0 0.8rem;
}
.docs-home-search:focus-within { border-color: var(--docs-accent); box-shadow: 0 0 0 3px rgb(18 103 214 / 12%); }
.docs-home-search svg { width: 1rem; flex: 0 0 auto; }
.docs-home-search input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: var(--docs-text); font-size: 0.9rem; padding: 0.75rem 0; }
.docs-home-search kbd { border: 1px solid #d4d4d8; border-radius: 0.3rem; background: white; font-size: 0.65rem; padding: 0.18rem 0.35rem; }

.docs-home-protocols, .docs-home-catalog, .docs-home-results { margin-top: 4rem; }
.docs-home-section-title { display: flex; align-items: end; justify-content: space-between; gap: 1rem; margin-bottom: 1.5rem; }
.docs-home-section-title-centered { justify-content: center; text-align: center; }
.docs-home-section-title h2 { margin: 0; font-size: clamp(1.65rem, 3vw, 2.1rem); letter-spacing: -0.025em; }
.docs-home-section-title p { margin: 0.55rem 0 0; color: var(--docs-muted); line-height: 1.6; }
.docs-home-section-title > span { color: var(--docs-muted); font-size: 0.82rem; }

.docs-home-protocol-shell { max-width: 52rem; margin: 0 auto; }
.docs-home-protocol-tabs {
  display: flex;
  overflow-x: auto;
  gap: 0.2rem;
  border: 1px solid var(--docs-border);
  border-bottom: 0;
  border-radius: 0.65rem 0.65rem 0 0;
  background: #fafafa;
  padding: 0.45rem;
}
.docs-home-protocol-tabs button { border: 0; border-radius: 0.4rem; background: transparent; color: #71717a; cursor: pointer; font-size: 0.82rem; font-weight: 600; padding: 0.55rem 0.85rem; white-space: nowrap; }
.docs-home-protocol-tabs button:hover { color: var(--docs-text); }
.docs-home-protocol-tabs button.active { background: white; color: var(--docs-accent); box-shadow: 0 1px 4px rgb(0 0 0 / 8%); }
.docs-home-code-panel { overflow: hidden; border-radius: 0 0 0.65rem 0.65rem; background: #17191d; color: #e5e7eb; }
.docs-home-code-toolbar {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  border-bottom: 1px solid #2b2e33;
  color: #a1a1aa;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.7rem;
  padding: 0.7rem 0.9rem;
}
.docs-home-code-toolbar > div { display: flex; gap: 0.35rem; }
.docs-home-code-toolbar i { width: 0.55rem; height: 0.55rem; border-radius: 50%; background: #4b4f56; }
.docs-home-code-toolbar i:first-child { background: #e0665c; }
.docs-home-code-toolbar i:nth-child(2) { background: #d8ae4d; }
.docs-home-code-toolbar i:nth-child(3) { background: #5aaa65; }
.docs-home-code-toolbar button { justify-self: end; border: 0; background: transparent; color: #aeb4bd; cursor: pointer; font: inherit; }
.docs-home-code-toolbar button:hover { color: white; }
.docs-home-code-panel pre { min-height: 17rem; margin: 0; overflow: auto; padding: 1.25rem 1.4rem; }
.docs-home-code-panel code { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 0.78rem; line-height: 1.7; }

.docs-home-card-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; max-width: 54rem; margin: 0 auto; }
.docs-home-card {
  position: relative;
  display: grid;
  grid-template-columns: 2.2rem 1fr auto;
  gap: 1rem;
  min-height: 9.5rem;
  align-items: start;
  border: 1px solid var(--docs-border);
  border-radius: 0.6rem;
  background: white;
  color: var(--docs-text);
  padding: 1.35rem;
  text-decoration: none;
  transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}
.docs-home-card:hover { transform: translateY(-1px); border-color: var(--docs-accent); box-shadow: 0 14px 34px rgb(31 31 32 / 8%); }
.docs-home-card-icon { display: block; width: 1.75rem; height: 1.75rem; color: var(--docs-accent); }
.docs-home-card-icon :deep(svg) { width: 100%; height: 100%; }
.docs-home-card h3 { margin: 0; font-size: 1.05rem; font-weight: 700; }
.docs-home-card p { margin: 0.45rem 0 0; color: var(--docs-muted); font-size: 0.9rem; line-height: 1.55; }
.docs-home-card-arrow { color: #a1a1aa; transition: color 160ms ease, transform 160ms ease; }
.docs-home-card:hover .docs-home-card-arrow { color: var(--docs-accent); transform: translateX(2px); }

.docs-home-result-list { display: grid; gap: 0.65rem; }
.docs-home-result-list > a { display: flex; align-items: center; justify-content: space-between; gap: 1.5rem; border: 1px solid var(--docs-border); border-radius: 0.55rem; color: inherit; padding: 1rem 1.2rem; text-decoration: none; }
.docs-home-result-list > a:hover { border-color: var(--docs-accent); }
.docs-home-result-list strong { font-size: 0.92rem; }
.docs-home-result-list p { margin: 0.25rem 0 0; color: var(--docs-muted); font-size: 0.8rem; }
.docs-home-empty { border: 1px dashed #c8c8cd; border-radius: 0.6rem; color: var(--docs-muted); padding: 2.5rem; text-align: center; }

.dark .docs-home { --docs-text: #f4f4f5; --docs-muted: #a1a1aa; --docs-border: #3f3f46; }
.dark .docs-home-button-secondary, .dark .docs-home-card, .dark .docs-home-search { background: #18181b; }
.dark .docs-home-search kbd { border-color: #52525b; background: #27272a; }
.dark .docs-home-protocol-tabs { background: #18181b; }
.dark .docs-home-protocol-tabs button.active { background: #27272a; }

@media (max-width: 760px) {
  .docs-home { padding: 2.8rem 1rem 4rem; }
  .docs-home-card-grid { grid-template-columns: 1fr; }
  .docs-home-code-toolbar { grid-template-columns: 1fr auto; }
  .docs-home-code-toolbar > span { display: none; }
}
@media (max-width: 480px) {
  .docs-home-search kbd { display: none; }
}
@media (prefers-reduced-motion: reduce) { .docs-home * { transition: none !important; } }
</style>
