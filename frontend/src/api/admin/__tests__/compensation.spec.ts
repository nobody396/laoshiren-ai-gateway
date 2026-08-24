import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post: mocks.post } }))

import { executeCompensationDraft } from '../compensation'

describe('compensation execution API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns the durable partial batch carried by a conflict response', async () => {
    const partial = {
      draft_id: 10,
      approval_id: 9,
      state: 'partial' as const,
      verified_count: 1,
      failed_count: 1,
      notices_created: 0,
      executions: [],
    }
    mocks.post.mockRejectedValueOnce({ response: { data: { data: partial } } })

    await expect(executeCompensationDraft(10)).resolves.toEqual(partial)
  })

  it('does not hide a conflict that has no resumable batch', async () => {
    const failure = new Error('permission disabled')
    mocks.post.mockRejectedValueOnce(failure)

    await expect(executeCompensationDraft(10)).rejects.toBe(failure)
  })
})
