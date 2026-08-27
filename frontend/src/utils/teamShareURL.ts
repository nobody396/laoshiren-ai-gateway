export function absoluteTeamShareURL(value: string | undefined, origin: string): string {
  const raw = value?.trim()
  if (!raw) return ''
  try {
    return new URL(raw, origin).toString()
  } catch {
    return ''
  }
}
