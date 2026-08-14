<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl pb-12 pt-2 md:pt-6">
      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="px-5 pb-8 pt-8 sm:px-8 sm:pt-10 lg:px-12">
          <div class="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <router-link :to="resourcesPagePath" class="inline-flex items-center gap-1.5 text-sm font-medium text-gray-500 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white">
                <Icon name="chevronLeft" size="xs" />返回下载资源
              </router-link>
              <h1 class="mt-4 font-serif text-3xl font-semibold tracking-tight text-gray-950 dark:text-white sm:text-4xl">版本状态</h1>
              <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300 sm:text-base">本站提供的安装版本，与官方最新版本一目了然。</p>
            </div>
            <button type="button" class="btn btn-secondary shrink-0 justify-center" :disabled="loading" @click="loadStatus">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
              {{ loading ? '正在检查' : '重新检查' }}
            </button>
          </div>

          <div class="mt-8 flex flex-wrap items-center gap-3 rounded-xl border px-4 py-3 text-sm" :class="healthBannerClass">
            <span class="flex h-7 w-7 items-center justify-center rounded-full bg-emerald-600 text-white"><Icon name="check" size="xs" /></span>
            <span class="font-semibold">{{ healthLabel }}</span>
            <span class="opacity-80">{{ healthDescription }}</span>
          </div>

          <div v-if="error" class="mt-6 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
            {{ error }}
          </div>

          <div class="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
            <article v-for="item in items" :key="item.tool" class="flex min-h-[330px] flex-col rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
              <div class="flex items-start justify-between gap-3">
                <div class="flex items-center gap-3">
                  <span class="flex h-10 w-10 items-center justify-center rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
                    <PlatformIcon v-if="platformFor(item.tool)" :platform="platformFor(item.tool)!" size="lg" />
                    <Icon v-else name="swap" size="sm" class="text-gray-600 dark:text-dark-200" />
                  </span>
                  <div>
                    <h2 class="font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
                    <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ sourceLabel(item) }}</p>
                  </div>
                </div>
                <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold" :class="stateClass(item.state)">{{ stateLabel(item.state) }}</span>
              </div>

              <dl class="mt-6 space-y-3">
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800/70">
                  <dt class="text-xs text-gray-500 dark:text-dark-400">本站提供</dt>
                  <dd class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">{{ cachedVersionLabel(item) }}</dd>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ cachedTimeLabel(item) }}</p>
                </div>
                <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
                  <dt class="text-xs text-gray-500 dark:text-dark-400">官方最新</dt>
                  <dd class="mt-1 font-mono text-base font-semibold text-gray-900 dark:text-white">{{ item.official_version || '暂未获取' }}</dd>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ formatTime(item.official_published_at, '官方发布时间暂不可用') }}</p>
                </div>
              </dl>

              <p class="mt-4 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ item.note }}</p>
              <a :href="item.official_url" target="_blank" rel="noopener noreferrer" class="mt-auto inline-flex items-center gap-1.5 pt-5 text-sm font-semibold text-primary-700 hover:text-primary-800 dark:text-primary-300">
                查看官方版本 <Icon name="externalLink" size="xs" />
              </a>
            </article>
          </div>

          <p v-if="previewMode" class="mt-5 text-center text-xs text-gray-400">当前为本地排版预览；正式页面会读取实时缓存和官方版本。</p>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { resourcesAPI, type DownloadVersionState, type DownloadVersionStatus } from '@/api/resources'

const route = useRoute()
const loading = ref(false)
const error = ref('')
const statuses = ref<DownloadVersionStatus[]>([])
const previewMode = computed(() => route.name === 'ResourceStatusPreview')
const resourcesPagePath = computed(() => previewMode.value ? '/resources-preview' : '/resources')

