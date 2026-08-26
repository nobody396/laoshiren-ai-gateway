/**
 * 页内锚点平滑滚动:不写 URL hash。
 * 之前用 $router.push({ path: '/', hash }) —— 锚点会留在地址栏,
 * 用户刷新时浏览器直接跳到页面中段(像"刷新就到最下面")。
 * 现在只滚动、不碰地址栏,刷新永远回到顶部。
 */
export function scrollToHash(hash: string): void {
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  document.querySelector(hash)?.scrollIntoView({
    behavior: reduce ? 'auto' : 'smooth',
    block: 'start'
  })
}
