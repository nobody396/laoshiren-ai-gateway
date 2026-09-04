import { describe, it, expect } from 'vitest'
import type { Group } from '@/types'
import { defaultKeyGroupIds, moveKeyGroup } from '../keyGroupSelection'

describe('multi-group authorization snapshots', () => {
  it('selects current concrete groups only, subscriptions first with stable order', () => {
    const groups = [{ id: 6, platform: 'openai', subscription_type: 'standard' }, { id: 5, platform: 'anthropic', subscription_type: 'subscription' }, { id: 90, platform: 'universal' }, { id: 57, platform: 'gemini', subscription_type: 'standard' }] as Group[]
    const selected = defaultKeyGroupIds(groups)
    expect(selected).toEqual([5, 6, 57])
    groups.push({ id: 61, platform: 'openai' } as Group)
    expect(selected).toEqual([5, 6, 57])
  })
  it('moves one priority without mutating or adding authorization', () => {
    const ids = [6, 5, 57]
    expect(moveKeyGroup(ids, 5, -1)).toEqual([5, 6, 57])
    expect(moveKeyGroup(ids, 57, 1)).toEqual(ids)
    expect(moveKeyGroup(ids, 99, -1)).toEqual(ids)
    expect(ids).toEqual([6, 5, 57])
  })
})
