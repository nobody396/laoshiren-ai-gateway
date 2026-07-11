import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n, registerLocaleChangeHandler } from './i18n'
import { useAppStore } from '@/stores/app'
import { resolveDocumentTitle } from './router/title'
import { updateRouteSeo } from '@/utils/seo'
import { initAnalytics } from '@/utils/analytics'
import { vPermission } from './directives/permission'
import { authSession } from '@/auth'
import { configureApiRuntime } from '@/api/runtime'
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
  authSession.hydrate()
  configureApiRuntime((event) => {
    if (event.type === 'forbidden') {
      appStore.showWarning(i18n.global.t(event.messageKey) as string)
      return
    }
    if (event.type === 'ops-monitoring-disabled') {
      localStorage.setItem('ops_monitoring_enabled_cached', 'false')
      window.dispatchEvent(new CustomEvent('ops-monitoring-disabled'))
      if (router.currentRoute.value.path.startsWith('/admin/ops')) {
        void router.replace('/admin/settings')
      }
      return
    }
    sessionStorage.setItem('auth_expired', '1')
    if (router.currentRoute.value.path !== '/login') {
      void router.replace('/login')
    }
  })
  initAnalytics()

  // Set document title immediately after config is loaded.
  document.title = resolveDocumentTitle('AI 编码网关', appStore.siteName, undefined, { siteNameFirst: true })

  await initI18n()

  app.use(router)
  app.use(i18n)
  app.directive('permission', vPermission)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()

  registerLocaleChangeHandler(() => {
    const route = router.currentRoute.value
    updateRouteSeo(route, {
      siteName: appStore.siteName,
      siteLogo: appStore.siteLogo
    })
  })

  app.mount('#app')
}

bootstrap()
