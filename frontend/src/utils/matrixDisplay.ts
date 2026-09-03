/**
 * 能力矩阵页面的共享展示规则：协议名、推理档位短标签和上下文窗口格式。
 * 两张矩阵表（模型/客户端）必须使用同一份标签，避免展示口径漂移。
 */

export const MATRIX_PROTOCOL_LABEL: Record<string, string> = {
  responses: 'Responses',
  chat_completions: 'Chat Completions',
  messages: 'Messages',
  generate_content: 'GenerateContent',
}

const EFFORT_SHORT_LABEL: Record<string, string> = {
  none: '关闭',
  minimal: 'Min',
  low: 'Low',
  medium: 'Med',
  high: 'High',
  xhigh: 'XHigh',
  max: 'Max',
  always_on: '常开',
  disabled: '禁用',
  adaptive: '自适应',
  off: '关',
  ultra: 'Ultra',
}

export function formatMatrixLevels(levels: readonly string[]): string {
  if (!levels.length) return '模型默认'
  return levels.map(level => EFFORT_SHORT_LABEL[level] ?? level).join(' / ')
}

export function formatMatrixContext(tokens: number | null | undefined): string {
  if (!tokens || tokens <= 0) return '—'
  if (tokens >= 1_000_000) {
    const value = tokens / 1_000_000
    return `${Number.isInteger(value) ? value : value.toFixed(2)}M`
  }
  return `${Math.round(tokens / 1000)}K`
}
