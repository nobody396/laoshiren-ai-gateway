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
}

export interface DocCategory {
  title: string
  collapsed?: boolean
  items: DocItem[]
}

export type DocsConfig = DocCategory[]

export const docsConfig: DocsConfig = [
  {
    title: '站点介绍',
    items: [
      { title: '简介', slug: 'introduction' },
      { title: '常见问题', slug: 'faq' },
      { title: 'Dragon Code 使用指南', slug: 'dragon-code-guide' },
    ],
  },
  {
    title: '环境准备',
    items: [
      { title: 'Node.js 环境安装指南', slug: 'nodejs-setup' },
      { title: '自动配置工具', slug: 'auto-config-tool' },
    ],
  },
  {
    title: '快速接入',
    items: [
      { title: 'Claude Code 快速开始指南', slug: 'claude-code-quickstart' },
      { title: 'Codex 快速开始指南', slug: 'codex-quickstart' },
      { title: 'OpenClaw 快速开始指南', slug: 'openclaw-quickstart' },
      { title: 'Hermes 快速开始指南', slug: 'hermes-quickstart' },
      { title: 'Cherry Studio 快速开始指南', slug: 'cherry-studio-quickstart' },
      { title: 'Claude Desktop 第三方 Provider 配置指南', slug: 'claude-desktop-configuration' },
      { title: 'GPT-Image-2 使用指南', slug: 'gpt-image-quickstart' },
    ],
  },
  {
    title: '进阶指南',
    items: [
      { title: 'VS Code 图形化操作教程', slug: 'vscode-gui-guide' },
      { title: 'Claude Code 与 Codex 协同开发', slug: 'claude-code-codex-collaboration-guide' },
      { title: '推荐全局规则', slug: 'recommended-global-rules' },
    ],
  },
]

export const defaultSlug = 'introduction'

/**
 * 历史文档链接兼容映射。
 * 旧版本直接使用中文文件名作为 slug，这里保留兼容，避免外部链接失效。
 */
const legacyDocSlugMap: Record<string, string> = {
  'Dragon Code × Claude Code VS Code 图形化操作教程': 'vscode-gui-guide',
  'Claude Code与Codex协同开发指南': 'claude-code-codex-collaboration-guide',
  'Claude Code快速开始指南': 'claude-code-quickstart',
  'Claude%20Code快速开始指南': 'claude-code-quickstart',
  'Codex快速开始指南': 'codex-quickstart',
  'Dragon Code × Hermes 快速开始指南': 'hermes-quickstart',
  'Hermes快速开始指南': 'hermes-quickstart',
  'Hermes-DragonCode(3)': 'hermes-quickstart',
  'Hermes-DragonCode(3).md': 'hermes-quickstart',
  'Hermes-DragonCode%283%29': 'hermes-quickstart',
  'Hermes-DragonCode%283%29.md': 'hermes-quickstart',
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

export async function loadMarkdown(slug: string): Promise<string | null> {
  const resolvedSlug = resolveDocSlug(slug)
  const loader = modules[`./content/${resolvedSlug}.md`]
  if (!loader) return null
  return (await loader()) as string
}
