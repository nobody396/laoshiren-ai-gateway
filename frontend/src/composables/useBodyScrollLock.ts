import { onScopeDispose, toValue, watch, type MaybeRefOrGetter } from 'vue'

const activeLocks = new Set<symbol>()
let overflowBeforeFirstLock: string | null = null

function acquireBodyScrollLock(token: symbol): void {
  if (typeof document === 'undefined' || activeLocks.has(token)) return

  if (activeLocks.size === 0) {
    overflowBeforeFirstLock = document.body.style.overflow
  }
  activeLocks.add(token)
  document.body.style.overflow = 'hidden'
}

function releaseBodyScrollLock(token: symbol): void {
  if (typeof document === 'undefined' || !activeLocks.delete(token)) return

  if (activeLocks.size === 0) {
    document.body.style.overflow = overflowBeforeFirstLock ?? ''
    overflowBeforeFirstLock = null
  }
}

/**
 * Locks document scrolling while a modal-like surface is open.
 *
 * Locks are reference-counted so one overlay cannot re-enable page scrolling
 * while another overlay is still active. The previous inline overflow value is
 * restored when the final consumer releases its lock.
 */
export function useBodyScrollLock(locked: MaybeRefOrGetter<boolean>): void {
  const token = Symbol('body-scroll-lock')

  watch(
    () => toValue(locked),
    (shouldLock) => {
      if (shouldLock) {
        acquireBodyScrollLock(token)
      } else {
        releaseBodyScrollLock(token)
      }
    },
    { immediate: true }
  )

  onScopeDispose(() => releaseBodyScrollLock(token))
}
