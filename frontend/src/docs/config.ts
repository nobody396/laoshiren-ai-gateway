/**
 * Documentation sidebar configuration and markdown loader.
 *
 * To add a new doc page:
 *   1. Create a `.md` file in `src/docs/content/`
 *   2. Add an entry to `docsConfig` below
 */

import { clientMatrix, type ClientMatrixEntry, type ClientMatrixProtocol } from '@/generated/clientMatrix'
import { simpleClientGuideById } from '@/docs/guides/simpleClientGuides'

export interface DocItem {
  title: string
  slug: string
  description: string
  publicPath?: string
  lastModified?: string
}

export interface DocCategory {
  key: string
  title: string
  collapsed?: boolean
  items: DocItem[]
}

export type DocsConfig = DocCategory[]

// Keep the public SEO timestamp unchanged while Docs remain hidden. Update this
// in the separate publication change that flips the public Docs flag.
export const docsLastModified = '2026-09-09'

function protocolLabel(protocol: ClientMatrixProtocol): string {
  if (protocol === 'responses') return 'Responses'
  if (protocol === 'chat_completions') return 'Chat Completions'
  if (protocol === 'messages') return 'Messages'
  return 'GenerateContent'
}

function integrationDescription(client: ClientMatrixEntry): string {
  const protocols = client.protocols.length
    ? client.protocols.map(protocolLabel).join('、')
    : '尚无可公开协议'
  const state = client.one_click_status === 'ready'
    ? '一键导入可用'
    : client.one_click_status === 'prototype'
      ? '配置合同已定义，自动导入准备中'
      : '集成未开放'
  return `${protocols}；${state}。`
}

const integrationItems: DocItem[] = clientMatrix
  .filter(client => simpleClientGuideById[client.id])
  .map(client => ({
    title: client.name,
    slug: client.slug,
    description: integrationDescription(client),
  }))

export const docsConfig: DocsConfig = [
  {
    key: 'start',
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
    key: 'api',
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
    key: 'integrations',
    title: '工具集成',
    items: integrationItems,
  },
  {
    key: 'models',
    title: '模型目录',
    items: [
      {
        title: '模型目录',
        slug: 'models',
        description: '按逻辑模型查看协议、可用方案、倍率和推荐工具。'
      },
      {
        title: '模型能力矩阵',
        slug: 'model-matrix',
        description: '每个模型已验证的协议与推理强度总表。'
      },
      {
        title: '客户端能力矩阵',
        slug: 'client-matrix',
        description: '每个客户端已验证的协议、推理强度控制与一键配置状态。'
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
