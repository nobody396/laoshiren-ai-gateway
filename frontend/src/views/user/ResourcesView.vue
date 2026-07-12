<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p class="text-sm font-medium text-primary-600 dark:text-primary-400">下载资源</p>
            <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">AI 编码工具下载安装</h1>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">
              只保留普通用户最常用的 Windows 和 macOS 安装入口。Mac 用户按 Apple 芯片和 Intel 芯片选择，Windows 用户优先选 64 位安装包。
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
              原版 Codex App 在 API Key / 第三方 API 模式下，常见限制是插件入口提示需要 ChatGPT 登录、官方插件无法正常使用；原版会话列表通常只有归档，没有真正删除按钮。Codex++ 的思路是先保留 ChatGPT/OpenAI 官方登录态，再通过外部启动器注入增强功能，并可选把模型请求切到兼容 API。
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
            <p class="mt-1 leading-5">从 Codex++ 入口启动，解锁增强菜单、插件入口、会话删除和自定义接口注入。</p>
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
              <p v-if="resource.downloadHint" class="text-xs leading-5 text-gray-500 dark:text-dark-400">
                {{ resource.downloadHint }}
              </p>

              <div v-if="loadingFor(resource.downloadToolId)" class="space-y-2">
                <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
                <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
              </div>

              <div v-else-if="errorFor(resource.downloadToolId)" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
                {{ errorFor(resource.downloadToolId) }}
              </div>

              <div v-else-if="preferredAssets(resource.downloadToolId).length === 0" class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300">
                暂无适合普通用户的 Windows 或 macOS 安装包，请先使用官方安装页。
              </div>

              <div v-else class="grid grid-cols-1 gap-2">
                <button
                  v-for="asset in preferredAssets(resource.downloadToolId)"
                  :key="asset.id"
                  class="inline-flex min-h-[56px] items-center justify-between gap-3 rounded-lg border px-3 py-2 text-left text-sm transition-all duration-150"
                  :class="downloadButtonClass(resource.downloadToolId, asset)"
                  type="button"
                  :disabled="isDownloadPreparing(resource.downloadToolId, asset)"
                  @click="downloadCachedAsset(resource.downloadToolId, asset)"
                >
                  <span class="min-w-0">
                    <span class="block truncate font-medium text-gray-800 dark:text-dark-100">{{ formatAssetLabel(resource.downloadToolId, asset) }}</span>
                    <span class="block truncate text-xs text-gray-500 dark:text-dark-400">{{ formatAssetDescription(asset) }} · {{ formatBytes(asset.size) }}</span>
                  </span>
                  <span class="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                    <Icon
                      :name="downloadIcon(resource.downloadToolId, asset)"
                      size="sm"
                      :class="{ 'animate-spin': isDownloadPreparing(resource.downloadToolId, asset) }"
                    />
                  </span>
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
import { useAppStore } from '@/stores/app'

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
  downloadHint?: string
  verifyCommand?: string
  verifyText?: string
  primaryLink?: string
  docsLink?: string
  primaryAction?: string
  note: string
}

