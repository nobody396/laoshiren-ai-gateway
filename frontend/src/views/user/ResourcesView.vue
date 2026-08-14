<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl pb-12 pt-2 md:pt-6">
      <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="px-5 pb-6 pt-8 text-center sm:px-8 sm:pb-8 sm:pt-10 lg:px-12 lg:pb-10">
          <div class="mx-auto max-w-3xl">
            <h1 class="font-serif text-3xl font-semibold tracking-tight text-gray-950 dark:text-white sm:text-4xl">
              你准备怎么样使用老实人AI？
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-500 dark:text-dark-300 sm:text-base">
              选择一种方式，3 分钟完成安装和配置。
            </p>
          </div>

          <div class="mt-8 grid grid-cols-1 gap-3 text-left sm:grid-cols-2 xl:grid-cols-4">
            <button
              v-for="setup in quickSetups"
              :key="setup.id"
              type="button"
              class="group relative flex min-h-[92px] items-center gap-3 rounded-xl border px-4 py-4 transition-all duration-200"
              :class="selectedSetup === setup.id
                ? 'border-primary-600 bg-primary-50/60 shadow-[0_10px_30px_rgba(156,65,38,0.08)] dark:border-primary-500 dark:bg-primary-950/20'
                : 'border-gray-200 bg-white hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-sm dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600'"
              :aria-pressed="selectedSetup === setup.id"
              @click="selectedSetup = setup.id"
            >
              <span
                class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border"
                :class="selectedSetup === setup.id
                  ? 'border-primary-200 bg-white text-primary-700 dark:border-primary-900 dark:bg-dark-900 dark:text-primary-300'
                  : 'border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300'"
              >
                <PlatformIcon v-if="setup.platform" :platform="setup.platform" size="lg" />
                <Icon v-else :name="setup.icon" size="sm" />
              </span>
              <span class="min-w-0">
                <span class="block font-semibold text-gray-900 dark:text-white">{{ setup.title }}</span>
                <span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-400">{{ setup.description }}</span>
              </span>
              <span
                v-if="selectedSetup === setup.id"
                class="ml-auto flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-600 text-white"
              >
                <Icon name="check" size="xs" />
              </span>
            </button>
          </div>

          <div class="mt-6 rounded-xl border border-gray-200 bg-gray-950 p-2 text-left shadow-inner dark:border-dark-700">
            <div class="flex min-w-0 items-center gap-2">
              <span class="hidden shrink-0 pl-3 text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 sm:inline">Terminal</span>
              <div class="min-w-0 flex-1 overflow-x-auto px-2 py-3 [scrollbar-width:thin]">
                <code class="block whitespace-nowrap font-mono text-xs leading-5 text-gray-100 sm:text-[13px]">{{ selectedCommand }}</code>
              </div>
              <button
                type="button"
                class="inline-flex h-11 shrink-0 items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 text-sm font-semibold text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="selectedState?.loading || detectedOS === 'unsupported'"
                @click="prepareAndCopySetup(selectedSetup)"
              >
                <Icon :name="selectedState?.loading ? 'refresh' : 'copy'" size="sm" :class="{ 'animate-spin': selectedState?.loading }" />
                <span class="hidden sm:inline">{{ selectedActionLabel }}</span>
              </button>
            </div>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-3 text-left sm:grid-cols-3">
            <div v-for="(step, index) in quickSteps" :key="step.title" class="flex items-center gap-3 rounded-lg px-2 py-2">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-950/50 dark:text-primary-300">{{ index + 1 }}</span>
              <span>
                <span class="block text-sm font-medium text-gray-800 dark:text-dark-100">{{ step.title }}</span>
                <span class="mt-0.5 flex flex-wrap items-center gap-1 text-xs text-gray-500 dark:text-dark-400">
                  <template v-for="(keyLabel, keyIndex) in step.keys" :key="keyLabel">
                    <kbd class="rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 font-sans text-[11px] font-semibold text-gray-700 shadow-sm dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200">{{ keyLabel }}</kbd>
                    <span v-if="keyIndex < step.keys.length - 1" aria-hidden="true">+</span>
                  </template>
                  <span>{{ step.description }}</span>
                </span>
              </span>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span>已识别：{{ detectedOSLabel }}</span>
            <template v-if="selectedState?.groupName">
              <span aria-hidden="true">·</span>
              <span>已准备 {{ selectedState.keyName }} · {{ selectedState.groupName }}</span>
            </template>
          </div>

        </div>

        <details class="group border-t border-gray-200 dark:border-dark-700">
          <summary class="flex cursor-pointer list-none items-center justify-between gap-4 px-5 py-5 text-left sm:px-8 lg:px-12">
            <span>
              <span class="block text-sm font-semibold text-gray-900 dark:text-white">其他工具</span>
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">Claude Desktop 需单独配置 · Codex++ 为可选增强</span>
            </span>
            <Icon name="chevronDown" size="sm" class="text-gray-400 transition-transform duration-200 group-open:rotate-180" />
          </summary>

          <div class="border-t border-gray-100 bg-gray-50/70 px-5 py-6 dark:border-dark-800 dark:bg-dark-950/30 sm:px-8 lg:px-12">
            <section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <article
                v-for="resource in advancedResources"
                :key="resource.name"
                class="flex flex-col rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
              >
                <div class="flex items-start gap-3">
                  <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-950/40 dark:text-primary-300">
                    <Icon :name="resource.icon" size="md" />
                  </div>
                  <div>
                    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ resource.name }}</h2>
                    <p class="text-xs text-gray-500 dark:text-dark-400">{{ resource.badge }}</p>
                  </div>
                </div>

                <p class="mt-4 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ resource.description }}</p>

                <div v-if="resource.downloadToolId" class="mt-5 rounded-lg bg-gray-950 p-2">
                  <div class="flex min-w-0 items-center gap-2">
                    <div class="min-w-0 flex-1 overflow-x-auto px-2 py-2 [scrollbar-width:thin]">
                      <code class="block whitespace-nowrap font-mono text-xs text-gray-100">{{ advancedInstallCommand(resource.downloadToolId) }}</code>
                    </div>
                    <button
                      type="button"
                      class="inline-flex h-10 shrink-0 items-center justify-center gap-2 rounded-md bg-primary-600 px-3 text-xs font-semibold text-white hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="loadingFor(resource.downloadToolId) || !!errorFor(resource.downloadToolId) || preferredAssets(resource.downloadToolId).length === 0 || advancedInstallState[resource.downloadToolId]?.loading"
                      @click="prepareAdvancedInstallCommand(resource.downloadToolId)"
                    >
                      <Icon :name="advancedInstallState[resource.downloadToolId]?.loading ? 'refresh' : 'copy'" size="xs" :class="{ 'animate-spin': advancedInstallState[resource.downloadToolId]?.loading }" />
                      复制一键安装命令
                    </button>
                  </div>
                </div>

                <div class="mt-5 space-y-4">
                  <div v-for="command in resource.commands" :key="command.label">
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <p class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ command.label }}</p>
                      <button class="btn btn-secondary btn-sm" type="button" @click="copyCommand(command.command)">
                        <Icon name="copy" size="xs" />复制
                      </button>
                    </div>
                    <pre class="overflow-x-auto whitespace-nowrap rounded-lg bg-gray-950 p-3 text-xs leading-5 text-gray-100"><code>{{ command.command }}</code></pre>
                    <p v-if="command.note" class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ command.note }}</p>
                  </div>

                  <div v-if="resource.downloadToolId" class="space-y-3">
                    <div class="flex items-center justify-between gap-2">
                      <p class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ resource.downloadTitle || '本站缓存安装包' }}</p>
                      <span v-if="manifestFor(resource.downloadToolId)" class="rounded-full bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">{{ manifestFor(resource.downloadToolId)?.version }}</span>
                    </div>
                    <p v-if="resource.downloadHint" class="text-xs leading-5 text-gray-500 dark:text-dark-400">{{ resource.downloadHint }}</p>
                    <div v-if="loadingFor(resource.downloadToolId)" class="space-y-2">
                      <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"></div>
                    </div>
                    <div v-else-if="errorFor(resource.downloadToolId)" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">{{ errorFor(resource.downloadToolId) }}</div>
                    <div v-else-if="preferredAssets(resource.downloadToolId).length === 0" class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300">暂无适合普通用户的安装包，请使用官方安装页。</div>
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
                        <span class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                          <Icon :name="downloadIcon(resource.downloadToolId, asset)" size="sm" :class="{ 'animate-spin': isDownloadPreparing(resource.downloadToolId, asset) }" />
                        </span>
                      </button>
                    </div>
                  </div>

                </div>

                <div class="mt-auto pt-5">
                  <div v-if="resource.primaryLink || resource.docsLink" class="flex flex-col gap-2 sm:flex-row">
                    <a v-if="resource.primaryLink" :href="resource.primaryLink" target="_blank" rel="noopener noreferrer" class="btn btn-secondary flex-1 justify-center">
                      <Icon name="download" size="sm" />{{ resource.primaryAction }}
                    </a>
                    <a v-if="resource.docsLink" :href="resource.docsLink" target="_blank" rel="noopener noreferrer" class="btn btn-secondary flex-1 justify-center">
                      <Icon name="externalLink" size="sm" />官方说明
                    </a>
                  </div>
                </div>
              </article>
            </section>
          </div>
        </details>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { useClipboard } from '@/composables/useClipboard'
