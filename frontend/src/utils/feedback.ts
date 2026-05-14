import type { FeedbackCategory, FeedbackPriority, FeedbackStatus } from '@/types'

export function feedbackStatusTone(status: FeedbackStatus): 'gray' | 'info' | 'success' | 'warning' | 'danger' {
  switch (status) {
    case 'pending':
      return 'warning'
    case 'processing':
      return 'info'
    case 'replied':
      return 'success'
    case 'closed':
      return 'gray'
  }
}

export function feedbackPriorityTone(priority: FeedbackPriority): 'gray' | 'info' | 'success' | 'warning' | 'danger' {
  switch (priority) {
    case 'low':
      return 'gray'
    case 'normal':
      return 'info'
    case 'high':
      return 'warning'
    case 'urgent':
      return 'danger'
  }
}

export const feedbackCategoryOptions: FeedbackCategory[] = ['bug', 'suggestion', 'complaint', 'other']
export const feedbackStatusOptions: Array<FeedbackStatus | ''> = ['', 'pending', 'processing', 'replied', 'closed']
export const feedbackPriorityOptions: Array<FeedbackPriority | ''> = ['', 'low', 'normal', 'high', 'urgent']

export function toDayStartRFC3339(value: string): string {
  const d = new Date(value)
  d.setHours(0, 0, 0, 0)
  return d.toISOString()
}

export function toDayEndRFC3339(value: string): string {
  const d = new Date(value)
  d.setHours(23, 59, 59, 999)
  return d.toISOString()
}