const { copyToClipboard } = useClipboard()
const appStore = useAppStore()
const manifests = ref<Partial<Record<DownloadToolID, DownloadManifest>>>({})
const loading = ref<Partial<Record<DownloadToolID, boolean>>>({})
const errors = ref<Partial<Record<DownloadToolID, string>>>({})
const downloadStates = ref<Record<string, 'preparing' | 'started' | 'error'>>({})

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
    description: '适合在本地终端中运行 Codex，也可以使用 Codex App 体验。下载区会每天检查新版本：官方仓库已有的包直接取官方源，Windows 桌面版取发布镜像。',
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
        label: 'Windows Codex App 一键更新',
        command: '$r=Invoke-RestMethod "https://api.github.com/repos/Wangnov/codex-app-mirror/releases/latest"; $a=$r.assets | Where-Object { $_.name -match "_x64__.*\\.Msix$" } | Select-Object -First 1; if (-not $a) { throw "未找到最新版 Windows x64 MSIX" }; $f="$env:USERPROFILE\\Downloads\\$($a.name)"; curl.exe -L $a.browser_download_url -o $f; if ($LASTEXITCODE -ne 0) { throw "下载失败" }; Add-AppxPackage -Path $f -DeferRegistrationWhenPackagesAreInUse; echo "OK-DONE $($r.tag_name)"',
        note: '复制整行到 PowerShell 执行；每次都会查询最新 Release，下载完成后安全更新；如果 Codex 正在运行，会等到应用退出后再完成。'
      },
      {
        label: 'npm 方式',
        command: 'npm install -g @openai/codex'
      }
    ],
    downloadToolId: 'codex',
    downloadTitle: '自动更新安装包',
    downloadHint: 'Windows 64 位提供 Codex App 的 MSIX 安装包；macOS 提供 OpenAI 官方 Codex 包。本站每天自动检查并缓存最新版。',
    verifyCommand: 'codex\ncodex app',
    primaryLink: 'https://github.com/openai/codex/releases/latest',
    docsLink: 'https://developers.openai.com/codex/cli',
    primaryAction: '查看官方 Release',
    note: '官方仓库有对应安装包时优先缓存官方版本；Windows Codex App 使用 Wangnov/codex-app-mirror 的最新 Release。'
  },
  {
    name: 'Codex++',
    badge: 'Codex App 增强启动器',
    description: '适合已经安装并登录 Codex App，但在 API Key / 第三方 API 模式下需要插件入口、会话删除、Markdown 导出、Timeline 和自定义接口注入能力的用户。使用时请从 Codex++ 入口启动，不要从原版 Codex App 入口打开。',
    icon: 'sparkles',
    commands: [],
    downloadToolId: 'codex-plus-plus',
    downloadTitle: '常用安装包',
    downloadHint: 'Windows 选 64 位安装包；Mac 按芯片选择，M1/M2/M3/M4 选 Apple 芯片，老款 Mac 选 Intel。',
    verifyText: '安装后先打开 Codex++ 管理工具检查状态，再从 Codex++ 入口启动 Codex App。顶部出现 Codex++ 菜单即表示增强注入成功。',
    primaryLink: 'https://github.com/BigPizzaV3/CodexPlusPlus/releases/latest',
    docsLink: 'https://github.com/BigPizzaV3/CodexPlusPlus',
    primaryAction: '查看 Codex++ Release',
    note: 'Codex++ 是第三方外部增强工具，不是 OpenAI 官方产品。它不修改 Codex App 原始安装文件；使用自定义接口注入前，建议确认 Codex App 已有官方登录态并保留 ~/.codex 配置备份。'
  },
  {
    name: 'Claude Desktop',
    badge: 'Claude 官方桌面 App',
    description: 'Claude 桌面端集成聊天、Cowork 和 Code 标签页，适合需要图形界面的用户。',
    icon: 'cube',
    commands: [],
    downloadToolId: 'claude-desktop',
    downloadTitle: '常用安装包',
    downloadHint: 'Windows 优先选择 64 位；macOS 官方包通常是通用版，Apple 芯片和 Intel Mac 都能用。',
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
    downloadTitle: '常用安装包',
    downloadHint: '只展示最适合普通用户的 Windows 安装版和 macOS 安装包，Linux、绿色版和校验文件不放在主列表里。',
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
  const selected = new Map<string, { asset: DownloadAsset; score: number }>()

  for (const asset of assets) {
    const option = installOptionFor(tool, asset)
    if (!option) continue
    const current = selected.get(option.key)
    if (!current || option.score < current.score || asset.name.localeCompare(current.asset.name) < 0) {
      selected.set(option.key, { asset, score: option.score })
    }
  }

  if (selected.has('macos-universal') && (selected.has('macos-arm64') || selected.has('macos-x64'))) {
    selected.delete('macos-universal')
  }

  return [...selected.values()]
    .sort((a, b) => a.score - b.score || a.asset.name.localeCompare(b.asset.name))
    .map((entry) => entry.asset)
}