import {
  resourcesAPI,
  type ClientSetupTarget,
  type DownloadAsset,
  type DownloadManifest,
  type DownloadToolID
} from '@/api/resources'
import { useAppStore } from '@/stores/app'
import { buildClientAutoConfigCommand, getClientAutoConfigName } from '@/utils/clientAutoConfig'
import { buildCcsDiagnosticCommand } from '@/utils/ccSwitchDiagnostics'
import {
  buildClaudeDesktopWindowsInstallCommand,
  buildImmutableResourceDownloadPath,
  buildWindowsDesktopInstallCommand
} from '@/utils/resourceInstallCommands'

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

type QuickSetupID = ClientSetupTarget | 'cc-switch'
type DetectedOS = 'windows' | 'macos' | 'unsupported'

interface QuickSetupState {
  loading?: boolean
  command?: string
  keyName?: string
  groupName?: string
}

interface AdvancedInstallState {
  loading?: boolean
  command?: string
}

const detectedOS = computed<DetectedOS>(() => {
  const agent = navigator.userAgent.toLowerCase()
  if (agent.includes('windows')) return 'windows'
  if (agent.includes('macintosh') || agent.includes('mac os')) return 'macos'
  return 'unsupported'
})
const detectedOSLabel = computed(() => {
  if (detectedOS.value === 'windows') return 'Windows'
  if (detectedOS.value === 'macos') return 'macOS'
  return '暂不支持的系统'
})
const setupState = ref<Partial<Record<QuickSetupID, QuickSetupState>>>({})
const advancedInstallState = ref<Partial<Record<DownloadToolID, AdvancedInstallState>>>({})
const selectedSetup = ref<QuickSetupID>('codex')
const quickSetups: Array<{
  id: QuickSetupID
  title: string
  description: string
  icon: IconName
  platform?: 'openai' | 'anthropic' | 'grok'
}> = [
  {
    id: 'codex',
    title: '在 Codex 中使用',
    description: '图形界面与 CLI，一次配置完成',
    icon: 'cpu',
    platform: 'openai'
  },
  {
    id: 'claude',
    title: '在 Claude Code 中使用',
    description: '在 CLI 中使用；Desktop 需单独配置',
    icon: 'terminal',
    platform: 'anthropic'
  },
  {
    id: 'grok',
    title: '在 Grok Build 中使用',
    description: '原生 Grok 协议，一次安装并配置',
    icon: 'terminal',
    platform: 'grok'
  },
  {
    id: 'cc-switch',
    title: '安装 CC Switch',
    description: '使用 CC Switch 管理和切换配置',
    icon: 'swap'
  }
]

