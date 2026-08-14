export const BALANCE_TOPUP_PRESETS = [20, 50, 100] as const

export type PromotionalBalanceTopup = {
  paidAmountCny: number
  bonusAmountCny: number
  creditedAmountCny: number
  bonusPercent: number
  newcomerOnly?: boolean
}

export const PROMOTIONAL_BALANCE_TOPUPS: readonly PromotionalBalanceTopup[] = [
  { paidAmountCny: 5, bonusAmountCny: 5, creditedAmountCny: 10, bonusPercent: 100, newcomerOnly: true },
  { paidAmountCny: 500, bonusAmountCny: 50, creditedAmountCny: 550, bonusPercent: 10 },
  { paidAmountCny: 1000, bonusAmountCny: 100, creditedAmountCny: 1100, bonusPercent: 10 }
] as const

const supportedBalanceTopupAmounts = new Set<number>([
  ...BALANCE_TOPUP_PRESETS,
  ...PROMOTIONAL_BALANCE_TOPUPS.map((product) => product.paidAmountCny)
])

export function isSupportedBalanceTopupAmount(amount: number): boolean {
  return supportedBalanceTopupAmounts.has(amount)
}

export function isNewcomerBalanceTopup(amount: number): boolean {
  return getPromotionalBalanceTopup(amount)?.newcomerOnly === true
}

export function getPromotionalBalanceTopup(amount: number): PromotionalBalanceTopup | undefined {
  return PROMOTIONAL_BALANCE_TOPUPS.find((product) => product.paidAmountCny === amount)
}

export function getCreditedBalanceTopupAmount(amount: number): number {
  return getPromotionalBalanceTopup(amount)?.creditedAmountCny ?? amount
}
