import { describe, expect, it, vi } from 'vitest'
const get = vi.hoisted(() => vi.fn().mockResolvedValue({ data: { items: [], total: 0 } }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { list } from '../users'

describe('admin deleted-user audit filter', () => {
  it('passes an explicit opt-in without changing the default', async () => {
    await list()
    expect(get.mock.lastCall?.[1].params.include_deleted).toBeUndefined()
    await list(1, 20, { include_deleted: true, search: 'deleted@example.com' })
    expect(get.mock.lastCall?.[1].params).toMatchObject({ include_deleted: true, search: 'deleted@example.com' })
    await list(1, 20, { include_deleted: false })
    expect(get.mock.lastCall?.[1].params.include_deleted).toBe(false)
  })
})
