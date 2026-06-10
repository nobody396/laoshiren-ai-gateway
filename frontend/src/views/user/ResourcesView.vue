<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p class="text-sm font-medium text-primary-600 dark:text-primary-400">下载资源</p>
            <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">AI 编码工具下载安装</h1>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">
              先把 Claude Code、Codex、Codex++、Claude Desktop 和 CC Switch 的安装入口集中到这里，方便用户登录后直接安装和验证。
            </p>
          </div>
          <a
            href="/docs"
            class="inline-flex items-center gap-2 rounded-md border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-dark-700 dark:text-dark-200 dark:hover:bg-dark-800"
          >
            <Icon name="book" size="sm" />
            查看文档
          </a>
        </div>
      </section>

      <section class="rounded-lg border border-primary-200 bg-primary-50 p-5 dark:border-primary-900/60 dark:bg-primary-950/20">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">Codex App 用户先看这里</p>
            <h2 class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">只用 API Key 启动时，部分 Codex App 功能会受限</h2>
            <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-700 dark:text-dark-200">
              原版 Codex App 在 API Key / 中转模式下，常见限制是插件入口提示需要 ChatGPT 登录、官方插件无法正常使用；原版会话列表通常只有归档，没有真正删除按钮。Codex++ 的思路是先保留 ChatGPT/OpenAI 官方登录态，再通过外部启动器注入增强功能，并可选把模型请求切到兼容 API。
            </p>
          </div>
          <a
            href="https://github.com/BigPizzaV3/CodexPlusPlus"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-primary shrink-0 justify-center"
          >
            <Icon name="externalLink" size="sm" />
            查看 Codex++ 项目
          </a>
        </div>
        <div class="mt-4 grid grid-cols-1 gap-3 text-sm md:grid-cols-3">
          <div class="rounded-md bg-white p-3 text-gray-700 shadow-sm dark:bg-dark-900 dark:text-dark-200">
            <span class="font-medium text-gray-900 dark:text-white">原版 API Key 模式</span>
            <p class="mt-1 leading-5">适合本地调用模型，但插件入口和官方账号能力可能不可用。</p>
          </div>
          <div class="rounded-md bg-white p-3 text-gray-700 shadow-sm dark:bg-dark-900 dark:text-dark-200">
            <span class="font-medium text-gray-900 dark:text-white">官方登录态</span>
            <p class="mt-1 leading-5">先在 Codex App 里登录 ChatGPT/OpenAI 账号，保留插件和账号能力。</p>
          </div>
          <div class="rounded-md bg-white p-3 text-gray-700 shadow-sm dark:bg-dark-900 dark:text-dark-200">
            <span class="font-medium text-gray-900 dark:text-white">Codex++ 启动</span>
            <p class="mt-1 leading-5">从 Codex++ 入口启动，解锁增强菜单、插件入口、会话删除和中转注入。</p>
          </div>
        </div>
      </section>

      <section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <article
          v-for="resource in resources"
          :key="resource.name"
          class="flex min-h-[560px] flex-col rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="flex items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/40 dark:text-primary-300">
                <Icon :name="resource.icon" size="md" />
              </div>
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ resource.name }}</h2>
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ resource.badge }}</p>
              </div>
            </div>
          </div>

          <p class="mt-4 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ resource.description }}</p>

          <div class="mt-5 space-y-4">
            <div v-for="command in resource.commands" :key="command.label">
              <div class="mb-2 flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ command.label }}</p>
                <button class="btn btn-secondary btn-sm" type="button" @click="copyCommand(command.command)">
                  <Icon name="copy" size="xs" />
                  复制
                </button>
              </div>
              <pre class="overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs leading-5 text-gray-100"><code>{{ command.command }}</code></pre>
              <p v-if="command.note" class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ command.note }}</p>
            </div>

            <div v-if="resource.downloadToolId" class="space-y-3">
              <div class="flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ resource.downloadTitle || '本站缓存安装包' }}</p>
                <span
                  v-if="manifestFor(resource.downloadToolId)"
                  class="rounded-full bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300"
                >
                  {{ manifestFor(resource.downloadToolId)?.version }}
                </span>
              </div>

              <div v-if="loadingFor(resource.downloadToolId)" class="space-y-2">
                <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
                <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
                <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
              </div>

              <div v-else-if="errorFor(resource.downloadToolId)" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                {{ errorFor(resource.downloadToolId) }}
              </div>

              <div v-else class="grid grid-cols-1 gap-2">
                <button
                  v-for="asset in preferredAssets(resource.downloadToolId)"
                  :key="asset.id"
                  class="inline-flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 text-left text-sm hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-800"
                  type="button"
                  @click="downloadCachedAsset(resource.downloadToolId, asset)"
                >
                  <span class="min-w-0">
                    <span class="block truncate font-medium text-gray-800 dark:text-dark-100">{{ formatAssetLabel(resource.downloadToolId, asset) }}</span>
                    <span class="block text-xs text-gray-500 dark:text-dark-400">{{ formatBytes(asset.size) }}</span>
                  </span>
                  <Icon name="download" size="sm" class="flex-shrink-0" />
                </button>
              </div>

              <p v-if="manifestFor(resource.downloadToolId)" class="text-xs leading-5 text-gray-500 dark:text-dark-400">
                更新时间：{{ formatDate(manifestFor(resource.downloadToolId)?.updated_at || '') }}。SHA256 校验值已随缓存记录保存。
              </p>
            </div>

            <div>
              <div class="mb-2 flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-gray-800 dark:text-dark-100">安装后验证</p>
                <button
                  v-if="resource.verifyCommand"
                  class="btn btn-secondary btn-sm"
                  type="button"
                  @click="copyCommand(resource.verifyCommand)"
                >
                  <Icon name="copy" size="xs" />
                  复制
                </button>
              </div>
              <pre
                v-if="resource.verifyCommand"
                class="overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs leading-5 text-gray-100"
              ><code>{{ resource.verifyCommand }}</code></pre>
              <p v-else class="rounded-lg bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                {{ resource.verifyText }}
              </p>
            </div>
          </div>

          <div class="mt-auto pt-5">
            <div v-if="resource.primaryLink || resource.docsLink" class="flex flex-col gap-2 sm:flex-row">
              <a
                v-if="resource.primaryLink"
                :href="resource.primaryLink"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-primary flex-1 justify-center"
              >
                <Icon name="download" size="sm" />
                {{ resource.primaryAction }}
              </a>
              <a
                v-if="resource.docsLink"
                :href="resource.docsLink"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-secondary flex-1 justify-center"
              >
                <Icon name="externalLink" size="sm" />
                官方说明
              </a>
            </div>
            <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ resource.note }}</p>
          </div>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { resourcesAPI, type DownloadAsset, type DownloadManifest, type DownloadToolID } from '@/api/resources'

