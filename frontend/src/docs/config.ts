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

// Keep the public SEO timestamp unchanged while Docs remain hidden. Update this
// in the separate publication change that flips the public Docs flag.
export const docsLastModified = '2026-06-30'

export const docsConfig: DocsConfig = [
  {
    title: '快速开始',
    items: [
      {
        title: '快速开始',
        slug: 'quickstart',
        description: '创建 API Key，并通过一个最小请求验证接入。'
      }
    ],
  },
  {
    title: 'API 参考',
    items: [
      {
        title: 'API 概览',
        slug: 'api-overview',
        description: 'Base URL、鉴权、协议和错误格式。'
      },
      {
        title: 'OpenAI Responses',
        slug: 'api-responses',
        description: '调用 POST /v1/responses。'
      },
      {
        title: 'OpenAI Chat Completions',
        slug: 'api-chat-completions',
        description: '调用 POST /v1/chat/completions。'
      },
      {
        title: 'Anthropic Messages',
        slug: 'api-anthropic-messages',
        description: '调用 POST /v1/messages。'
      },
      {
        title: 'Gemini',
        slug: 'api-gemini',
        description: '调用 Gemini GenerateContent 接口。'
      },
      {
        title: 'Models',
        slug: 'api-models',
        description: '查询当前 API Key 可用的模型。'
      },
      {
        title: 'Images',
        slug: 'api-images',
        description: '调用图片生成和图片编辑接口。'
      }
    ],
  },
  {
    title: '工具集成',
    items: [
      {
        title: 'Claude Code',
        slug: 'integration-claude-code',
        description: '通过 Anthropic Messages 接入 Claude Code。'
      },
      {
        title: 'Codex',
        slug: 'integration-codex',
        description: '通过 OpenAI Responses 接入 Codex。'
      },
      {
        title: 'Grok Build',
        slug: 'integration-grok-build',
        description: '配置 Grok Build。'
      },
      {
        title: 'Gemini CLI',
        slug: 'integration-gemini-cli',
        description: '通过 Gemini 原生协议接入 Gemini CLI。'
      },
      {
        title: 'Antigravity',
        slug: 'integration-antigravity',
        description: '配置 Antigravity 的 Claude 和 Gemini 接口。'
      },
      {
        title: 'Kimi Code',
        slug: 'integration-kimi-code',
        description: '配置 Kimi Code。'
      },
      {
        title: 'ZCode',
        slug: 'integration-zcode',
        description: '配置 ZCode。'
      },
      {
        title: 'OpenCode',
        slug: 'integration-opencode',
        description: '配置 OpenCode。'
      }
    ],
  },
  {
    title: '模型目录',
    items: [
      {
        title: '模型目录',
        slug: 'models',
        description: '按逻辑模型查看协议、可用方案、倍率和推荐工具。'
      }
    ],
  },
]

export const defaultSlug = 'quickstart'

/**
 * 历史文档链接兼容映射。
 * 旧版本直接使用中文文件名作为 slug，这里保留兼容，避免外部链接失效。
 */
const legacyDocSlugMap: Record<string, string> = {
  'Claude Code快速开始指南': 'integration-claude-code',
  'Claude%20Code快速开始指南': 'integration-claude-code',
  'Codex快速开始指南': 'integration-codex',
  '常见问题': 'quickstart',
  '自动配置工具': 'quickstart',
  'claude-code-quickstart': 'integration-claude-code',
  'claude-code-troubleshooting': 'integration-claude-code',
  'claude-code-china-guide': 'integration-claude-code',
  'codex-quickstart': 'integration-codex',
  'codex-troubleshooting': 'integration-codex',
  'codex-china-guide': 'integration-codex',
  'codex-custom-api-guide': 'integration-codex',
  'base-url-guide': 'api-overview',
  'common-api-errors': 'api-overview',
  'api-key-group-guide': 'models',
  'gpt-image-quickstart': 'api-images',
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
