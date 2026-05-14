const DEFAULT_TABLE_PAGE_SIZE = 20
const TABLE_PAGE_SIZE_STORAGE_KEY = 'dragoncode.table.page_size'

export function getPersistedPageSize(fallback = DEFAULT_TABLE_PAGE_SIZE): number {
  if (typeof window === 'undefined') {
    return fallback
  }

  const raw = window.localStorage.getItem(TABLE_PAGE_SIZE_STORAGE_KEY)
  const parsed = raw ? Number.parseInt(raw, 10) : Number.NaN

  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed
  }

  return fallback
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') {
    return
  }
  if (!Number.isFinite(size) || size <= 0) {
    return
  }
  window.localStorage.setItem(TABLE_PAGE_SIZE_STORAGE_KEY, String(size))
}
