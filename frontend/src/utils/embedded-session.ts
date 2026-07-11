import { issueEmbedTicket, type EmbedDelivery, type EmbedTargetKind } from '@/api/auth'
import { buildEmbeddedUrl } from './embedded-url'

export interface EmbedTarget {
  kind: EmbedTargetKind
  id: string
}

/**
 * Builds a one-time, audience-bound launch URL for a ticket-compatible staging
 * consumer. Callers must not fall back to an access JWT when this fails.
 */
export async function createTicketedEmbedUrl(
  target: EmbedTarget,
  delivery: EmbedDelivery,
  theme: 'light' | 'dark',
  lang?: string,
): Promise<string> {
  const issued = await issueEmbedTicket(target.kind, target.id, delivery)
  const safeUrl = buildEmbeddedUrl(issued.target_url, theme, lang)
  if (!safeUrl) throw new Error('Embed target is unavailable')

  const url = new URL(safeUrl)
  url.searchParams.set('embed_ticket', issued.ticket)
  url.searchParams.set('embed_delivery', delivery)
  return url.toString()
}
