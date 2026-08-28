import type { GroupPlatform, SubscriptionType } from '@/types'

export type GroupOptionSectionId = 'monthly' | 'payg'
export type GroupOptionFamilyId = 'openai' | 'claude' | 'grok' | 'gemini' | 'domestic' | 'image' | 'other'

export const GROUP_OPTION_FAMILY_ORDER: GroupOptionFamilyId[] = [
  'openai',
  'claude',
  'grok',
  'gemini',
  'domestic',
  'image',
  'other'
]

export interface SectionableGroupOption {
  label: string
  rate: number
  userRate: number | null
  subscriptionType: SubscriptionType
  platform: GroupPlatform
  familyKey: GroupOptionFamilyId
}

export interface GroupOptionSection<T extends SectionableGroupOption> {
  id: GroupOptionSectionId
  options: T[]
}

export interface GroupOptionFamily<T extends SectionableGroupOption> {
  id: GroupOptionFamilyId
  options: T[]
}

const DOMESTIC_MODEL_NAME = /(?:^|[\s（(·/_-])(glm|kimi|deepseek|qwen|minimax|千问|智谱|月之暗面|文心|混元|豆包)(?:$|[\s（()）·/_.-])/i
const IMAGE_GROUP_NAME = /(?:gpt[\s_-]*image|image[\s_-]*\d|生图|图像生成)/i

export const classifyGroupOptionFamily = (input: Pick<SectionableGroupOption, 'label' | 'platform'>): GroupOptionFamilyId => {
  if (input.platform === 'gpt-image' || IMAGE_GROUP_NAME.test(input.label)) return 'image'
  if (DOMESTIC_MODEL_NAME.test(input.label)) return 'domestic'
  if (input.platform === 'anthropic') return 'claude'
  if (input.platform === 'grok') return 'grok'
  if (input.platform === 'gemini' || input.platform === 'antigravity') return 'gemini'
  if (input.platform === 'openai') return 'openai'
  return 'other'
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
    ...(payg.length > 0 ? [{ id: 'payg' as const, options: payg }] : []),
    ...(monthly.length > 0 ? [{ id: 'monthly' as const, options: monthly }] : [])
  ]
}

export const buildGroupOptionFamilies = <T extends SectionableGroupOption>(options: T[]): GroupOptionFamily<T>[] => {
  return GROUP_OPTION_FAMILY_ORDER.flatMap((family) => {
    const familyOptions = sortGroupOptionsByRate(options.filter((option) => option.familyKey === family))
    return familyOptions.length > 0 ? [{ id: family, options: familyOptions }] : []
  })
}
