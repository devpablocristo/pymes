import type { ModuleDefinition } from '../lib/moduleCatalog.types';

type CrudModuleId =
  | 'customers'
  | 'suppliers'
  | 'products'
  | 'priceLists'
  | 'quotes'
  | 'sales'
  | 'purchases'
  | 'procurementRequests'
  | 'procurementPolicies'
  | 'accounts'
  | 'roles'
  | 'parties'
  | 'appointments'
  | 'recurring'
  | 'webhooks'
  | 'professionals'
  | 'specialties'
  | 'intakes'
  | 'sessions'
  | 'workshopVehicles'
  | 'workshopServices'
  | 'workOrders'
  | 'beautyStaff'
  | 'beautySalonServices'
  | 'restaurantDiningAreas'
  | 'restaurantDiningTables';

type CrudModuleDefaults = {
  title: string;
  navLabel: string;
  labelPlural: string;
};

type CrudModuleMeta = Pick<ModuleDefinition, 'group' | 'icon' | 'summary'> &
  Partial<Pick<ModuleDefinition, 'title' | 'navLabel' | 'badge' | 'notes' | 'datasets' | 'actions'>>;

const crudModuleDefaults: Record<CrudModuleId, CrudModuleDefaults> = {
  customers: { title: 'Clientes', navLabel: 'Clientes', labelPlural: 'clientes' },
  suppliers: { title: 'Proveedores', navLabel: 'Proveedores', labelPlural: 'proveedores' },
  products: { title: 'Productos', navLabel: 'Productos', labelPlural: 'productos' },
  priceLists: { title: 'Listas de precios', navLabel: 'Listas de precios', labelPlural: 'listas de precios' },
  quotes: { title: 'Presupuestos', navLabel: 'Presupuestos', labelPlural: 'presupuestos' },
  sales: { title: 'Ventas', navLabel: 'Ventas', labelPlural: 'ventas' },
  purchases: { title: 'Compras', navLabel: 'Compras', labelPlural: 'compras' },
  procurementRequests: {
    title: 'Solicitudes de compra internas',
    navLabel: 'Solicitudes compra',
    labelPlural: 'solicitudes de compra internas',
  },
  procurementPolicies: {
    title: 'Políticas de compras (governance)',
    navLabel: 'Políticas compras',
    labelPlural: 'políticas de compras',
  },
  accounts: { title: 'Cuentas corrientes', navLabel: 'Cuentas corrientes', labelPlural: 'cuentas corrientes' },
  roles: { title: 'Roles', navLabel: 'Roles', labelPlural: 'roles' },
  parties: { title: 'Entidades', navLabel: 'Entidades', labelPlural: 'entidades' },
  appointments: { title: 'Turnos', navLabel: 'Turnos', labelPlural: 'turnos' },
  recurring: { title: 'Gastos recurrentes', navLabel: 'Gastos recurrentes', labelPlural: 'gastos recurrentes' },
  webhooks: { title: 'Webhooks', navLabel: 'Webhooks', labelPlural: 'endpoints webhook' },
  professionals: { title: 'Teachers', navLabel: 'Teachers', labelPlural: 'teachers' },
  specialties: { title: 'Especialidades', navLabel: 'Especialidades', labelPlural: 'especialidades' },
  intakes: { title: 'Ingresos', navLabel: 'Ingresos', labelPlural: 'ingresos' },
  sessions: { title: 'Sesiones', navLabel: 'Sesiones', labelPlural: 'sesiones' },
  workshopVehicles: { title: 'Vehículos', navLabel: 'Vehículos', labelPlural: 'vehículos' },
  workshopServices: {
    title: 'Servicios de taller',
    navLabel: 'Servicios taller',
    labelPlural: 'servicios de taller',
  },
  workOrders: { title: 'Órdenes de trabajo', navLabel: 'Órdenes trabajo', labelPlural: 'órdenes de trabajo' },
  beautyStaff: { title: 'Equipo', navLabel: 'Equipo', labelPlural: 'equipo' },
  beautySalonServices: {
    title: 'Servicios de salón',
    navLabel: 'Servicios salón',
    labelPlural: 'servicios de salón',
  },
  restaurantDiningAreas: {
    title: 'Zonas del salón',
    navLabel: 'Zonas salón',
    labelPlural: 'zonas del salón',
  },
  restaurantDiningTables: { title: 'Mesas', navLabel: 'Mesas', labelPlural: 'mesas' },
};

