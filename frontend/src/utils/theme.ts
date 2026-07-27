/**
 * 亮/暗主题的单一判定入口。
 *
 * 这段逻辑此前在 main.ts、AppSidebar.vue、KeyUsageView.vue 里各有一份拷贝，
 * 三份都写着「没存过就跟随系统」。改默认值时漏掉任何一份，都会出现「首屏
 * 是浅色、进了控制台变深色」这种自相矛盾的表现。
 */

const STORAGE_KEY = 'theme'

export type ThemePreference = 'light' | 'dark'

/**
 * 首次访问一律浅色。
 *
 * 刻意不读 prefers-color-scheme：这是产品决定，不是技术限制。暖纸色的亮色
 * 皮肤是这套设计的主形态，暗色是可选项；跟随系统会让相当一部分新用户第一
 * 眼看到的不是我们想给的样子。只有用户自己切换过（localStorage 里有值）
 * 才尊重其选择。
 */
export function resolveTheme(): ThemePreference {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'dark' ? 'dark' : 'light'
  } catch {
    // 隐私模式下 localStorage 可能抛异常，此时按默认走
    return 'light'
  }
}

export function isDarkTheme(): boolean {
  return resolveTheme() === 'dark'
}

/** 把主题写到 <html> 上。挂载前调用可避免闪烁。 */
export function applyThemeClass(dark = isDarkTheme()): void {
  document.documentElement.classList.toggle('dark', dark)
}

/** 记住用户的手动选择，并立即生效。 */
export function setTheme(dark: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
  } catch {
    // 存不下就只在本次会话生效，不阻断切换
  }
  applyThemeClass(dark)
}
