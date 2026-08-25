/**
 * 落地页翻牌计数器(split-flap / odometer)的数字格式化工具。
 *
 * 与预览版保持同一约定:千分位分组关闭(逗号会让翻牌列错位),
 * 整数部分前导补零到 pad 位,小数固定 decimals 位。
 * 组件只负责把返回字符串逐字符渲染成滚轮列。
 */

export interface FlipNumberOptions {
  /** 整数部分最小位数(不足前导补零) */
  pad?: number
  /** 小数位数 */
  decimals?: number
}

/**
 * 把数值格式化成翻牌文本,例如 (19216, { pad: 4 }) -> "19216.00"。
 * 非有限数与负数一律归零 —— 计数器永远不显示负值或 NaN。
 */
export function formatFlipNumber(value: number, options: FlipNumberOptions = {}): string {
  const { pad = 5, decimals = 2 } = options
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  return safe.toLocaleString('en-US', {
    minimumIntegerDigits: pad,
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
    useGrouping: false,
  })
}

/** 判断翻牌文本中的字符是否为可滚动数字(否则是 "." 等分隔符)。 */
export function isFlipDigit(char: string): boolean {
  return /\d/.test(char)
}
