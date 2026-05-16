/**
 * Vue Router configuration for laoshirenai frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import {createRouter, createWebHistory, type RouteRecordRaw} from 'vue-router'
import {useAuthStore} from '@/stores/auth'
import {useAppStore} from '@/stores/app'
import {useAdminSettingsStore} from '@/stores/adminSettings'
import {usePermissionStore} from '@/stores/permission'
import {useNavigationLoadingState} from '@/composables/useNavigationLoading'
import {useRoutePrefetch} from '@/composables/useRoutePrefetch'
import {resolveDocumentTitle} from './title'

/**
 * Route definitions with lazy loading
 */
const routes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup',
      titleKey: 'setup.title'
    }
  },

  // ==================== Public Routes ====================
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'AI 编码中转',
      titleSiteNameFirst: true
    }
  },
  {
    path: '/home',
    redirect: '/'
  },

  // ==================== Documentation Routes ====================
  {
    path: '/docs',
    name: 'Docs',
    component: () => import('@/views/docs/DocsView.vue'),
    meta: {
      requiresAuth: false,
      title: '文档'
    }
  },
  {
    path: '/docs/:slug',
    name: 'DocsPage',
    component: () => import('@/views/docs/DocsView.vue'),
    meta: {
      requiresAuth: false,
      title: '文档'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'common.login'
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email',
      titleKey: 'auth.verifyYourEmail'
    }
  },
  {
    path: '/auth/callback',
    name: 'OAuthCallback',
    component: () => import('@/views/auth/OAuthCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OAuth Callback',
      titleKey: 'auth.oauthCallbackTitle'
    }
  },
  {
    path: '/auth/linuxdo/callback',
    name: 'LinuxDoOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'LinuxDo OAuth Callback',
      titleKey: 'auth.linuxdo.callbackTitle'
    }
  },
  {
    path: '/auth/google/callback',
    name: 'GoogleOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Google OAuth Callback',
      titleKey: 'auth.oauth.callbackTitle'
    }
  },
  {
    path: '/auth/github/callback',
    name: 'GitHubOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'GitHub OAuth Callback',
      titleKey: 'auth.oauth.callbackTitle'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password',
      titleKey: 'auth.resetPasswordTitle'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
      titleKey: 'keyUsage.title',
      descriptionKey: 'keyUsage.subtitle',
    }
  },

  // ==================== User Routes ====================
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/user/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Dashboard',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage'
    }
  },
  {
    path: '/keys',
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'API Keys',
      titleKey: 'keys.title',
      descriptionKey: 'keys.description'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage Records',
      titleKey: 'usage.title',
      descriptionKey: 'usage.description'
    }
  },
  {
    path: '/redeem',
    name: 'Redeem',
    component: () => import('@/views/user/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Redeem Code',
      titleKey: 'redeem.title',
      descriptionKey: 'redeem.description'
    }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Profile',
      titleKey: 'profile.title',
      descriptionKey: 'profile.description'
    }
  },
  {
    path: '/subscriptions',
    name: 'Subscriptions',
    component: () => import('@/views/user/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Subscriptions',
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description'
    }
  },
  {
    path: '/purchase',
    name: 'PurchaseSubscription',
    component: () => import('@/views/user/PurchaseSubscriptionView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Purchase Subscription',
      titleKey: 'purchase.title',
      descriptionKey: 'purchase.description'
    }
  },
  {
    path: '/get-subscription',
    name: 'GetSubscription',
    component: () => import('@/views/user/GetSubscriptionView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Top Up',
      titleKey: 'topup.title',
      descriptionKey: 'subscriptionAccess.description'
    }
  },
  {
    path: '/topup/orders',
    name: 'TopupOrders',
    component: () => import('@/views/user/TopupOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Top-up Orders',
      titleKey: 'topupOrdersPage.title',
      descriptionKey: 'topupOrdersPage.description'
    }
  },
  {
    path: '/invoice',
    name: 'InvoiceManagement',
    component: () => import('@/views/user/InvoiceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Invoice Management',
      titleKey: 'invoicePage.title',
      descriptionKey: 'invoicePage.description'
    }
  },
  {
    path: '/feedbacks',
    name: 'Feedbacks',
    component: () => import('@/views/user/FeedbackListView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Feedback',
      titleKey: 'feedback.title',
      descriptionKey: 'feedback.description'
    }
  },
  {
    path: '/feedbacks/new',
    name: 'FeedbackCreate',
    component: () => import('@/views/user/FeedbackCreateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Submit Feedback',
      titleKey: 'feedback.form.submit'
    }
  },
  {
    path: '/feedbacks/:id',
    name: 'FeedbackDetail',
    component: () => import('@/views/user/FeedbackDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Feedback Detail',
      titleKey: 'feedback.detail.title'
    }
  },
  {
    path: '/feedbacks/:id/edit',
    name: 'FeedbackEdit',
    component: () => import('@/views/user/FeedbackEditView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Edit Feedback',
      titleKey: 'feedback.edit.title'
    }
  },
  {
    path: '/custom/:id',
    name: 'CustomPage',
    component: () => import('@/views/user/CustomPageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Custom Page',
      titleKey: 'customPage.title',
    }
  },

  // ==================== Agent Routes ====================
  {
    path: '/agent',
    redirect: '/agent/dashboard'
  },
  {
    path: '/agent/dashboard',
    name: 'AgentDashboard',
    component: () => import('@/views/agent/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAgent: true,
      title: 'Agent Dashboard',
      titleKey: 'agent.dashboard'
    }
  },
  {
    path: '/agent/users',
    name: 'AgentUsers',
    component: () => import('@/views/agent/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAgent: true,
      title: 'Invited Users',
      titleKey: 'agent.invitedUsers'
    }
  },
  {
    path: '/agent/commissions',
    name: 'AgentCommissions',
    component: () => import('@/views/agent/CommissionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAgent: true,
      title: 'Commission Records',
      titleKey: 'agent.commissionRecords'
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:ops',
      title: 'Ops Monitoring',
      titleKey: 'admin.ops.title',
      descriptionKey: 'admin.ops.description'
    }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:users',
      title: 'User Management',
      titleKey: 'admin.users.title',
      descriptionKey: 'admin.users.description'
    }
  },
  {
    path: '/admin/agents',
    name: 'AdminAgents',
    component: () => import('@/views/admin/AgentsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:agents',
      title: 'Agent Management',
      titleKey: 'admin.agents.title',
      descriptionKey: 'admin.agents.description'
    }
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:groups',
      title: 'Group Management',
      titleKey: 'admin.groups.title',
      descriptionKey: 'admin.groups.description'
    }
  },
  {
    path: '/admin/channels',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:channels',
      title: 'Channel Management',
      titleKey: 'admin.channels.title',
      descriptionKey: 'admin.channels.description'
    }
  },
  {
    path: '/admin/subscriptions',
    name: 'AdminSubscriptions',
    component: () => import('@/views/admin/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:subscriptions',
      title: 'Subscription Management',
      titleKey: 'admin.subscriptions.title',
      descriptionKey: 'admin.subscriptions.description'
    }
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:accounts',
      title: 'Account Management',
      titleKey: 'admin.accounts.title',
      descriptionKey: 'admin.accounts.description'
    }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:announcements',
      title: 'Announcements',
      titleKey: 'admin.announcements.title',
      descriptionKey: 'admin.announcements.description'
    }
  },
  {
    path: '/admin/feedbacks',
    name: 'AdminFeedbacks',
    component: () => import('@/views/admin/FeedbacksView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:feedbacks',
      title: 'Feedback Management',
      titleKey: 'feedback.admin.title',
      descriptionKey: 'feedback.admin.description'
    }
  },
  {
    path: '/admin/feedbacks/:id',
    name: 'AdminFeedbackDetail',
    component: () => import('@/views/admin/FeedbackDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:feedbacks',
      title: 'Feedback Detail',
      titleKey: 'feedback.admin.detailTitle'
    }
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:proxies',
      title: 'Proxy Management',
      titleKey: 'admin.proxies.title',
      descriptionKey: 'admin.proxies.description'
    }
  },
  {
    path: '/admin/redeem',
    name: 'AdminRedeem',
    component: () => import('@/views/admin/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:redeem',
      title: 'Redeem Code Management',
      titleKey: 'admin.redeem.title',
      descriptionKey: 'admin.redeem.description'
    }
  },
  {
    path: '/admin/promo-codes',
    name: 'AdminPromoCodes',
    component: () => import('@/views/admin/PromoCodesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:promo-codes',
      title: 'Promo Code Management',
      titleKey: 'admin.promo.title',
      descriptionKey: 'admin.promo.description'
    }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:settings',
      title: 'System Settings',
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description'
    }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:usage',
      title: 'Usage Records',
      titleKey: 'admin.usage.title',
      descriptionKey: 'admin.usage.description'
    }
  },
  {
    path: '/admin/topup-orders',
    name: 'AdminTopupOrders',
    component: () => import('@/views/admin/TopupOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:topup-orders',
      title: 'Top-up Order Management',
      titleKey: 'admin.topupOrders.title',
      descriptionKey: 'admin.topupOrders.description'
    }
  },
  {
    path: '/admin/invoice-requests',
    name: 'AdminInvoiceRequests',
    component: () => import('@/views/admin/InvoiceRequestsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:invoice-requests',
      title: 'Invoice Request Management',
      titleKey: 'admin.invoiceRequests.title',
      descriptionKey: 'admin.invoiceRequests.description'
    }
  },
  {
    path: '/admin/roles',
    name: 'AdminRoles',
    component: () => import('@/views/admin/RolesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:roles',
      title: 'Role Management',
      titleKey: 'admin.rbac.roles'
    }
  },
  {
    path: '/admin/menus',
    name: 'AdminMenus',
    component: () => import('@/views/admin/MenusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:menus',
      title: 'Menu Management',
      titleKey: 'admin.rbac.menus'
    }
  },
  {
    path: '/admin/apis',
    name: 'AdminApis',
    component: () => import('@/views/admin/ApisView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:apis',
      title: 'API Management',
      titleKey: 'admin.rbac.apis'
    }
  },
  {
    path: '/admin/permissions',
    redirect: '/admin/menus'
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found',
      titleKey: 'errors.pageNotFound'
    }
  }
]

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    // Scroll to saved position when using browser back/forward
    if (savedPosition) {
      return savedPosition
    }
    // Scroll to top for new routes
    return { top: 0 }
  }
})