const previewItems: DownloadVersionStatus[] = [
  {
    tool: 'codex', name: 'Codex', cached_version: 'codex-app-26.727.51351', cached_updated_at: '2026-08-02T10:48:04Z',
    official_version: 'rust-v0.146.0', official_published_at: '2026-07-29T01:44:34Z', official_url: 'https://github.com/openai/codex/releases/latest',
    cache_mode: 'cached', state: 'cached', note: 'macOS 缓存 OpenAI 官方安装包；Windows 缓存第三方镜像的 MSIX，并通过本站 AppInstaller 提供更新。'
  },
  {
    tool: 'codex-plus-plus', name: 'Codex++', cached_version: 'v1.2.4', cached_updated_at: '2026-08-02T10:48:04Z',
    official_version: 'v1.2.4', official_published_at: '2026-07-29T01:44:34Z', official_url: 'https://github.com/BigPizzaV3/CodexPlusPlus/releases/latest',
    cache_mode: 'cached', state: 'current', note: '本站缓存 Codex++ 官方 Release 中的 Windows 和 macOS 安装包。'
  },
  {
    tool: 'claude-desktop', name: 'Claude Desktop', cached_version: '1.25927.0', cached_updated_at: '2026-08-02T10:48:04Z',
    official_version: '', official_published_at: '', official_url: 'https://claude.com/download',
    cache_mode: 'cached', state: 'cached', note: '本站缓存 Claude Desktop 官方安装包，并通过内容寻址静态路径分发。'
  },
  {
    tool: 'claude-code', name: 'Claude Code', cached_version: '', cached_updated_at: '',
    official_version: 'v2.1.220', official_published_at: '2026-07-25T01:35:55Z', official_url: 'https://github.com/anthropics/claude-code/releases/latest',
    cache_mode: 'npm-mirror', state: 'npm-mirror', note: '一键安装优先使用国内 npm 镜像，失败后才回退官方 npm；不走 Anthropic 安装器直连。'
  },
  {
    tool: 'grok-build', name: 'Grok Build', cached_version: '1.0.3', cached_updated_at: '2026-08-02T10:47:59Z',
    official_version: '', official_published_at: '', official_url: 'https://docs.x.ai/build/overview',
    cache_mode: 'cached', state: 'cached', note: '本站对比 xAI 两个官方制品源的 stable 版本，并缓存 Windows x64/ARM64 二进制。'
  },
  {
    tool: 'git-for-windows', name: 'Git for Windows', cached_version: 'v2.53.0.windows.2', cached_updated_at: '2026-08-02T10:47:59Z',
    official_version: 'v2.53.0.windows.2', official_published_at: '2026-07-31T02:25:44Z', official_url: 'https://github.com/git-for-windows/git/releases/latest',
    cache_mode: 'cached', state: 'current', note: '本站缓存官方 Windows x64/ARM64 安装版，供 Claude Code 自动准备 Git Bash。'
  },
  {
    tool: 'cc-switch', name: 'CC Switch', cached_version: 'v3.19.1', cached_updated_at: '2026-08-02T10:47:59Z',
    official_version: 'v3.19.1', official_published_at: '2026-08-01T02:25:44Z', official_url: 'https://github.com/farion1231/cc-switch/releases/latest',
    cache_mode: 'cached', state: 'current', note: '本站定时同步官方 Release，用户下载时不需要访问 GitHub。'
  }
]

const items = computed(() => statuses.value.length ? statuses.value : (previewMode.value ? previewItems : []))
const hasStatusWarning = computed(() => items.value.some((item) => ['update-available', 'cache-missing', 'official-unavailable', 'unknown'].includes(item.state)))
const healthLabel = computed(() => previewMode.value ? '版本状态排版预览' : (hasStatusWarning.value ? '部分版本正在检查' : '下载服务正常'))
const healthDescription = computed(() => previewMode.value
  ? '以下为示例数据；正式页面会读取本站缓存和官方版本。'
  : 'Claude Desktop 每 5 分钟检查版本；其他下载资源每 30 分钟检查。页面数据最多缓存 5 分钟。')
const healthBannerClass = computed(() => hasStatusWarning.value && !previewMode.value
  ? 'border-amber-200 bg-amber-50/70 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/25 dark:text-amber-200'
  : 'border-emerald-200 bg-emerald-50/70 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/25 dark:text-emerald-200')

function platformFor(tool: DownloadVersionStatus['tool']): 'openai' | 'anthropic' | undefined {
  if (tool === 'codex') return 'openai'
  if (tool === 'claude-code') return 'anthropic'
  return undefined
}

function stateLabel(state: DownloadVersionState): string {
  return ({
    current: '已是最新版', cached: '缓存可用', 'npm-mirror': '国内镜像',
    'update-available': '等待同步', 'cache-missing': '缓存准备中',
    'official-unavailable': '官方暂不可用', unknown: '检查中'
  } as Record<DownloadVersionState, string>)[state]
}

function stateClass(state: DownloadVersionState): string {
  if (state === 'current' || state === 'cached' || state === 'npm-mirror') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  if (state === 'update-available') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
}

function sourceLabel(item: DownloadVersionStatus): string {
  return item.cache_mode === 'npm-mirror' ? '国内 npm 镜像优先' : '本站高速缓存'
}

function cachedVersionLabel(item: DownloadVersionStatus): string {
  return item.cache_mode === 'npm-mirror' ? '国内 npm 镜像' : (item.cached_version || '正在准备')
}

function cachedTimeLabel(item: DownloadVersionStatus): string {
  return item.cache_mode === 'npm-mirror' ? '失败时自动回退官方 npm' : formatTime(item.cached_updated_at, '同步时间暂不可用')
}

function formatTime(value: string, fallback: string): string {
  if (!value) return fallback
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return fallback
  return new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(parsed)
}

async function loadStatus() {
  loading.value = true
  error.value = ''
  try {
    statuses.value = await resourcesAPI.getVersionStatus()
  } catch (cause: any) {
    if (!previewMode.value) error.value = cause?.message || '版本状态加载失败，请稍后重试。'
  } finally {
    loading.value = false
  }
}

onMounted(() => { void loadStatus() })
</script>