const quickSteps = computed(() => {
  if (detectedOS.value === 'windows') {
    return [
      { title: '生成并复制命令', description: '点击上方按钮', keys: [] as string[] },
      { title: '打开 PowerShell', description: '输入“PowerShell”并回车', keys: ['Win', 'S'] },
      { title: '粘贴并运行', description: '然后按 Enter', keys: ['Ctrl', 'V'] }
    ]
  }

  if (detectedOS.value === 'macos') {
    return [
      { title: '生成并复制命令', description: '点击上方按钮', keys: [] as string[] },
      { title: '打开终端', description: '输入“终端”并回车', keys: ['⌘', 'Space'] },
      { title: '粘贴并运行', description: '然后按 Enter', keys: ['⌘', 'V'] }
    ]
  }

  return [
    { title: '生成并复制命令', description: '点击上方按钮', keys: [] as string[] },
    { title: '打开终端', description: '打开系统自带终端', keys: [] as string[] },
    { title: '粘贴并运行', description: '粘贴命令后按 Enter', keys: [] as string[] }
  ]
})

const selectedState = computed(() => setupState.value[selectedSetup.value])
const selectedCommand = computed(() => {
  const command = selectedState.value?.command
  if (command) return command
  if (selectedSetup.value === 'cc-switch') return buildQuickSetupCommand('cc-switch')
  return '点击右侧按钮生成一键安装配置命令'
})
const selectedActionLabel = computed(() => {
  if (selectedState.value?.loading) return '正在生成'
  if (selectedSetup.value === 'cc-switch') return '复制一键安装命令'
  if (selectedState.value?.command) return '复制一键安装配置命令'
  return '生成一键安装配置命令'
})

