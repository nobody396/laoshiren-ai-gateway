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
  lastModified?: string
}

export interface DocCategory {
  title: string
  collapsed?: boolean
  items: DocItem[]
}

export type DocsConfig = DocCategory[]

export const docsLastModified = '2026-05-21'

export const docsConfig: DocsConfig = [
  {
    title: '站点介绍',
    items: [
      {
        title: '简介',
        slug: 'introduction',
        description: '了解老实人 AI 的 AI 编码中转服务、适用场景、模型支持和基础接入方式。'
      },
      {
        title: '常见问题',
        slug: 'faq',
        description: '汇总老实人 AI 账号、充值、模型额度、API Key、Claude Code 和 Codex 使用中的常见问题。'
      },
      {
        title: '老实人 AI 使用指南',
        slug: 'laoshirenai-guide',
        description: '从注册、充值、创建 API Key 到配置 Claude Code 和 Codex 的老实人 AI 完整使用指南。'
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
        description: '使用老实人 AI 自动配置脚本快速写入 Claude Code、Codex 和本地开发环境所需配置。'
      },
    ],
  },
  {
    title: '快速接入',
    items: [
      {
        title: 'Claude Code 快速开始指南',
        slug: 'claude-code-quickstart',
        description: '配置老实人 AI 的 ANTHROPIC_BASE_URL 和 API Key，在 Claude Code 中开始使用 Claude 编码模型。'
      },
      {
        title: 'Codex 快速开始指南',
        slug: 'codex-quickstart',
        description: '在 Codex CLI 中配置老实人 AI Provider、Base URL 和 API Key，快速接入 AI 编码模型。'
      },
      {
        title: 'OpenClaw 快速开始指南',
        slug: 'openclaw-quickstart',
        description: '在 OpenClaw 中配置老实人 AI 的 Anthropic-compatible 或 OpenAI-compatible Provider。'
      },
      {
        title: 'Hermes 快速开始指南',
        slug: 'hermes-quickstart',
        description: '通过 Hermes 配置老实人 AI Base URL、Provider 和模型，完成 Claude 兼容接口接入。'
      },
      {
        title: 'Cherry Studio 快速开始指南',
        slug: 'cherry-studio-quickstart',
        description: '在 Cherry Studio 中添加老实人 AI 提供商，配置 API 地址、密钥和可用模型。'
      },
      {
        title: 'Claude Desktop 第三方 Provider 配置指南',
        slug: 'claude-desktop-configuration',
        description: '在 Claude Desktop 中配置第三方 Provider，使用老实人 AI Gateway base URL 和 API Key。'
      },
      {
        title: 'GPT-Image-2 使用指南',
        slug: 'gpt-image-quickstart',
        description: '使用老实人 AI 的 GPT-Image-2 接口完成图像生成请求、任务查询和结果获取。'
      },
    ],
  },
  {
    title: '进阶指南',
    items: [
      {
        title: 'VS Code 图形化操作教程',
        slug: 'vscode-gui-guide',
        description: '通过 VS Code 图形界面配置 Claude Code、环境变量和老实人 AI API Key。'
      },
      {
        title: 'Claude Code 与 Codex 协同开发',
        slug: 'claude-code-codex-collaboration-guide',
        description: '介绍 Claude Code 与 Codex 的协同开发流程、角色分工和老实人 AI 配置方式。'
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
  'Claude Code与Codex协同开发指南': 'claude-code-codex-collaboration-guide',
  'Claude Code快速开始指南': 'claude-code-quickstart',
  'Claude%20Code快速开始指南': 'claude-code-quickstart',
  'Codex快速开始指南': 'codex-quickstart',
  '老实人 AI × Hermes 快速开始指南': 'hermes-quickstart',
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
