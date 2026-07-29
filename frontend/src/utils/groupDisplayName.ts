const MONTHLY_GROUP_VERSION_SUFFIX = /\s+V\d+(?=\s*月卡组$)/i

/**
 * Keep internal monthly-card version markers out of customer-facing copy.
 *
 * Group names remain stable in the backend because routing and accounting use
 * them as internal identifiers. The public UI should only expose the product
 * name, not implementation versions such as V3 or V12.
 */
export function publicGroupDisplayName(name: string | null | undefined): string {
  return String(name ?? '').replace(MONTHLY_GROUP_VERSION_SUFFIX, '').trim()
}
