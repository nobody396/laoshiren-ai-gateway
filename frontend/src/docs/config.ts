/**
 * Documentation sidebar configuration and markdown loader.
 *
 * To add a new doc page:
 *   1. Create a `.md` file in `src/docs/content/`
 *   2. Add an entry to `docsConfig` below
 */

export interface DocItem {
  title: string
  slug: string
  description: string
  publicPath?: string
  lastModified?: string
}

export interface DocCategory {
  title: string
  collapsed?: boolean
  items: DocItem[]
}

export type DocsConfig = DocCategory[]

export const docsLastModified = '2026-06-30'

export const docsConfig: DocsConfig = [
  {
    title: '站点介绍',
    items: [
      {
        title: '简介',
        slug: 'introduction',
        description: '了解老实人AI 的 AI 编码接口与网关服务、适用场景、模型支持和基础接入方式。'
      },
      {
        title: '常见问题',
        slug: 'faq',
        description: '汇总老实人AI 账号、充值、模型额度、API Key、Claude Code 和 Codex 使用中的常见问题。'
      },
      {
        title: '新手总览',
        slug: 'newcomer-overview',
        description: '从注册、按量与月卡选择、充值兑换、创建API Key、选择分组到第一次成功调用的老实人AI新手总览。',
        lastModified: '2026-08-29'
      },
      {
        title: '老实人AI 使用指南',
        slug: 'laoshirenai-guide',
        description: '从注册、充值、创建 API Key 到配置 Claude Code 和 Codex 的老实人AI 完整使用指南。'
      },
    ],
  },
  {
    title: '环境准备',
    items: [
      {
        title: 'Node.js 环境安装指南',
        slug: 'nodejs-setup',
        description: '面向 Claude Code、Codex 和相关开发工具的 Node.js 环境安装与验证教程。'
      },
      {
        title: '自动配置工具',
        slug: 'auto-config-tool',
        description: '使用老实人AI 自动配置脚本快速写入 Claude Code、Codex 和本地开发环境所需配置。',
        lastModified: '2026-07-25'
      },
      {
        title: 'Codex App Windows 下载',
        slug: 'codex-app-windows-download',
        description: '下载并安装 Codex App for Windows 的 MSIX 缓存镜像，明确区分 Codex App 和 Codex CLI。'
      },
    ],
  },
  {
    title: '快速接入',
    items: [
      {
        title: 'Base URL 填写总指南',
        slug: 'base-url-guide',
        description: '区分 Claude Code、Codex、OpenAI SDK、Anthropic SDK、Antigravity 和 GPT-Image 的老实人AI Base URL 填写方式。'
      },
      {
        title: 'Claude Code 快速开始指南',
        slug: 'claude-code-quickstart',
        description: '配置老实人AI 的 ANTHROPIC_BASE_URL 和 API Key，在 Claude Code 中开始使用 Claude 编码模型。'
      },
      {
        title: 'Codex 快速开始指南',
        slug: 'codex-quickstart',
        description: '在 Codex CLI 中配置老实人AI Provider、Base URL 和 API Key，快速接入 AI 编码模型。'
      },
      {
        title: 'OpenClaw 快速开始指南',
        slug: 'openclaw-quickstart',
        description: '在 OpenClaw 中配置老实人AI 的 Anthropic-compatible 或 OpenAI-compatible Provider。'
      },
      {
        title: 'Hermes 快速开始指南',
        slug: 'hermes-quickstart',
        description: '通过 Hermes 配置老实人AI Base URL、Provider 和模型，完成 Claude 兼容接口接入。'
      },
      {
        title: 'Cherry Studio 快速开始指南',
        slug: 'cherry-studio-quickstart',
        description: '在 Cherry Studio 中添加老实人AI 提供商，配置 API 地址、密钥和可用模型。'
      },
      {
        title: 'Claude Desktop 第三方 Provider 配置指南',
        slug: 'claude-desktop-configuration',
        description: '在 Claude Desktop 中配置第三方 Provider，使用老实人AI Gateway base URL 和 API Key。'
      },
      {
        title: 'GPT-Image-2 使用指南',
        slug: 'gpt-image-quickstart',
        description: '使用老实人AI 的 GPT-Image-2 接口完成图像生成请求、任务查询和结果获取。'
      },
    ],
  },
  {
    title: 'API 参考',
    items: [
      {
        title: 'API 参考总览',
        slug: 'api-reference',
        description: '老实人AI API 的鉴权、Base URL、Models、Responses、Chat Completions、Anthropic Messages、Gemini、流式、图片、参数和错误处理参考。',
        lastModified: '2026-08-29'
      },
    ],
  },
  {
    title: '排错与运维',
    items: [
      {
        title: 'API Key 与分组选择指南',
        slug: 'api-key-group-guide',
        description: '解释老实人AI API Key、订单号、兑换码、分组、模型支持和倍率之间的区别。'
      },
      {
        title: 'Claude Code 配置排错指南',
        slug: 'claude-code-troubleshooting',
        description: '排查 Claude Code 接入老实人AI 时的 Base URL、API Key、分组、旧环境变量和旧窗口缓存问题。'
      },
      {
        title: 'Codex 配置排错指南',
        slug: 'codex-troubleshooting',
        description: '排查 Codex CLI/App 接入老实人AI 时的 Provider、Base URL、Responses 模式、WSL 和登录问题。'
      },
      {
        title: '常见 API 报错排查',
        slug: 'common-api-errors',
        description: '解释老实人AI 常见 401、403、429、502、503、连接超时和模型不可用问题的排查路径。'
      },
      {
        title: 'Windows 配置指南',
        slug: 'windows-setup',
        description: '在 Windows、PowerShell 和 WSL 环境下配置 Claude Code、Codex 与老实人AI API Key。'
      },
      {
        title: 'macOS 配置指南',
        slug: 'macos-setup',
        description: '在 macOS 终端环境下配置 Claude Code、Codex、环境变量和老实人AI API Key。'
      },
    ],
  },
  {
    title: '国内使用与选型',
    items: [
      {
        title: '豆包场景下的 AI 编码工具接入指南',
        slug: 'doubao-geo-ai-coding',
        description: '面向豆包和国内 AI 搜索场景，说明老实人AI 如何帮助用户接入 Claude Code、Codex 和 AI 编码工具。'
      },
      {
        title: 'Claude Code 国内使用指南',
        slug: 'claude-code-china-guide',
        description: '说明国内开发者如何通过老实人AI 配置 Claude Code 的 Base URL、API Key、环境变量和排错路径。'
      },
      {
        title: 'Codex 国内使用指南',
        slug: 'codex-china-guide',
        description: '说明国内开发者如何通过老实人AI 配置 Codex、OpenAI 兼容 Provider、Base URL、WSL 和 API Key。'
      },
      {
        title: 'Codex 免 API Key 使用指南',
        slug: 'codex-no-api-key-guide',
        description: '解释 Codex ChatGPT 登录、API Key 登录、访问令牌和第三方接口之间的区别，避免把“免 API Key”误解成免费或无限制。'
      },
      {
        title: 'Codex 自定义 API 配置：Base URL 与 config.toml',
        slug: 'codex-custom-api-guide',
        description: '面向 Codex CLI/App 的自定义 API、Base URL、config.toml、auth.json、Responses 模式和第三方兼容接口配置指南。',
        lastModified: '2026-08-10'
      },
      {
        title: 'Claude Code 和 Codex 怎么选',
        slug: 'claude-code-vs-codex',
        description: '对比 Claude Code 和 Codex 的协议、Base URL、常见问题和团队使用场景。'
      },
      {
        title: 'AI API 网关和基础 API 接入有什么区别',
        slug: 'ai-api-gateway-service',
        description: '解释个人开发者和企业团队如何区分基础 API 接入、AI API 网关和多模型统一接入。'
      },
    ],
  },
  {
    title: '企业方案',
    items: [
      {
        title: '企业 AI API 网关方案',
        slug: 'enterprise-ai-api-gateway',
        publicPath: '/enterprise',
        description: '面向企业团队介绍老实人AI 的多模型统一接入、团队 API Key 管理、成本控制和调用审计方案。'
      },
      {
        title: '团队 API Key 管理',
        slug: 'team-api-key-management',
        description: '介绍企业团队如何用老实人AI 管理项目级 API Key、分组、权限、用量和成本归因。'
      },
      {
        title: '企业发票与合同说明',
        slug: 'invoice-contract-enterprise',
        description: '说明老实人AI 企业客户在发票、合同、采购沟通和售后支持中的常见流程。'
      },
      {
        title: '服务状态与支持说明',
        slug: 'sla-support',
        publicPath: '/status',
        description: '说明老实人AI 的服务状态入口、健康检查、模型检测报告、故障反馈和支持边界。'
      },
      {
        title: '安全与隐私说明',
        slug: 'security',
        publicPath: '/security',
        description: '说明老实人AI 在 API Key、调用日志、客服排查、敏感信息和企业接入中的安全与隐私边界。'
      },
      {
        title: '团队为什么需要 AI API 网关',
        slug: 'ai-api-gateway-for-teams',
        description: '说明企业团队为什么需要统一管理 AI API 入口、API Key、用量、成本、权限和排错。'
      },
      {
        title: '多模型统一接入管理指南',
        slug: 'multi-model-api-management',
        description: '介绍企业如何统一管理 Claude Code、Codex、ChatGPT、Gemini 和 OpenAI 兼容工具的 API 接入。'
      },
      {
        title: 'AI API 成本控制指南',
        slug: 'ai-api-cost-control',
        description: '说明企业如何通过 API Key 拆分、分组、额度、限速和调用记录控制 AI API 成本。'
      },
      {
        title: '团队 AI 编码工具接入方案',
        slug: 'team-ai-coding-solution',
        description: '介绍企业团队如何统一接入 Claude Code、Codex 和其他 AI 编码工具，并建立排错和成本管理流程。'
      },
      {
        title: '企业客户常见问题',
        slug: 'enterprise-ai-faq',
        description: '汇总企业客户在团队接入、多模型管理、成本控制、发票合同、安全隐私和故障支持中的常见问题。'
      },
    ],
  },
  {
    title: '进阶指南',
    items: [
      {
        title: 'VS Code 图形化操作教程',
        slug: 'vscode-gui-guide',
        description: '通过 VS Code 图形界面配置 Claude Code、环境变量和老实人AI API Key。'
      },
      {
        title: 'Claude Code 与 Codex 协同开发',
        slug: 'claude-code-codex-collaboration-guide',
        description: '介绍 Claude Code 与 Codex 的协同开发流程、角色分工和老实人AI 配置方式。'
      },
      {
        title: '推荐全局规则',
        slug: 'recommended-global-rules',
        description: '适用于 AI 编码工作流的 AGENTS.md 全局规则示例，帮助规范 Claude Code 和 Codex 协作。'
      },
    ],
  },
]

