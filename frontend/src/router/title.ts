import { i18n } from '@/i18n'

/**
 * 统一生成页面标题，避免多处写入 document.title 产生覆盖冲突。
 * 优先使用 titleKey 通过 i18n 翻译，fallback 到静态 routeTitle。
 */
type TitleOptions = {
  siteNameFirst?: boolean
}

export function resolveDocumentTitle(
  routeTitle: unknown,
  siteName?: string,
  titleKey?: string,
  options: TitleOptions = {}
): string {
  const normalizedSiteName = typeof siteName === 'string' && siteName.trim() ? siteName.trim() : '老实人AI'

  if (typeof titleKey === 'string' && titleKey.trim()) {
    const translated = i18n.global.t(titleKey)
    if (translated && translated !== titleKey) {
      return options.siteNameFirst ? `${normalizedSiteName} - ${translated}` : `${translated} - ${normalizedSiteName}`
    }
  }

  if (typeof routeTitle === 'string' && routeTitle.trim()) {
    const normalizedRouteTitle = routeTitle.trim()
    return options.siteNameFirst ? `${normalizedSiteName} - ${normalizedRouteTitle}` : `${normalizedRouteTitle} - ${normalizedSiteName}`
  }

  return normalizedSiteName
}
