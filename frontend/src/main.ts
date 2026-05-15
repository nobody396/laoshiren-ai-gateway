import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n, registerLocaleChangeHandler } from './i18n'
import { useAppStore } from '@/stores/app'
import { resolveDocumentTitle } from './router/title'
import { vPermission } from './directives/permission'
import './style.css'

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded.
  document.title = resolveDocumentTitle('AI 编码中转', appStore.siteName, undefined, { siteNameFirst: true })

  await initI18n()

  app.use(router)
  app.use(i18n)
  app.directive('permission', vPermission)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()

  registerLocaleChangeHandler(() => {
    const route = router.currentRoute.value
    document.title = resolveDocumentTitle(route.meta.title, appStore.siteName, route.meta.titleKey as string, {
      siteNameFirst: route.meta.titleSiteNameFirst === true
    })
  })

  app.mount('#app')
}

bootstrap()
