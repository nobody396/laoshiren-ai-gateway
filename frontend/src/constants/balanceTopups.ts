export const BALANCE_TOPUP_PRESETS = [20, 50, 100] as const

const supportedBalanceTopupAmounts = new Set<number>(BALANCE_TOPUP_PRESETS)

export function isSupportedBalanceTopupAmount(amount: number): boolean {
  return supportedBalanceTopupAmounts.has(amount)
}