type IconName = InstanceType<typeof Icon>['$props']['name']

interface InstallCommand {
  label: string
  command: string
  note?: string
}

interface DownloadResource {
  name: string
  badge: string
  description: string
  icon: IconName
  commands: InstallCommand[]
  downloadToolId?: DownloadToolID
  downloadTitle?: string
  verifyCommand?: string
  verifyText?: string
  primaryLink?: string
  docsLink?: string
  primaryAction?: string
  note: string
}

const { copyToClipboard } = useClipboard()
const manifests = ref<Partial<Record<DownloadToolID, DownloadManifest>>>({})
const loading = ref<Partial<Record<DownloadToolID, boolean>>>({})
const errors = ref<Partial<Record<DownloadToolID, string>>>({})

const resources: DownloadResource[] = [
  {
    name: 'Claude Code',
    badge: 'Anthropic 官方编码 CLI',
    description: '适合在终端里直接让 Claude 阅读、修改和运行项目代码。推荐优先使用官方原生安装器，网络慢时使用 npm 方式兜底。',
    icon: 'terminal',
    commands: [
      {
        label: 'macOS / Linux / WSL',
        command: 'curl -fsSL https://claude.ai/install.sh | bash'
      },
      {
        label: 'Windows PowerShell',
        command: 'irm https://claude.ai/install.ps1 | iex'
      },
      {
        label: 'npm 兜底方式',
        command: 'npm install -g @anthropic-ai/claude-code',
        note: '如果 curl / PowerShell 安装非常慢或失败，可以先安装 Node.js 18+，再用这个命令安装。'
      }
    ],
    verifyCommand: 'claude --version\nclaude doctor',
    primaryLink: 'https://code.claude.com/docs/en/installation',
    docsLink: 'https://code.claude.com/docs/en/installation',
    primaryAction: '打开安装页',
    note: 'npm 方式安装的是同一个 Claude Code 原生二进制包；后续升级可运行 npm install -g @anthropic-ai/claude-code@latest。'
  },
  {
    name: 'Codex',
    badge: 'OpenAI 官方编码工具',
    description: '适合在本地终端中运行 Codex，也可以使用 Codex App 体验。本站会缓存官方 release 中的 CLI 和 App 包。',
    icon: 'cpu',
    commands: [
      {
        label: 'macOS / Linux',
        command: 'curl -fsSL https://chatgpt.com/codex/install.sh | sh'
      },
      {
        label: 'Windows PowerShell',
        command: 'powershell -ExecutionPolicy ByPass -c "irm https://chatgpt.com/codex/install.ps1 | iex"'
      },
      {
        label: 'npm 方式',
        command: 'npm install -g @openai/codex'
      }
    ],
    downloadToolId: 'codex',
    downloadTitle: '本站缓存 Codex 包',
    verifyCommand: 'codex\ncodex app',
    primaryLink: 'https://github.com/openai/codex/releases/latest',
    docsLink: 'https://developers.openai.com/codex/cli',
    primaryAction: '查看官方 Release',
    note: '用户不方便访问 GitHub 时，可以直接下载本站缓存的 Codex CLI 或 Codex App Server 包。'
  },
  {
    name: 'Codex++',
    badge: 'Codex App 增强启动器',
    description: '适合已经安装并登录 Codex App，但在 API Key / 中转模式下需要插件入口、会话删除、Markdown 导出、Timeline 和中转注入能力的用户。使用时请从 Codex++ 入口启动，不要从原版 Codex App 入口打开。',
    icon: 'sparkles',
    commands: [],
    downloadToolId: 'codex-plus-plus',
    downloadTitle: '本站缓存 Codex++ 安装包',
    verifyText: '安装后先打开 Codex++ 管理工具检查状态，再从 Codex++ 入口启动 Codex App。顶部出现 Codex++ 菜单即表示增强注入成功。',
    primaryLink: 'https://github.com/BigPizzaV3/CodexPlusPlus/releases/latest',
    docsLink: 'https://github.com/BigPizzaV3/CodexPlusPlus',
    primaryAction: '查看 Codex++ Release',
    note: 'Codex++ 是第三方外部增强工具，不是 OpenAI 官方产品。它不修改 Codex App 原始安装文件；使用中转注入前，建议确认 Codex App 已有官方登录态并保留 ~/.codex 配置备份。'
  },
  {
    name: 'Claude Desktop',
    badge: 'Claude 官方桌面 App',
    description: 'Claude 桌面端集成聊天、Cowork 和 Code 标签页，适合需要图形界面的用户。',
    icon: 'cube',
    commands: [],
    downloadToolId: 'claude-desktop',
    downloadTitle: '本站缓存桌面安装包',
    verifyText: '安装后打开 Claude Desktop，登录账号，并进入 Code 标签页确认可用。',
    primaryLink: 'https://claude.com/download',
    docsLink: 'https://support.claude.com/en/articles/10065433-install-claude-desktop',
    primaryAction: '打开官方下载页',
    note: 'Claude Desktop 安装包由本站定时缓存；桌面端不支持 Linux，Linux 用户使用 Claude Code CLI。'
  },
  {
    name: 'CC Switch',
    badge: 'AI 编码工具切换器',
    description: '用于集中管理 Claude Code、Codex 等 AI 编码工具的 provider、配置和切换入口。',
    icon: 'swap',
    commands: [],
    downloadToolId: 'cc-switch',
    downloadTitle: '本站缓存安装包',
    verifyText: '安装完成后打开 CC Switch 应用，确认能看到 Claude Code 和 Codex 入口。',
    docsLink: 'https://ccswitch.ai/',
    note: 'CC Switch 安装包由本站定时缓存，用户下载时不需要访问 GitHub。'
  }
]