const resources: DownloadResource[] = [
  {
    name: 'Claude Code',
    badge: 'Anthropic 官方编码 CLI',
    description: '适合在终端里直接让 Claude 阅读、修改和运行项目代码。Windows 一键命令使用国内 npm 镜像，并从本站缓存准备 Git Bash。',
    icon: 'terminal',
    commands: [
      {
        label: 'macOS / Linux / WSL',
        command: 'curl -fsSL https://claude.ai/install.sh | bash'
      },
      {
        label: 'Windows PowerShell',
        command: 'irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex'
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
    note: '页面顶部的一键配置会自动生成一次性凭证并安装客户端；这里的交互命令只用于手动排障。'
  },
  {
    name: 'Codex',
    badge: 'OpenAI 官方编码工具',
    description: '适合在本地终端中运行 Codex，也可以使用 Codex App 体验。下载区每 30 分钟检查新版本：官方仓库已有的包直接取官方源，Windows 桌面版取发布镜像。',
    icon: 'cpu',
    commands: [
      {
        label: 'Windows Codex App 首次安装（自动更新）',
        command: 'Start-Process "ms-appinstaller:?source=https://laoshirenai.com/api/v1/public-downloads/codex/windows-x64/latest.appinstaller"',
        note: '首次安装请使用这一行。确认安装后，Windows 会登记本站更新地址；以后后台检查新版本，并在 Codex 未运行时安全完成更新。'
      },
      {
        label: 'Windows Codex App 强制更新',
        command: '$f="$env:TEMP\\Codex-latest.msix"; curl.exe -L "https://laoshirenai.com/api/v1/public-downloads/codex/windows-x64/latest.msix" -o $f; if ($LASTEXITCODE -ne 0) { throw "下载失败" }; Add-AppxPackage -Path $f -ForceApplicationShutdown; Remove-Item $f -Force -ErrorAction SilentlyContinue; echo "OK-DONE"',
        note: '危险：执行前请先保存 Codex 中的工作并主动退出 Codex。该命令会强制关闭仍在运行的 Codex，然后立即更新。'
      }
    ],
    downloadToolId: 'codex',
    downloadTitle: '自动更新安装包',
    downloadHint: 'Windows 64 位提供 Codex App 的 MSIX 安装包；macOS 提供 OpenAI 官方版本化 Codex 包。本站每 30 分钟检查并缓存最新版。',
    verifyCommand: 'codex\ncodex app',
    primaryLink: 'https://github.com/openai/codex/releases/latest',
    docsLink: 'https://developers.openai.com/codex/cli',
    primaryAction: '查看官方 Release',
    note: '官方仓库有对应安装包时优先缓存官方版本；Windows Codex App 使用 Wangnov/codex-app-mirror 的最新 Release。'
  },
  {
    name: 'Codex++',
    badge: 'Codex App 增强启动器',
    description: '为 Codex App 增加插件与自定义接口等增强功能。',
    icon: 'sparkles',
    commands: [],
    downloadToolId: 'codex-plus-plus',
    downloadTitle: '常用安装包',
    downloadHint: '选择与你的系统和芯片匹配的安装包。',
    verifyText: '安装后先打开 Codex++ 管理工具检查状态，再从 Codex++ 入口启动 Codex App。顶部出现 Codex++ 菜单即表示增强注入成功。',
    primaryLink: 'https://github.com/BigPizzaV3/CodexPlusPlus/releases/latest',
    docsLink: 'https://github.com/BigPizzaV3/CodexPlusPlus',
    primaryAction: '查看 Codex++ Release',
    note: 'Codex++ 是第三方外部增强工具，不是 OpenAI 官方产品。它不修改 Codex App 原始安装文件；使用自定义接口注入前，建议确认 Codex App 已有官方登录态并保留 ~/.codex 配置备份。'
  },
  {
    name: 'Claude Desktop',
    badge: 'Claude 官方桌面 App',
    description: '中国大陆无魔法、无代理环境下，一键安装会同时准备最新版 Claude Desktop 和匹配的 Code 本地组件；随后通过 CC Switch 配置即可使用普通聊天和 Code Local。',
    icon: 'cube',
    commands: [],
    downloadToolId: 'claude-desktop',
    downloadTitle: '官方安装包',
    downloadHint: 'Windows 一键命令只从本站内容寻址缓存下载并校验 SHA256，不再让中国大陆用户回退到容易超时的 Anthropic 官方 CDN。',
    verifyText: '安装后到 CC Switch 的 Claude Desktop 页面导入并启用 Provider，保持 CC Switch 运行，再打开 Claude Desktop 测试普通聊天和 Code → Local。',
    primaryLink: 'https://claude.com/download',
    docsLink: 'https://support.claude.com/en/articles/10065433-install-claude-desktop',
    primaryAction: '打开官方下载页',
    note: '中国大陆无魔法、无代理环境下不能使用 Cowork。Cowork 依赖 Anthropic 官方云端工作区，CC Switch 只能配置模型中转，不能替代 Cowork 的官方网络连接。'
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
    note: 'CC Switch 安装包由本站每 30 分钟检查并按版本缓存，用户下载时不需要访问 GitHub。'
  }
]

const advancedResources = computed(() => {
  return resources.filter((resource) => resource.name === 'Codex++' || resource.name === 'Claude Desktop')
})

const downloadableTools = computed(() => {
  return advancedResources.value.map((resource) => resource.downloadToolId).filter(Boolean) as DownloadToolID[]
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
  if (asset.role === 'claude-desktop-code') return null
  if (asset.platform === 'windows') {
    if (asset.arch === 'arm64') return null
    if (tool === 'cc-switch' && !name.endsWith('.msi')) return null
    if (tool === 'codex-plus-plus' && !name.endsWith('.exe')) return null
    if (tool === 'claude-desktop' && (asset.role !== 'installer' || !name.endsWith('.msix'))) return null
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
      if (!name.endsWith('.dmg') && !name.endsWith('apple-darwin.tar.gz')) return null
      const scoreOffset = name.endsWith('.dmg') ? 0 : 10
      if (asset.arch === 'arm64') return { key: 'macos-arm64', score: 20 + scoreOffset }
      if (asset.arch === 'x64') return { key: 'macos-x64', score: 30 + scoreOffset }
      return { key: 'macos-universal', score: 20 + scoreOffset }
    }
    if (asset.arch === 'arm64') return { key: 'macos-arm64', score: 20 }
    if (asset.arch === 'x64') return { key: 'macos-x64', score: 30 }
    return { key: 'macos-universal', score: 20 }
  }

  return null
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, "'\"'\"'")}'`
}

