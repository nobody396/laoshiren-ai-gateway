import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTicketedEmbedUrl } from '../embedded-session'
import { issueEmbedTicket } from '@/api/auth'

vi.mock('@/api/auth', () => ({ issueEmbedTicket: vi.fn() }))

describe('embedded-session', () => {
  beforeEach(() => vi.mocked(issueEmbedTicket).mockReset())

  it.each(['iframe', 'new_tab'] as const)('requests a separate %s ticket', async (delivery) => {
    vi.mocked(issueEmbedTicket).mockResolvedValue({
      ticket: `ticket-${delivery}`,
      expires_in: 60,
      target_url: 'https://consumer.example/embed?token=must-be-removed',
      audience: 'https://consumer.example',
    })

    const result = await createTicketedEmbedUrl(
      { kind: 'custom_menu', id: 'reports' },
      delivery,
      'dark',
      'zh-CN',
    )
    const url = new URL(result)
    expect(issueEmbedTicket).toHaveBeenCalledWith('custom_menu', 'reports', delivery)
    expect(url.searchParams.get('embed_ticket')).toBe(`ticket-${delivery}`)
    expect(url.searchParams.get('embed_delivery')).toBe(delivery)
    expect(url.searchParams.has('token')).toBe(false)
  })

  it('never falls back to a raw token after ticket failure', async () => {
    vi.mocked(issueEmbedTicket).mockResolvedValue({
      ticket: 'bounded-ticket',
      expires_in: 60,
      target_url: 'http://insecure.example/embed',
      audience: 'http://insecure.example',
    })
    await expect(
      createTicketedEmbedUrl({ kind: 'purchase_subscription', id: 'purchase' }, 'iframe', 'light'),
    ).rejects.toThrow('Embed target is unavailable')
  })
})