const downloadableTools = computed(() => {
  return resources.map((resource) => resource.downloadToolId).filter(Boolean) as DownloadToolID[]
})

function manifestFor(tool: DownloadToolID): DownloadManifest | undefined {
  return manifests.value[tool]
}

function loadingFor(tool: DownloadToolID): boolean {
  return !!loading.value[tool]
}

function errorFor(tool: DownloadToolID): string {
  return errors.value[tool] || ''
}

function preferredAssets(tool: DownloadToolID): DownloadAsset[] {
  const assets = manifests.value[tool]?.assets ?? []
  const score = (asset: DownloadAsset) => {
    const name = asset.name.toLowerCase()
    if (tool === 'claude-desktop') {
      if (asset.platform === 'macos' && name.endsWith('.dmg')) return 10
      if (asset.platform === 'windows' && asset.arch === 'x64') return 20
      if (asset.platform === 'windows' && asset.arch === 'arm64') return 30
    }
    if (tool === 'codex') {
      if (name.startsWith('codex-app-server-package') && asset.platform === 'macos' && asset.arch === 'arm64') return 10
      if (name.startsWith('codex-app-server-package') && asset.platform === 'macos' && asset.arch === 'x64') return 20
      if (name.startsWith('codex-app-server-package') && asset.platform === 'windows' && asset.arch === 'x64') return 30
      if (name.startsWith('codex-app-server-package') && asset.platform === 'linux' && asset.arch === 'x64') return 40
      if (!name.startsWith('codex-app-server-package') && asset.platform === 'macos' && asset.arch === 'arm64') return 50
      if (!name.startsWith('codex-app-server-package') && asset.platform === 'macos' && asset.arch === 'x64') return 60
      if (!name.startsWith('codex-app-server-package') && asset.platform === 'windows' && asset.arch === 'x64') return 70
      if (!name.startsWith('codex-app-server-package') && asset.platform === 'linux' && asset.arch === 'x64') return 80
    }
    if (tool === 'cc-switch') {
      if (asset.platform === 'windows' && name.endsWith('.msi')) return 10
      if (asset.platform === 'windows' && name.includes('portable')) return 20
      if (asset.platform === 'macos' && name.endsWith('.dmg')) return 30
      if (asset.platform === 'macos' && name.endsWith('.zip')) return 40
      if (asset.platform === 'linux' && name.endsWith('.appimage') && asset.arch === 'x64') return 50
      if (asset.platform === 'linux' && name.endsWith('.deb') && asset.arch === 'x64') return 60
      if (asset.platform === 'linux' && name.endsWith('.rpm') && asset.arch === 'x64') return 70
    }
    if (tool === 'codex-plus-plus') {
      if (asset.platform === 'windows' && name.endsWith('.exe')) return 10
      if (asset.platform === 'macos' && asset.arch === 'arm64') return 20
      if (asset.platform === 'macos' && asset.arch === 'x64') return 30
    }
    return 100
  }
  return [...assets].sort((a, b) => score(a) - score(b) || a.name.localeCompare(b.name))
}

