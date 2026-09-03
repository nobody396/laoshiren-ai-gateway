import type { Group } from '@/types'

// Select a snapshot of today's eligible groups, never a wildcard for future groups.
export function defaultKeyGroupIds(groups: Group[]): number[] {
  return groups
    .filter((group) => group.platform !== 'universal')
    .map((group, index) => ({ group, index }))
    .sort((a, b) => Number(b.group.subscription_type === 'subscription' || b.group.subscription_type === 'credit') - Number(a.group.subscription_type === 'subscription' || a.group.subscription_type === 'credit') || a.index - b.index)
    .map(({ group }) => group.id)
}

export function moveKeyGroup(ids: number[], id: number, delta: -1 | 1): number[] {
  const next = [...ids]
  const from = next.indexOf(id)
  const to = from + delta
  if (from < 0 || to < 0 || to >= next.length) return next
  ;[next[from], next[to]] = [next[to], next[from]]
  return next
}