function installOptionFor(tool: DownloadToolID, asset: DownloadAsset): { key: string; score: number } | null {
  const name = asset.name.toLowerCase()
  if (asset.platform === 'windows') {
    if (asset.arch === 'arm64') return null
    if (tool === 'cc-switch' && !name.endsWith('.msi')) return null
    if (tool === 'codex-plus-plus' && !name.endsWith('.exe')) return null
    if (tool === 'claude-desktop' && !name.endsWith('.exe')) return null
    if (tool === 'codex') {
      if (name.startsWith('codex-app-server-package')) return null
      if (!name.endsWith('.msix') && !name.endsWith('pc-windows-msvc.exe.zip')) return null
      return { key: 'windows-x64', score: name.endsWith('.msix') ? 1 : 10 }
    }
    return { key: 'windows-x64', score: 10 }
  }

  if (asset.platform === 'macos') {
    if (tool === 'cc-switch' && !name.endsWith('.dmg')) return null
    if (tool === 'codex-plus-plus' && !name.endsWith('.dmg')) return null
    if (tool === 'claude-desktop' && !name.endsWith('.dmg')) return null
    if (tool === 'codex') {
      if (name.startsWith('codex-app-server-package')) return null
      if (!name.endsWith('apple-darwin.tar.gz')) return null
    }
    if (asset.arch === 'arm64') return { key: 'macos-arm64', score: 20 }
    if (asset.arch === 'x64') return { key: 'macos-x64', score: 30 }
    return { key: 'macos-universal', score: 20 }
  }

  return null
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
  const key = downloadAssetKey(tool, asset)
  if (downloadStates.value[key] === 'preparing') return

  downloadStates.value = { ...downloadStates.value, [key]: 'preparing' }
  try {
    await resourcesAPI.downloadAsset(tool, asset)
    downloadStates.value = { ...downloadStates.value, [key]: 'started' }
    appStore.showInfo('下载已开始，请查看浏览器下载栏。')
    window.setTimeout(() => {
      if (downloadStates.value[key] === 'started') {
        const next = { ...downloadStates.value }
        delete next[key]
        downloadStates.value = next
      }
    }, 4000)
  } catch (error: any) {
    downloadStates.value = { ...downloadStates.value, [key]: 'error' }
    appStore.showError(error?.message || '创建下载链接失败，请稍后重试。')
    window.setTimeout(() => {
      if (downloadStates.value[key] === 'error') {
        const next = { ...downloadStates.value }
        delete next[key]
        downloadStates.value = next
      }
    }, 4000)
  }
}

function downloadAssetKey(tool: DownloadToolID, asset: DownloadAsset): string {
  return `${tool}:${asset.id}`
}

function isDownloadPreparing(tool: DownloadToolID, asset: DownloadAsset): boolean {
  return downloadStates.value[downloadAssetKey(tool, asset)] === 'preparing'
}

function downloadIcon(tool: DownloadToolID, asset: DownloadAsset): IconName {
  const state = downloadStates.value[downloadAssetKey(tool, asset)]
  if (state === 'preparing') return 'refresh'
  if (state === 'started') return 'checkCircle'
  if (state === 'error') return 'exclamationCircle'
  return 'download'
}

function downloadButtonClass(tool: DownloadToolID, asset: DownloadAsset): string {
  const state = downloadStates.value[downloadAssetKey(tool, asset)]
  if (state === 'preparing') {
    return 'cursor-wait border-primary-300 bg-primary-50 text-primary-700 dark:border-primary-800 dark:bg-primary-950/30 dark:text-primary-200'
  }
  if (state === 'started') {
    return 'border-emerald-300 bg-emerald-50 dark:border-emerald-900/70 dark:bg-emerald-950/30'
  }
  if (state === 'error') {
    return 'border-rose-300 bg-rose-50 dark:border-rose-900/70 dark:bg-rose-950/30'
  }
  return 'border-gray-200 hover:-translate-y-0.5 hover:bg-gray-50 hover:shadow-sm active:translate-y-0 dark:border-dark-700 dark:hover:bg-dark-800'
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
  if (tool === 'codex' && asset.name.toLowerCase().endsWith('.msix')) return 'Windows 64 位 Codex App'
  if (asset.platform === 'windows') return 'Windows 64 位安装包'
  if (asset.platform === 'macos' && asset.arch === 'arm64') return 'macOS Apple 芯片版'
  if (asset.platform === 'macos' && asset.arch === 'x64') return 'macOS Intel 芯片版'
  if (asset.platform === 'macos') return 'macOS 通用版'

  if (tool === 'codex') {
    return `Codex CLI ${asset.name}`
  }
  return asset.name
}

function formatAssetDescription(asset: DownloadAsset): string {
  if (asset.platform === 'windows') return '适合绝大多数 Windows 10/11 电脑'
  if (asset.platform === 'macos' && asset.arch === 'arm64') return 'M1/M2/M3/M4 等 Apple 芯片 Mac'
  if (asset.platform === 'macos' && asset.arch === 'x64') return 'Intel 芯片老款 Mac'
  if (asset.platform === 'macos') return 'Apple 芯片和 Intel Mac 都可用'
  return asset.name
}

onMounted(() => {
  downloadableTools.value.forEach((tool) => {
    void loadDownloads(tool)
  })
})
</script>