export const defaultSlug = 'introduction'

/**
 * 历史文档链接兼容映射。
 * 旧版本直接使用中文文件名作为 slug，这里保留兼容，避免外部链接失效。
 */
const legacyDocSlugMap: Record<string, string> = {
  '老实人 AI × Claude Code VS Code 图形化操作教程': 'vscode-gui-guide',
  '老实人AI × Claude Code VS Code 图形化操作教程': 'vscode-gui-guide',
  'Claude Code与Codex协同开发指南': 'claude-code-codex-collaboration-guide',
  'Claude Code快速开始指南': 'claude-code-quickstart',
  'Claude%20Code快速开始指南': 'claude-code-quickstart',
  'Codex快速开始指南': 'codex-quickstart',
  'Codex App Windows下载': 'codex-app-windows-download',
  'Codex App Windows 下载': 'codex-app-windows-download',
  '老实人 AI × Hermes 快速开始指南': 'hermes-quickstart',
  '老实人AI × Hermes 快速开始指南': 'hermes-quickstart',
  'Hermes快速开始指南': 'hermes-quickstart',
  'Node.js环境安装指南': 'nodejs-setup',
  'Node.js环境安装指南.md': 'nodejs-setup',
  'OpenClaw快速开始指南': 'openclaw-quickstart',
  '常见问题': 'faq',
  '推荐全局规则': 'recommended-global-rules',
  '自动配置工具': 'auto-config-tool',
}

// Lazy-load all markdown files via Vite glob
const modules = import.meta.glob('./content/*.md', {
  query: '?raw',
  import: 'default',
})

/**
 * 将外部传入的文档 slug 归一化为实际文件名。
 */
export function resolveDocSlug(slug: string): string {
  return legacyDocSlugMap[slug] ?? slug
}

export function getAllDocItems(): DocItem[] {
  return docsConfig.flatMap((category) =>
    category.items.map((item) => ({
      lastModified: docsLastModified,
      ...item,
    }))
  )
}

export function findDocItemBySlug(slug: string): DocItem | undefined {
  const resolvedSlug = resolveDocSlug(slug)
  return getAllDocItems().find((item) => item.slug === resolvedSlug)
}

export async function loadMarkdown(slug: string): Promise<string | null> {
  const resolvedSlug = resolveDocSlug(slug)
  const loader = modules[`./content/${resolvedSlug}.md`]
  if (!loader) return null
  return (await loader()) as string
}
