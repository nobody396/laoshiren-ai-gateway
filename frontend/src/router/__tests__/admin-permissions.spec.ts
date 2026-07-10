import { describe, expect, it } from 'vitest'
import router from '@/router'
import { evaluateRoutePolicy, type RoutePolicyInput } from '@/router/route-policy'

const policy = (overrides: Partial<RoutePolicyInput> = {}) => evaluateRoutePolicy({
  path: '/dashboard',
  requiresAuth: true,
  requiresAdmin: false,
  requiresAgent: false,
  isAuthenticated: true,
  isAdmin: false,
  role: 'user',
  isSimpleMode: false,
  backendModeEnabled: false,
  permissionAllowed: true,
  invoiceManagementEnabled: true,
  feedbackManagementEnabled: true,
  requiresInvoiceManagement: false,
  requiresFeedbackManagement: false,
  ...overrides
})

describe('后台路由权限声明', () => {
  it('除后台仪表盘外的后台页面声明 RBAC 权限', () => {
    const expectedPermissions: Record<string, string> = {
      '/admin/ops': 'admin:ops',
      '/admin/monthly-upstreams': 'admin:ops',
      '/admin/users': 'admin:users',
      '/admin/agents': 'admin:agents',
      '/admin/groups': 'admin:groups',
      '/admin/channels': 'admin:channels',
      '/admin/suppliers': 'admin:suppliers',
      '/admin/subscriptions': 'admin:subscriptions',
      '/admin/accounts': 'admin:accounts',
      '/admin/announcements': 'admin:announcements',
      '/admin/feedbacks': 'admin:feedbacks',
      '/admin/feedbacks/:id': 'admin:feedbacks',
      '/admin/proxies': 'admin:proxies',
      '/admin/redeem': 'admin:redeem',
      '/admin/billing': 'admin:redeem',
      '/admin/promo-codes': 'admin:promo-codes',
      '/admin/settings': 'admin:settings',
      '/admin/usage': 'admin:usage',
      '/admin/topup-orders': 'admin:topup-orders',
      '/admin/invoice-requests': 'admin:invoice-requests',
      '/admin/roles': 'admin:roles',
      '/admin/menus': 'admin:menus',
      '/admin/apis': 'admin:apis',
    }

    for (const [path, permission] of Object.entries(expectedPermissions)) {
      expect(router.getRoutes().find((route) => route.path === path)?.meta.permission).toBe(permission)
    }
    expect(router.getRoutes().find((route) => route.path === '/admin/dashboard')?.meta.permission).toBeUndefined()
  })
})

describe('RoutePolicy', () => {
  it('fails closed for an administrator missing the route permission', () => {
    expect(policy({ path: '/admin/users', requiresAdmin: true, isAdmin: true, permissionAllowed: false }))
      .toEqual({ allow: false, redirect: '/admin/dashboard' })
  })

  it('uses the same simple-mode decision for guards and navigation', () => {
    expect(policy({ path: '/subscriptions', isSimpleMode: true }))
      .toEqual({ allow: false, redirect: '/dashboard' })
  })

  it('preserves the intended path only for unauthenticated protected routes', () => {
    expect(policy({ isAuthenticated: false }))
      .toEqual({ allow: false, redirect: '/login', preserveIntent: true })
  })

  it('allows only documented public paths in backend mode', () => {
    expect(policy({ path: '/docs/quickstart', requiresAuth: false, isAuthenticated: false, backendModeEnabled: true }).allow).toBe(true)
    expect(policy({ path: '/pricing', requiresAuth: false, isAuthenticated: false, backendModeEnabled: true }))
      .toEqual({ allow: false, redirect: '/login' })
  })
})