function buildQuickSetupCommand(id: QuickSetupID, ticket?: string): string {
  if (detectedOS.value === 'unsupported') {
    return '当前系统暂不支持一键安装，请使用 Windows 或 macOS。'
  }
  if (id === 'cc-switch') {
    if (detectedOS.value === 'windows') {
      return buildCcsDiagnosticCommand('windows', window.location.origin)
    }
    return buildCcsDiagnosticCommand('macos', window.location.origin)
  }
  return buildClientAutoConfigCommand({
    target: id,
    ticket: ticket || '<点击生成一次性安装凭证>',
    isWindows: detectedOS.value === 'windows',
    installCodexApp: id === 'codex'
  })
}

async function prepareAndCopySetup(id: QuickSetupID) {
  if (detectedOS.value === 'unsupported') {
    appStore.showError('目前一键安装只支持 Windows 和 macOS。')
    return
  }
  if (id === 'cc-switch') {
    await copyCommand(buildQuickSetupCommand(id))
    return
  }

  setupState.value = {
    ...setupState.value,
    [id]: { ...setupState.value[id], loading: true }
  }
  try {
    const result = await resourcesAPI.createClientSetupTicket(id)
    const command = buildQuickSetupCommand(id, result.ticket)
    setupState.value = {
      ...setupState.value,
      [id]: {
        loading: false,
        command,
        keyName: result.key_name,
        groupName: result.group_name
      }
    }
    await copyToClipboard(command, `${getClientAutoConfigName(id)} 一键命令已复制`)
  } catch (error: any) {
    setupState.value = {
      ...setupState.value,
      [id]: { ...setupState.value[id], loading: false }
    }
    appStore.showError(error?.message || '生成一键安装命令失败，请稍后重试。')
  }
}

