/**
 * Claves estables para TanStack Query — evitar strings sueltos en páginas.
 */
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
  review: {
    policies: ['review', 'policies'] as const,
    watchers: ['review', 'watchers'] as const,
  },
  rbac: {
    members: (orgId: string) => ['rbac', 'members', orgId] as const,
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
  workOrders: {
    kanban: (archived: boolean) => ['work-orders', 'kanban', archived ? 'archived' : 'active'] as const,
    crudConfig: ['work-orders', 'crud-config'] as const,
  },
  modules: {
    isCrud: (moduleId: string) => ['modules', 'is-crud', moduleId] as const,
  },
} as const;
