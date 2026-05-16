import { describe, expect, it } from 'vitest'
import router from '@/router'

describe('后台路由权限声明', () => {
  it('除后台仪表盘外的后台页面声明 RBAC 权限', () => {
    const expectedPermissions: Record<string, string> = {
      '/admin/ops': 'admin:ops',
      '/admin/users': 'admin:users',
      '/admin/agents': 'admin:agents',
      '/admin/groups': 'admin:groups',
      '/admin/channels': 'admin:channels',
      '/admin/subscriptions': 'admin:subscriptions',
      '/admin/accounts': 'admin:accounts',
      '/admin/announcements': 'admin:announcements',
      '/admin/feedbacks': 'admin:feedbacks',
      '/admin/feedbacks/:id': 'admin:feedbacks',
      '/admin/proxies': 'admin:proxies',
      '/admin/redeem': 'admin:redeem',
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