async function copyCommand(command: string) {
  await copyToClipboard(command, '安装命令已复制')
}

function advancedInstallCommand(tool: DownloadToolID): string {
  return advancedInstallState.value[tool]?.command || '点击右侧按钮生成一键下载安装命令'
}

function buildMacDesktopInstallCommand(urlByArch: { universal?: string; arm64?: string; x64?: string }): string {
  const urlSelection = urlByArch.universal
    ? `URL=${shellQuote(urlByArch.universal)}`
    : `if [ "$(uname -m)" = 'arm64' ]; then URL=${shellQuote(urlByArch.arm64 || '')}; else URL=${shellQuote(urlByArch.x64 || '')}; fi`
  return `TMP="$(mktemp -d)"; MOUNT="$TMP/mount"; mkdir -p "$MOUNT"; ${urlSelection}; curl -fL "$URL" -o "$TMP/app.dmg" && hdiutil attach "$TMP/app.dmg" -nobrowse -readonly -mountpoint "$MOUNT" >/dev/null && APP="$(find "$MOUNT" -maxdepth 1 -name '*.app' -print -quit)" && test -n "$APP" && codesign --verify --deep --strict "$APP" && mkdir -p "$HOME/Applications" && DEST="$HOME/Applications/$(basename "$APP")" && rm -rf "$DEST" && ditto "$APP" "$DEST"; hdiutil detach "$MOUNT" >/dev/null 2>&1 || true; rm -rf "$TMP"; test -n "$DEST" && open "$DEST"`
}

