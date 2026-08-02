export const BALANCE_TOPUP_PRESETS = [20, 50, 100] as const

export type PromotionalBalanceTopup = {
  paidAmountCny: number
  bonusAmountCny: number
  creditedAmountCny: number
  bonusPercent: number
}

export const PROMOTIONAL_BALANCE_TOPUPS: readonly PromotionalBalanceTopup[] = [
  { paidAmountCny: 500, bonusAmountCny: 75, creditedAmountCny: 575, bonusPercent: 15 },
  { paidAmountCny: 1000, bonusAmountCny: 200, creditedAmountCny: 1200, bonusPercent: 20 }
] as const

const supportedBalanceTopupAmounts = new Set<number>(BALANCE_TOPUP_PRESETS)

export function isSupportedBalanceTopupAmount(amount: number): boolean {
  return supportedBalanceTopupAmounts.has(amount)
}

export function getPromotionalBalanceTopup(amount: number): PromotionalBalanceTopup | undefined {
  return PROMOTIONAL_BALANCE_TOPUPS.find((product) => product.paidAmountCny === amount)
}

export function getCreditedBalanceTopupAmount(amount: number): number {
  return getPromotionalBalanceTopup(amount)?.creditedAmountCny ?? amount
}
