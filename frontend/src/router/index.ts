/**
 * Vue Router configuration for Sub2API frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import {createRouter, createWebHistory, type RouteLocationNormalized, type RouteRecordRaw} from 'vue-router'
import {useAuthStore} from '@/stores/auth'
import {useAppStore} from '@/stores/app'
import {useAdminSettingsStore} from '@/stores/adminSettings'
import {usePermissionStore} from '@/stores/permission'
import {useNavigationLoadingState} from '@/composables/useNavigationLoading'
import {useRoutePrefetch} from '@/composables/useRoutePrefetch'
import {getSetupStatus, type SetupStatus} from '@/api/setup'
import {updateRouteSeo} from '@/utils/seo'
import {trackPageView} from '@/utils/analytics'
import {evaluateRoutePolicy} from './route-policy'

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
      title: 'AI 编码网关',
      description: '老实人AI 提供面向开发者的 AI 编码接口与网关服务，支持 Claude Code、Codex、ChatGPT、Gemini 等主流编码模型，适合快速配置、精确计费和稳定调用。',
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
      title: '文档',
      description: '老实人AI 文档中心提供 Claude Code、Codex、OpenClaw、Hermes、Cherry Studio 和 GPT-Image-2 的配置教程与常见问题。'
    }
  },
  {
    path: '/docs/:slug',
    name: 'DocsPage',
    component: () => import('@/views/docs/DocsView.vue'),
    meta: {
      requiresAuth: false,
      title: '文档',
      description: '老实人AI 文档中心提供 Claude Code、Codex、OpenClaw、Hermes、Cherry Studio 和 GPT-Image-2 的配置教程与常见问题。'
    }
  },
  {
    path: '/enterprise',
    name: 'Enterprise',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '企业 AI API 网关',
      description: '老实人AI 为企业团队提供 Claude Code、Codex、ChatGPT、Gemini 等多模型统一接入、API Key 管理、用量统计、成本控制和技术支持方案。',
      publicDocSlug: 'enterprise-ai-api-gateway'
    }
  },
  {
    path: '/security',
    name: 'Security',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '安全与隐私',
      description: '了解老实人AI 在 API Key、调用日志、客服排查、企业接入和敏感信息处理中的安全与隐私边界。',
      publicDocSlug: 'security'
    }
  },
  {
    path: '/status',
    name: 'Status',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '服务状态',
      description: '查看老实人AI 主站、API 健康检查、模型检测报告和异常反馈入口，用于判断 Claude Code、Codex 等接入链路状态。',
      publicDocSlug: 'sla-support'
    }
  },
  {
    path: '/legal',
    redirect: '/legal/terms'
  },
  {
    path: '/legal/terms',
    name: 'LegalTerms',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '服务条款',
      description: '老实人AI 服务条款，说明账号、API Key、计费、上游服务、地区声明、责任边界和条款更新规则。',
      publicDocSlug: 'legal-terms'
    }
  },
  {
    path: '/legal/usage-policy',
    name: 'LegalUsagePolicy',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '使用政策',
      description: '老实人AI 使用政策，说明禁止行为、安全边界、隐私保护、高风险使用、下游用户管理和违规处理规则。',
      publicDocSlug: 'legal-usage-policy'
    }
  },
  {
    path: '/legal/supported-regions',
    name: 'LegalSupportedRegions',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '支持的国家和地区',
      description: '老实人AI 支持的国家和地区说明，明确中国大陆地区不支持使用以及地区、制裁、出口管制和上游政策限制。',
      publicDocSlug: 'legal-supported-regions'
    }
  },
  {
    path: '/legal/service-specific-terms',
    name: 'LegalServiceSpecificTerms',
    component: () => import('@/views/PublicInfoView.vue'),
    meta: {
      requiresAuth: false,
      title: '服务特定条款',
      description: '老实人AI 服务特定条款，说明模型接入、AI 编码工具、上游凭证、计费、文件数据、Beta 能力和企业管理员责任。',
      publicDocSlug: 'legal-service-specific-terms'
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
    path: '/pricing/subscription',
    name: 'PricingSubscription',
    component: () => import('@/views/user/GetSubscriptionView.vue'),
    meta: {
      requiresAuth: false,
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
    path: '/resources',
    name: 'Resources',
    component: () => import('@/views/user/ResourcesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Download Resources',
      titleKey: 'resources.title',
      descriptionKey: 'resources.description'
    }
  },
  {
    path: '/invoice',
    name: 'InvoiceManagement',
    component: () => import('@/views/user/InvoiceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresInvoiceManagement: true,
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
      requiresFeedbackManagement: true,
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
      requiresFeedbackManagement: true,
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
      requiresFeedbackManagement: true,
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
      requiresFeedbackManagement: true,
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
    path: '/admin/monthly-upstreams',
    name: 'AdminMonthlyUpstreams',
    component: () => import('@/views/admin/MonthlyUpstreamsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:ops',
      title: '月卡监控'
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
    path: '/admin/suppliers',
    name: 'AdminSuppliers',
    component: () => import('@/views/admin/SuppliersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:suppliers',
      title: 'Supplier Evaluation',
      titleKey: 'admin.suppliers.title',
      descriptionKey: 'admin.suppliers.description'
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
    path: '/admin/billing',
    name: 'AdminBilling',
    component: () => import('@/views/admin/BillingView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      permission: 'admin:redeem',
      title: 'Billing Management',
      titleKey: 'admin.billing.title',
      descriptionKey: 'admin.billing.description'
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
  scrollBehavior(to, _from, savedPosition) {
    // Scroll to saved position when using browser back/forward
    if (savedPosition) {
      return savedPosition
    }

    if (to.hash) {
      return {
        el: to.hash,
        top: 88,
        behavior: 'smooth'
      }
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
let cachedSetupStatus: SetupStatus | null = null
let setupStatusPromise: Promise<SetupStatus | null> | null = null

function resolveCustomPageTitle(to: RouteLocationNormalized): string | undefined {
  if (to.name !== 'CustomPage') return undefined

  const authStore = useAuthStore()
  const appStore = useAppStore()
  const adminSettingsStore = useAdminSettingsStore()
  const id = to.params.id as string
  const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
  const menuItem = publicItems.find((item) => item.id === id)
    ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
  return menuItem?.label
}

function isSetupRoutePath(path: string): boolean {
  return path === '/setup' || path.startsWith('/setup/')
}

async function getCachedSetupStatus(): Promise<SetupStatus | null> {
  if (cachedSetupStatus) {
    return cachedSetupStatus
  }
  if (!setupStatusPromise) {
    setupStatusPromise = getSetupStatus()
      .then((status) => {
        cachedSetupStatus = status
        return status
      })
      .catch(() => {
        setupStatusPromise = null
        return null
      })
  }
  return setupStatusPromise
}

router.beforeEach(async (to, _from, next) => {
  // 开始导航加载状态
  navigationLoading.startNavigation()

  const authStore = useAuthStore()
  const appStore = useAppStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  const setupStatus = await getCachedSetupStatus()
  if (setupStatus?.needs_setup && !isSetupRoutePath(to.path)) {
    next('/setup')
    return
  }
  if (setupStatus && !setupStatus.needs_setup && isSetupRoutePath(to.path)) {
    next(authStore.isAuthenticated ? (authStore.isAdmin ? '/admin/dashboard' : '/dashboard') : '/login')
    return
  }

  // Set page title and SEO metadata in one place to avoid duplicate head tags.
  updateRouteSeo(to, {
    siteName: appStore.siteName,
    siteLogo: appStore.siteLogo,
    customTitle: resolveCustomPageTitle(to)
  })

  const requiresAdmin = to.meta.requiresAdmin === true
  const requiredPermission = typeof to.meta.permission === 'string' ? to.meta.permission : undefined
  let permissionAllowed = true
  if (requiresAdmin && authStore.isAdmin && requiredPermission) {
    const permissionStore = usePermissionStore()
    if (!permissionStore.loaded) await permissionStore.fetchPermissions()
    permissionAllowed = permissionStore.hasPermission(requiredPermission)
  }

  const requiresInvoiceManagement = to.meta.requiresInvoiceManagement === true
  const requiresFeedbackManagement = to.meta.requiresFeedbackManagement === true
  if ((requiresInvoiceManagement || requiresFeedbackManagement) && !appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings()
  }

  const decision = evaluateRoutePolicy({
    path: to.path,
    requiresAuth: to.meta.requiresAuth !== false,
    requiresAdmin,
    requiresAgent: to.meta.requiresAgent === true,
    isAuthenticated: authStore.isAuthenticated,
    isAdmin: authStore.isAdmin,
    role: authStore.user?.role,
    isSimpleMode: authStore.isSimpleMode,
    backendModeEnabled: appStore.backendModeEnabled,
    permissionAllowed,
    invoiceManagementEnabled: appStore.cachedPublicSettings?.invoice_management_enabled === true,
    feedbackManagementEnabled: appStore.cachedPublicSettings?.feedback_management_enabled !== false,
    requiresInvoiceManagement,
    requiresFeedbackManagement
  })

  if (!decision.allow && decision.redirect) {
    next(decision.preserveIntent
      ? { path: decision.redirect, query: { redirect: to.fullPath } }
      : decision.redirect)
    return
  }
  next()
})

/**
 * Navigation guard: End loading and trigger prefetch
 */
router.afterEach((to) => {
  // 结束导航加载状态
  navigationLoading.endNavigation()

  trackPageView(to.path, {
    route_name: typeof to.name === 'string' ? to.name : '',
  })

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
