import type { SubscriptionType } from '@/types'

export type GroupOptionSectionId = 'monthly' | 'payg'

export interface SectionableGroupOption {
  label: string
  rate: number
  userRate: number | null
  subscriptionType: SubscriptionType
}

export interface GroupOptionSection<T extends SectionableGroupOption> {
  id: GroupOptionSectionId
  options: T[]
}

export const isMonthlyGroupOption = (option: SectionableGroupOption): boolean => {
  return option.subscriptionType === 'subscription' || option.subscriptionType === 'credit'
}

const getEffectiveRate = (option: SectionableGroupOption): number => {
  return option.userRate ?? option.rate
}

export const sortGroupOptionsByRate = <T extends SectionableGroupOption>(options: T[]): T[] => {
  return [...options].sort((a, b) => {
    const rateDifference = getEffectiveRate(a) - getEffectiveRate(b)
    if (rateDifference !== 0) return rateDifference
    return a.label.localeCompare(b.label, 'zh-CN')
  })
}

export const buildGroupOptionSections = <T extends SectionableGroupOption>(
  options: T[],
  includeMonthly: boolean
): GroupOptionSection<T>[] => {
  const monthly = includeMonthly
    ? sortGroupOptionsByRate(options.filter(isMonthlyGroupOption))
    : []
  const payg = sortGroupOptionsByRate(options.filter((option) => !isMonthlyGroupOption(option)))

  return [
    ...(monthly.length > 0 ? [{ id: 'monthly' as const, options: monthly }] : []),
    ...(payg.length > 0 ? [{ id: 'payg' as const, options: payg }] : [])
  ]
}