async function copyCommand(command: string) {
  await copyToClipboard(command, '安装命令已复制')
}

async function loadDownloads(tool: DownloadToolID) {
  loading.value = { ...loading.value, [tool]: true }
  errors.value = { ...errors.value, [tool]: '' }
  try {
    const manifest = await resourcesAPI.getDownloads(tool)
    manifests.value = { ...manifests.value, [tool]: manifest }
  } catch (error: any) {
    errors.value = {
      ...errors.value,
      [tool]: error?.message || '安装包正在同步，请稍后刷新。'
    }
  } finally {
    loading.value = { ...loading.value, [tool]: false }
  }
}

async function downloadCachedAsset(tool: DownloadToolID, asset: DownloadAsset) {
  await resourcesAPI.downloadAsset(tool, asset)
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`
}

function formatDate(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatAssetLabel(tool: DownloadToolID, asset: DownloadAsset): string {
  const name = asset.name.toLowerCase()
  if (tool === 'claude-desktop') {
    if (asset.platform === 'macos') return 'macOS 通用版'
    if (asset.platform === 'windows' && asset.arch === 'arm64') return 'Windows ARM64 安装包'
    if (asset.platform === 'windows') return 'Windows x64 安装包'
  }
  if (tool === 'codex') {
    const prefix = name.startsWith('codex-app-server-package') ? 'Codex App' : 'Codex CLI'
    if (asset.platform === 'macos' && asset.arch === 'arm64') return `${prefix} macOS Apple Silicon`
    if (asset.platform === 'macos') return `${prefix} macOS Intel`
    if (asset.platform === 'windows' && asset.arch === 'arm64') return `${prefix} Windows ARM64`
    if (asset.platform === 'windows') return `${prefix} Windows x64`
    if (asset.platform === 'linux' && asset.arch === 'arm64') return `${prefix} Linux ARM64`
    if (asset.platform === 'linux') return `${prefix} Linux x64`
  }
  if (tool === 'cc-switch') {
    if (asset.platform === 'windows' && name.endsWith('.msi')) return 'Windows 安装版'
    if (asset.platform === 'windows' && name.includes('portable')) return 'Windows 绿色版'
    if (asset.platform === 'macos' && name.endsWith('.dmg')) return 'macOS DMG'
    if (asset.platform === 'macos' && name.endsWith('.zip')) return 'macOS ZIP'
    if (asset.platform === 'linux' && name.endsWith('.appimage')) return `Linux AppImage ${asset.arch}`
    if (asset.platform === 'linux' && name.endsWith('.deb')) return `Linux deb ${asset.arch}`
    if (asset.platform === 'linux' && name.endsWith('.rpm')) return `Linux rpm ${asset.arch}`
  }
  if (tool === 'codex-plus-plus') {
    if (asset.platform === 'windows') return 'Windows x64 安装包'
    if (asset.platform === 'macos' && asset.arch === 'arm64') return 'macOS Apple Silicon DMG'
    if (asset.platform === 'macos') return 'macOS Intel DMG'
  }
  return asset.name
}

onMounted(() => {
  downloadableTools.value.forEach((tool) => {
    void loadDownloads(tool)
  })
})
</script>