const crudModuleMeta: Partial<Record<CrudModuleId, CrudModuleMeta>> = {
  parties: {
    group: 'commercial',
    icon: 'PT',
    summary: 'Vista transversal de personas y organizaciones con roles y relaciones.',
  },
  customers: {
    group: 'commercial',
    icon: 'CL',
    summary: 'Base de clientes con historial comercial e importación/exportación CSV.',
  },
  suppliers: {
    group: 'commercial',
    icon: 'PR',
    summary: 'Catálogo de proveedores y datos de abastecimiento.',
  },
  products: {
    group: 'commercial',
    icon: 'PD',
    summary: 'Catálogo de productos, precios, costos y stock.',
  },
  priceLists: {
    group: 'commercial',
    icon: 'LP',
    summary: 'Manejo de listas activas, default y markups.',
  },
  quotes: {
    group: 'commercial',
    icon: 'QT',
    summary: 'Embudo comercial y conversión de oportunidades a ventas.',
  },
  sales: {
    group: 'commercial',
    icon: 'VT',
    summary: 'Ventas, comprobantes, cobros y seguimiento de estado.',
  },
  purchases: {
    group: 'operations',
    icon: 'CP',
    summary: 'Circuito de compras, recepciones y costos.',
  },
  procurementRequests: {
    group: 'operations',
    icon: 'SC',
    summary: 'Solicitudes internas, aprobación y políticas CEL (governance).',
    datasets: [
      {
        id: 'procurement-requests-active',
        title: 'Solicitudes activas',
        description: 'Listado excluye archivados (mismo contrato que el CRUD).',
        path: '/v1/procurement-requests',
        autoLoad: true,
      },
      {
        id: 'procurement-requests-archived',
        title: 'Solicitudes archivadas',
        description: 'Incluye registros con archived_at; equivale a ?archived=true en el listado.',
        path: '/v1/procurement-requests?archived=true',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'procurement-request-detail',
        title: 'Detalle de solicitud',
        description: 'GET por ID (misma entidad que edita el CRUD).',
        path: '/v1/procurement-requests/:requestId',
        method: 'GET',
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-submit',
        title: 'Enviar a evaluación',
        description: 'Evalúa políticas CEL (governance) y actualiza estado.',
        path: '/v1/procurement-requests/:requestId/submit',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-approve',
        title: 'Aprobar',
        description: 'Solo si el estado lo permite (pending_approval).',
        path: '/v1/procurement-requests/:requestId/approve',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-reject',
        title: 'Rechazar',
        description: 'Solo si el estado lo permite (pending_approval).',
        path: '/v1/procurement-requests/:requestId/reject',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-archive',
        title: 'Archivar',
        description: 'Soft delete; el CRUD usa este endpoint en “Archivar”.',
        path: '/v1/procurement-requests/:requestId/archive',
        method: 'POST',
        response: 'none',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
      {
        id: 'procurement-request-restore',
        title: 'Restaurar',
        description: 'Quita archived_at; usado desde la vista de archivados.',
        path: '/v1/procurement-requests/:requestId/restore',
        method: 'POST',
        response: 'none',
        sendEmptyBody: true,
        fields: [{ name: 'requestId', label: 'ID de solicitud', location: 'path', required: true }],
      },
    ],
  },
  procurementPolicies: {
    group: 'control',
    icon: 'GP',
    title: 'Políticas de compras',
    navLabel: 'Políticas compras',
    summary: 'Reglas CEL por organización para el circuito de solicitudes (motor governance).',
    datasets: [
      {
        id: 'procurement-policies-list',
        title: 'Políticas CEL',
        description: 'Reglas por org; se evalúan al enviar una solicitud (submit).',
        path: '/v1/procurement-policies',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'procurement-policy-detail',
        title: 'Detalle de política',
        description: 'GET por ID.',
        path: '/v1/procurement-policies/:policyId',
        method: 'GET',
        fields: [{ name: 'policyId', label: 'ID de política', location: 'path', required: true }],
      },
    ],
  },
  accounts: {
    group: 'commercial',
    icon: 'CC',
    summary: 'Saldo por entidad, deuda y movimientos de cuentas.',
  },
  appointments: {
    group: 'operations',
    icon: 'TR',
    summary: 'Agenda operativa con filtros por fecha, estado y asignación.',
  },
  recurring: {
    group: 'operations',
    icon: 'GR',
    summary: 'Obligaciones periódicas, frecuencia y próximos vencimientos.',
  },
  roles: {
    group: 'control',
    icon: 'RB',
    title: 'RBAC',
    navLabel: 'RBAC',
    summary: 'Roles, permisos efectivos y asignación operativa.',
  },
  webhooks: {
    group: 'integrations',
    icon: 'WH',
    summary: 'Endpoints, entregas y replay de eventos outbound.',
  },
};

export const crudModuleCatalog: Record<string, ModuleDefinition> = Object.fromEntries(
  Object.entries(crudModuleDefaults).map(([resourceId, defaults]) => {
    const meta = crudModuleMeta[resourceId as CrudModuleId];
    const definition: ModuleDefinition = {
      id: resourceId,
      title: meta?.title ?? defaults.title,
      navLabel: meta?.navLabel ?? defaults.navLabel,
      summary: meta?.summary ?? `Gestión CRUD de ${defaults.labelPlural}.`,
      group: meta?.group ?? 'operations',
      icon: meta?.icon ?? 'CR',
      badge: meta?.badge,
      notes: meta?.notes,
      datasets: meta?.datasets,
      actions: meta?.actions,
    };
    return [resourceId, definition];
  }),
);

export function hasCrudModule(resourceId: string): boolean {
  return resourceId in crudModuleCatalog;
}
