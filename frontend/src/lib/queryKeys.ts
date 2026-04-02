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
  appointments: {
    list: ['appointments', 'list'] as const,
  },
  notifications: {
    preferences: ['notifications', 'preferences'] as const,
    inApp: ['notifications', 'in-app'] as const,
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
  teachers: {
    previewBootstrap: ['teachers', 'preview', 'bootstrap'] as const,
    publicBySlug: (slug: string) => ['teachers', 'public', slug] as const,
  },
  workOrders: {
    kanban: (archived: boolean) => ['work-orders', 'kanban', archived ? 'archived' : 'active'] as const,
    crudConfig: ['work-orders', 'crud-config'] as const,
  },
  modules: {
    isCrud: (moduleId: string) => ['modules', 'is-crud', moduleId] as const,
  },
} as const;
