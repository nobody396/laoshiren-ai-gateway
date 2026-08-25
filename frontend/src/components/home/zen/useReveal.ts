/**
 * 落地页滚动进场动效(IntersectionObserver fade-up)。
 *
 * 从旧 HomeView 内联实现抽出的可复用版本,约定不变:
 * - 元素挂 `.zen-reveal` 类;容器挂 `home-page--reveal-ready` 门控类后才隐藏,
 *   JS 没跑起来时页面直接是最终样式,不会出现一直看不见的降级事故。
 * - 入场用 animation 而不是 transition —— 组件自己的 `transition:` 简写会把
 *   共享 reveal 类的整条 transition(连同 delay)覆盖掉,这是仓库实测过的坑
 *   (见 AGENTS.md Frontend theming 一节),所以全局样式里走 keyframes。
 * - 兜底只揭开当前视口内的元素,视口外继续交给 observer;只有
 *   IntersectionObserver 不可用时才全量兜底。
 */
import { nextTick, onMounted, onUnmounted, ref, type Ref } from 'vue'

const REVEAL_CLASS = 'zen-reveal'
const VISIBLE_CLASS = 'is-visible'

export function useReveal(root: Ref<HTMLElement | null>) {
  const revealReady = ref(false)
  let observer: IntersectionObserver | null = null
  let revealFallbackTimer: number | null = null

  function revealAllSections(): void {
    root.value?.querySelectorAll(`.${REVEAL_CLASS}`).forEach((node) => {
      node.classList.add(VISIBLE_CLASS)
    })
  }

  function revealVisibleSections(): void {
    const vh = window.innerHeight || 0
    root.value?.querySelectorAll(`.${REVEAL_CLASS}`).forEach((node) => {
      const r = node.getBoundingClientRect()
      if (r.top < vh && r.bottom > 0) node.classList.add(VISIBLE_CLASS)
    })
  }

  onMounted(() => {
    nextTick(() => {
      const el = root.value
      if (!el) return
      const revealNodes = Array.from(el.querySelectorAll(`.${REVEAL_CLASS}`))
      if (revealNodes.length === 0) return

      if (typeof window === 'undefined' || typeof IntersectionObserver === 'undefined') {
        revealAllSections()
        return
      }

      try {
        observer = new IntersectionObserver(
          (entries) => {
            entries.forEach((entry) => {
              if (entry.isIntersecting) {
                entry.target.classList.add(VISIBLE_CLASS)
                observer?.unobserve(entry.target)
              }
            })
          },
          { threshold: 0.12, rootMargin: '0px 0px -48px 0px' }
        )

        // 顺序很关键:先挂门控类 → 双 rAF 确保隐藏态真的被绘制过 → 再启动
        // observer。反了的话首屏元素从来没画过隐藏态,就没有入场过渡。
        revealReady.value = true
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            revealNodes.forEach((node) => observer?.observe(node))
          })
        })
        revealFallbackTimer = window.setTimeout(revealVisibleSections, 1600)
      } catch {
        revealAllSections()
      }
    })
  })

  onUnmounted(() => {
    if (revealFallbackTimer != null) {
      window.clearTimeout(revealFallbackTimer)
    }
    observer?.disconnect()
  })

  return { revealReady }
}
