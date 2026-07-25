export interface RoutePolicyInput {
  path: string
  requiresAuth: boolean
  requiresAdmin: boolean
  requiresAgent: boolean
  isAuthenticated: boolean
  isAdmin: boolean
  role?: string
  isSimpleMode: boolean
  backendModeEnabled: boolean
  permissionAllowed: boolean
  invoiceManagementEnabled: boolean
  feedbackManagementEnabled: boolean
  requiresInvoiceManagement: boolean
  requiresFeedbackManagement: boolean
}

export interface RoutePolicyDecision {
  allow: boolean
  redirect?: string
  preserveIntent?: boolean
}

export const BACKEND_MODE_ALLOWED_PREFIXES = ['/login', '/key-usage', '/setup', '/docs', '/legal', '/changelog']
export const BACKEND_MODE_EXACT_PATHS = ['/', '/home']
export const SIMPLE_MODE_RESTRICTED_PREFIXES = [
  '/admin/groups', '/admin/subscriptions', '/admin/redeem', '/admin/billing',
  '/subscriptions', '/redeem'
]

function dashboard(input: Pick<RoutePolicyInput, 'isAdmin'>): string {
  return input.isAdmin ? '/admin/dashboard' : '/dashboard'
}

function backendPathAllowed(path: string): boolean {
  return BACKEND_MODE_EXACT_PATHS.includes(path) ||
    BACKEND_MODE_ALLOWED_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix))
}

export function evaluateRoutePolicy(input: RoutePolicyInput): RoutePolicyDecision {
  if (!input.requiresAuth) {
    if (input.isAuthenticated && (input.path === '/login' || input.path === '/register')) {
      if (input.backendModeEnabled && !input.isAdmin) return { allow: true }
      return { allow: false, redirect: dashboard(input) }
    }
    if (input.backendModeEnabled && !input.isAuthenticated && !backendPathAllowed(input.path)) {
      return { allow: false, redirect: '/login' }
    }
    return { allow: true }
  }

  if (!input.isAuthenticated) return { allow: false, redirect: '/login', preserveIntent: true }
  if (input.requiresAdmin && !input.isAdmin) return { allow: false, redirect: '/dashboard' }
  if (input.requiresAdmin && input.isAdmin && !input.permissionAllowed) {
    return { allow: false, redirect: '/admin/dashboard' }
  }
  if (input.requiresAgent && !input.isAdmin && input.role !== 'agent') {
    return { allow: false, redirect: '/dashboard' }
  }
  if (input.requiresInvoiceManagement && !input.invoiceManagementEnabled) {
    return { allow: false, redirect: dashboard(input) }
  }
  if (input.requiresFeedbackManagement && !input.feedbackManagementEnabled) {
    return { allow: false, redirect: dashboard(input) }
  }
  if (input.isSimpleMode && SIMPLE_MODE_RESTRICTED_PREFIXES.some((prefix) => input.path.startsWith(prefix))) {
    return { allow: false, redirect: dashboard(input) }
  }
  if (input.backendModeEnabled && !(input.isAdmin || backendPathAllowed(input.path))) {
    return { allow: false, redirect: '/login' }
  }
  return { allow: true }
}
