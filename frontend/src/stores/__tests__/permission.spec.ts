import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePermissionStore } from '@/stores/permission'

const mockGetMyPermissions = vi.fn()
const mockGetMyMenuTree = vi.fn()

vi.mock('@/api/admin/rbac', () => ({
  default: {
    getMyPermissions: (...args: any[]) => mockGetMyPermissions(...args),
    getMyMenuTree: (...args: any[]) => mockGetMyMenuTree(...args),
  },
}))

describe('usePermissionStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('权限加载失败时失败关闭, 不授予超级管理员', async () => {
    mockGetMyPermissions.mockRejectedValue(new Error('forbidden'))
    mockGetMyMenuTree.mockRejectedValue(new Error('forbidden'))
    const store = usePermissionStore()

    await store.fetchPermissions()

    expect(store.loaded).toBe(true)
    expect(store.isSuperAdmin).toBe(false)
    expect(store.permissionKeys).toEqual([])
    expect(store.menuTree).toEqual([])
    expect(store.hasPermission('admin:announcements')).toBe(false)
  })

  it('权限加载成功时按接口结果授予权限', async () => {
    mockGetMyPermissions.mockResolvedValue(['admin:announcements'])
    mockGetMyMenuTree.mockResolvedValue([
      { id: 1, name: '公告管理', type: 'menu', path: '/admin/announcements', sort_order: 1 },
    ])
    const store = usePermissionStore()

    await store.fetchPermissions()

    expect(store.loaded).toBe(true)
    expect(store.isSuperAdmin).toBe(false)
    expect(store.hasPermission('admin:announcements')).toBe(true)
    expect(store.hasPermission('admin:users')).toBe(false)
  })

  it('reset 后忽略旧权限请求返回', async () => {
    let resolvePermissions!: (value: string[]) => void
    let resolveMenu!: (value: any[]) => void
    mockGetMyPermissions.mockReturnValue(new Promise((resolve) => {
      resolvePermissions = resolve
    }))
    mockGetMyMenuTree.mockReturnValue(new Promise((resolve) => {
      resolveMenu = resolve
    }))
    const store = usePermissionStore()

    const pending = store.fetchPermissions()
    store.reset()
    resolvePermissions(['*'])
    resolveMenu([{ id: 1, name: '用户管理', type: 'menu', path: '/admin/users', sort_order: 1 }])
    await pending

    expect(store.loaded).toBe(false)
    expect(store.isSuperAdmin).toBe(false)
    expect(store.permissionKeys).toEqual([])
    expect(store.menuTree).toEqual([])
  })
})
