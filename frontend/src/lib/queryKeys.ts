/**
 * Claves estables para TanStack Query — evitar strings sueltos en páginas.
 */
export function tenantKey<T extends readonly unknown[]>(tenantId: string, key: T): readonly ['tenant', string, ...T] {
  return ['tenant', tenantId, ...key] as const;
}

export function tenantSlugKey<T extends readonly unknown[]>(tenantSlug: string, key: T): readonly ['tenant-slug', string, ...T] {
  return ['tenant-slug', tenantSlug, ...key] as const;
}

export const queryKeys = {
  ai: {
    conversations: {
      list: (limit: number) => ['ai', 'conversations', 'list', limit] as const,
      detail: (conversationId: string) => ['ai', 'conversations', 'detail', conversationId] as const,
    },
  },
  notifications: {
    preferences: ['notifications', 'preferences'] as const,
    inApp: ['notifications', 'in-app'] as const,
    summary: ['notifications', 'summary'] as const,
  },
  customerMessaging: {
    conversations: ['customer-messaging', 'conversations'] as const,
    campaigns: ['customer-messaging', 'campaigns'] as const,
  },
  scheduling: {
    branches: ['scheduling', 'branches'] as const,
    services: ['scheduling', 'services'] as const,
    resources: (branchId: string | null) => ['scheduling', 'resources', branchId ?? 'all'] as const,
    dashboard: (branchId: string | null, day: string) => ['scheduling', 'dashboard', branchId ?? 'all', day] as const,
    slots: (branchId: string | null, serviceId: string | null, resourceId: string | null, day: string) =>
      ['scheduling', 'slots', branchId ?? 'none', serviceId ?? 'none', resourceId ?? 'any', day] as const,
    bookingsRange: (branchId: string | null, start: string, end: string) =>
      ['scheduling', 'bookings-range', branchId ?? 'none', start, end] as const,
  },
  me: {
    current: ['me', 'current'] as const,
  },
  governance: {
    policies: ['governance', 'policies'] as const,
    watchers: ['governance', 'watchers'] as const,
  },
  rbac: {
    members: (tenantId: string) => ['rbac', 'members', tenantId] as const,
    invites: (tenantId: string) => ['rbac', 'invites', tenantId] as const,
    roles: ['rbac', 'roles'] as const,
    permissions: (userId: string) => ['rbac', 'permissions', userId] as const,
  },
  tenant: {
    settings: ['tenant', 'settings'] as const,
  },
  audit: {
    entries: ['audit', 'entries'] as const,
  },
  session: {
    current: ['session', 'current'] as const,
  },
  carWorkOrders: {
    kanban: (archived: boolean) => ['car-work-orders', 'kanban', archived ? 'archived' : 'active'] as const,
    crudConfig: ['car-work-orders', 'crud-config'] as const,
  },
  products: {
    crudConfig: ['products', 'crud-config'] as const,
  },
  modules: {
    isCrud: (moduleId: string) => ['modules', 'is-crud', moduleId] as const,
  },
} as const;
