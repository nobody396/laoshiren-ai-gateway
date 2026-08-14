/**
 * Type definitions for Vue Router meta fields
 * Extends the RouteMeta interface with custom properties
 */

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /**
     * Whether this route requires authentication
     * @default true
     */
    requiresAuth?: boolean

    /**
     * Whether this route requires admin role
     * @default false
     */
    requiresAdmin?: boolean

    /**
     * RBAC permission key required for this route
     */
    permission?: string

    /**
     * Any one of these RBAC permission keys grants access to the route.
     */
    anyPermission?: string[]

    /**
     * Page title for this route
     */
    title?: string

    /**
     * i18n key used to resolve the page title.
     */
    titleKey?: string

    /**
     * Whether the site name should appear before the page title.
     * @default false
     */
    titleSiteNameFirst?: boolean

    /**
     * Plain text page description used by headers and SEO metadata.
     */
    description?: string

    /**
     * i18n key used to resolve the page description.
     */
    descriptionKey?: string

    /**
     * Markdown doc slug used by public information pages.
     */
    publicDocSlug?: string

    /**
     * Prevent this route from being indexed by search engines.
     * @default false
     */
    noindex?: boolean

    /**
     * Optional breadcrumb items for navigation
     */
    breadcrumbs?: Array<{
      label: string
      to?: string
    }>

    /**
     * Icon name for this route (for sidebar navigation)
     */
    icon?: string

    /**
     * Whether to hide this route from navigation menu
     * @default false
     */
    hideInMenu?: boolean

    /**
     * Whether this route requires the user-facing invoice management switch.
     * @default false
     */
    requiresInvoiceManagement?: boolean

    /**
     * Whether this route requires the user-facing feedback switch.
     * @default false
     */
    requiresFeedbackManagement?: boolean
  }
}