/**
 * Navigation guard: Authentication check
 */
let authInitialized = false

// 初始化导航加载状态和预加载
const navigationLoading = useNavigationLoadingState()
// 延迟初始化预加载，传入 router 实例
let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
const BACKEND_MODE_ALLOWED_PATHS = ['/login', '/key-usage', '/setup', '/docs']
const BACKEND_MODE_EXACT_PATHS = ['/', '/home']

router.beforeEach(async (to, _from, next) => {
  // 开始导航加载状态
  navigationLoading.startNavigation()

  const authStore = useAuthStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  // Set page title
  const appStore = useAppStore()
  // For custom pages, use menu item label as document title
  if (to.name === 'CustomPage') {
    const id = to.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const adminSettingsStore = useAdminSettingsStore()
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) {
      const siteName = appStore.siteName || '老实人 AI'
      document.title = `${menuItem.label} - ${siteName}`
    } else {
      document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string, {
        siteNameFirst: to.meta.titleSiteNameFirst === true
      })
    }
  } else {
    document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string, {
      siteNameFirst: to.meta.titleSiteNameFirst === true
    })
  }

  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true
  const requiresAdmin = to.meta.requiresAdmin === true

  // If route doesn't require auth, allow access
  if (!requiresAuth) {
    // If already authenticated and trying to access login/register, redirect to appropriate dashboard
    if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
      // In backend mode, non-admin users should NOT be redirected away from login
      // (they are blocked from all protected routes, so redirecting would cause a loop)
      if (appStore.backendModeEnabled && !authStore.isAdmin) {
        next()
        return
      }
      // Admin users go to admin dashboard, regular users go to user dashboard
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
    // Backend mode: block public pages for unauthenticated users (except login, key-usage, setup)
    if (appStore.backendModeEnabled && !authStore.isAuthenticated) {
      const isAllowed = BACKEND_MODE_EXACT_PATHS.includes(to.path) || BACKEND_MODE_ALLOWED_PATHS.some((p) => to.path === p || to.path.startsWith(p))
      if (!isAllowed) {
        next('/login')
        return
      }
    }
    next()
    return
  }

  // Route requires authentication
  if (!authStore.isAuthenticated) {
    // Not authenticated, redirect to login
    next({
      path: '/login',
      query: { redirect: to.fullPath } // Save intended destination
    })
    return
  }

  // Check admin requirement
  if (requiresAdmin && !authStore.isAdmin) {
    // User is authenticated but not admin, redirect to user dashboard
    next('/dashboard')
    return
  }

  if (requiresAdmin && authStore.isAdmin) {
    const requiredPerm = to.meta.permission
    if (requiredPerm) {
      const permissionStore = usePermissionStore()
      if (!permissionStore.loaded) {
        await permissionStore.fetchPermissions()
      }
      if (!permissionStore.hasPermission(requiredPerm)) {
        next('/admin/dashboard')
        return
      }
    }
  }

  // Check agent requirement (agent or admin can access)
  const requiresAgent = to.meta.requiresAgent === true
  if (requiresAgent && !authStore.isAdmin && authStore.user?.role !== 'agent') {
    next('/dashboard')
    return
  }

  // 简易模式下限制访问某些页面
  if (authStore.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem'
    ]

    if (restrictedPaths.some((path) => to.path.startsWith(path))) {
      // 简易模式下访问受限页面,重定向到仪表板
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
  }

  // Backend mode: admin gets full access, non-admin blocked
  if (appStore.backendModeEnabled) {
    if (authStore.isAuthenticated && authStore.isAdmin) {
      next()
      return
    }
    const isAllowed = BACKEND_MODE_EXACT_PATHS.includes(to.path) || BACKEND_MODE_ALLOWED_PATHS.some((p) => to.path === p || to.path.startsWith(p))
    if (!isAllowed) {
      next('/login')
      return
    }
  }

  // All checks passed, allow navigation
  next()
})

/**
 * Navigation guard: End loading and trigger prefetch
 */
router.afterEach((to) => {
  // 结束导航加载状态
  navigationLoading.endNavigation()

  // 懒初始化预加载（首次导航时创建，传入 router 实例）
  if (!routePrefetch) {
    routePrefetch = useRoutePrefetch(router)
  }
  // 触发路由预加载（在浏览器空闲时执行）
  routePrefetch.triggerPrefetch(to)
})

/**
 * Navigation guard: Error handling
 * Handles dynamic import failures caused by deployment updates
 */
router.onError((error) => {
  console.error('Router error:', error)

  // Check if this is a dynamic import failure (chunk loading error)
  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk') ||
    error.message?.includes('Loading CSS chunk') ||
    error.name === 'ChunkLoadError'

  if (isChunkLoadError) {
    // Avoid infinite reload loop by checking sessionStorage
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()

    // Allow reload if never attempted or more than 10 seconds ago
    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      console.warn('Chunk load error detected, reloading page to fetch latest version...')
      window.location.reload()
    } else {
      console.error('Chunk load error persists after reload. Please clear browser cache.')
    }
  }
})

export default router