function immutableResourceDownloadURL(tool: DownloadToolID, asset: DownloadAsset): string {
  const manifest = manifests.value[tool]
  if (!manifest) throw new Error('安装包清单尚未加载')
  return new URL(
    buildImmutableResourceDownloadPath(tool, manifest.version, asset),
    window.location.origin
  ).toString()
}

async function prepareAdvancedInstallCommand(tool: DownloadToolID) {
  if (detectedOS.value === 'unsupported') {
    appStore.showError('目前一键安装只支持 Windows 和 macOS。')
    return
  }

  advancedInstallState.value = {
    ...advancedInstallState.value,
    [tool]: { ...advancedInstallState.value[tool], loading: true }
  }

  try {
    const assets = preferredAssets(tool).filter((asset) => asset.platform === detectedOS.value)
    if (assets.length === 0) throw new Error('暂无适合当前系统的安装包')

    let command = ''
    if (detectedOS.value === 'windows') {
      const asset = assets.find((item) => item.arch === 'x64') || assets[0]
      if (tool === 'claude-desktop') {
        const component = manifests.value[tool]?.assets.find((item) =>
          item.platform === 'windows' && item.arch === asset.arch &&
          item.role === 'claude-desktop-code' && !!item.component_version && !!item.upstream_sha256
        )
        if (!component) throw new Error('最新版 Claude Desktop 尚未准备好匹配的 Code 本地组件')
        command = buildClaudeDesktopWindowsInstallCommand(window.location.origin)
      } else {
        command = buildWindowsDesktopInstallCommand({
          tool,
          sources: [{ url: immutableResourceDownloadURL(tool, asset), sha256: asset.sha256 }]
        })
      }
    } else {
      const universal = assets.find((item) => item.arch === 'universal')
      if (universal) {
        command = buildMacDesktopInstallCommand({ universal: immutableResourceDownloadURL(tool, universal) })
      } else {
        const arm64 = assets.find((item) => item.arch === 'arm64')
        const x64 = assets.find((item) => item.arch === 'x64')
        if (!arm64 || !x64) throw new Error('安装包缺少对应的 Mac 芯片版本')
        command = buildMacDesktopInstallCommand({
          arm64: immutableResourceDownloadURL(tool, arm64),
          x64: immutableResourceDownloadURL(tool, x64)
        })
      }
    }

    advancedInstallState.value = {
      ...advancedInstallState.value,
      [tool]: { loading: false, command }
    }
    await copyToClipboard(command, '一键下载安装命令已复制')
  } catch (error: any) {
    advancedInstallState.value = {
      ...advancedInstallState.value,
      [tool]: { ...advancedInstallState.value[tool], loading: false }
    }
    appStore.showError(error?.message || '生成安装命令失败，请稍后重试。')
  }
}

async function loadDownloads(tool: DownloadToolID) {
  loading.value = { ...loading.value, [tool]: true }
  errors.value = { ...errors.value, [tool]: '' }
  try {
    const manifest = await resourcesAPI.getDownloads(tool)
    manifests.value = { ...manifests.value, [tool]: manifest }
  } catch {
    errors.value = {
      ...errors.value,
      [tool]: '安装包暂未加载，请稍后重试。'
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
    window.location.assign(immutableResourceDownloadURL(tool, asset))
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

function formatAssetLabel(tool: DownloadToolID, asset: DownloadAsset): string {
  if (tool === 'codex' && asset.name.toLowerCase().endsWith('.msix')) return 'Windows 64 位 Codex App'
  if (tool === 'codex' && asset.name.toLowerCase().endsWith('.dmg')) {
    return asset.arch === 'arm64' ? 'macOS Apple 芯片 Codex App' : 'macOS Intel Codex App'
  }
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
