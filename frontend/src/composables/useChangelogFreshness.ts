import { computed, ref } from 'vue'
import { changelogAPI } from '@/api'

const STORAGE_KEY = 'laoshirenai:changelog:last-seen-at'
const latestPublishedAt = ref<string | null>(null)
const initialized = ref(false)

function readLastSeen(): string | null {
  try {
    return window.localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

// Module-scoped so the sidebar updates immediately when the changelog page
// marks the feed as seen.
const lastSeenAt = ref<string | null>(typeof window === 'undefined' ? null : readLastSeen())

export function useChangelogFreshness() {
  const hasNewChangelog = computed(() => {
    if (!latestPublishedAt.value) return false
    if (!lastSeenAt.value) return true
    return new Date(latestPublishedAt.value).getTime() > new Date(lastSeenAt.value).getTime()
  })

  async function refreshChangelogFreshness(force = false) {
    if (initialized.value && !force) return
    try {
      const latest = await changelogAPI.latest()
      latestPublishedAt.value = latest?.published_at ?? null
      initialized.value = true
    } catch (error) {
      console.error('Failed to check changelog freshness:', error)
    }
  }

  function markChangelogSeen(publishedAt?: string | null) {
    const candidate = publishedAt || latestPublishedAt.value || new Date().toISOString()
    const current = lastSeenAt.value
    const value = current && timestampMillis(current) > timestampMillis(candidate)
      ? current
      : candidate
    lastSeenAt.value = value
    latestPublishedAt.value = latestPublishedAt.value || value
    try {
      window.localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // The page remains usable when storage is unavailable.
    }
  }

  return {
    latestPublishedAt,
    hasNewChangelog,
    refreshChangelogFreshness,
    markChangelogSeen
  }
}

function timestampMillis(value: string): number {
  const milliseconds = new Date(value).getTime()
  return Number.isFinite(milliseconds) ? milliseconds : Number.NEGATIVE_INFINITY
}
