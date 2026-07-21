import type { ModuleDefinition } from '../lib/moduleCatalog.types';

type CrudModuleId =
  | 'invoices'
  | 'customers'
  | 'suppliers'
  | 'products'
  | 'services'
  | 'priceLists'
  | 'quotes'
  | 'sales'
  | 'returns'
  | 'creditNotes'
  | 'cashflow'
  | 'inventory'
  | 'payments'
  | 'purchases'
  | 'procurementRequests'
  | 'accounts'
  | 'roles'
  | 'parties'
  | 'employees'
  | 'recurring'
  | 'webhooks'
  | 'professionals'
  | 'specialties'
  | 'intakes'
  | 'sessions'
  | 'workshopVehicles'
  | 'carWorkOrders'
  | 'bikeWorkOrders'
  | 'restaurantDiningAreas'
  | 'restaurantDiningTables'
  | 'occupationalHealthExams'
  | 'ledgerAccounts';

type CrudModuleDefaults = {
  title: string;
  navLabel: string;
  labelPlural: string;
};

type CrudModuleMeta = Pick<ModuleDefinition, 'group' | 'icon' | 'summary'> &
  Partial<Pick<ModuleDefinition, 'title' | 'navLabel' | 'badge' | 'notes' | 'datasets' | 'actions' | 'customRoute'>>;

const crudModuleDefaults: Record<CrudModuleId, CrudModuleDefaults> = {
  invoices: { title: 'Facturación', navLabel: 'Facturación', labelPlural: 'facturas' },
  customers: { title: 'Clientes', navLabel: 'Clientes', labelPlural: 'clientes' },
  suppliers: { title: 'Proveedores', navLabel: 'Proveedores', labelPlural: 'proveedores' },
  products: { title: 'Productos', navLabel: 'Productos', labelPlural: 'productos' },
  services: { title: 'Servicios', navLabel: 'Servicios', labelPlural: 'servicios' },
  priceLists: { title: 'Listas de precios', navLabel: 'Listas de precios', labelPlural: 'listas de precios' },
  quotes: { title: 'Presupuestos', navLabel: 'Presupuestos', labelPlural: 'presupuestos' },
  sales: { title: 'Ventas', navLabel: 'Ventas', labelPlural: 'ventas' },
  returns: { title: 'Devoluciones', navLabel: 'Devoluciones', labelPlural: 'devoluciones' },
  creditNotes: { title: 'Notas de crédito', navLabel: 'Notas de crédito', labelPlural: 'notas de crédito' },
  cashflow: { title: 'Caja', navLabel: 'Caja', labelPlural: 'movimientos de caja' },
  inventory: { title: 'Inventario', navLabel: 'Inventario', labelPlural: 'productos en el inventario' },
  payments: { title: 'Pagos', navLabel: 'Pagos', labelPlural: 'pagos' },
  purchases: { title: 'Compras', navLabel: 'Compras', labelPlural: 'compras' },
  procurementRequests: {
    title: 'Solicitudes de compra internas',
    navLabel: 'Solicitudes compra',
    labelPlural: 'solicitudes de compra internas',
  },
  accounts: { title: 'Cuentas corrientes', navLabel: 'Cuentas corrientes', labelPlural: 'cuentas corrientes' },
  roles: { title: 'Roles', navLabel: 'Roles', labelPlural: 'roles' },
  parties: { title: 'Entidades', navLabel: 'Entidades', labelPlural: 'entidades' },
  employees: { title: 'Empleados', navLabel: 'Empleados', labelPlural: 'empleados' },
  recurring: { title: 'Gastos recurrentes', navLabel: 'Gastos recurrentes', labelPlural: 'gastos recurrentes' },
  webhooks: { title: 'Webhooks', navLabel: 'Webhooks', labelPlural: 'endpoints webhook' },
  professionals: { title: 'Profesionales', navLabel: 'Profesionales', labelPlural: 'profesionales' },
  specialties: { title: 'Especialidades', navLabel: 'Especialidades', labelPlural: 'especialidades' },
  intakes: { title: 'Ingresos', navLabel: 'Ingresos', labelPlural: 'ingresos' },
  sessions: { title: 'Sesiones', navLabel: 'Sesiones', labelPlural: 'sesiones' },
  workshopVehicles: { title: 'Vehículos', navLabel: 'Vehículos', labelPlural: 'vehículos' },
  carWorkOrders: { title: 'Órdenes de trabajo', navLabel: 'OT auto', labelPlural: 'órdenes de trabajo' },
  bikeWorkOrders: {
    title: 'Órdenes de trabajo',
    navLabel: 'Órdenes de trabajo',
    labelPlural: 'órdenes de trabajo',
  },
  restaurantDiningAreas: {
    title: 'Zonas del salón',
    navLabel: 'Zonas salón',
    labelPlural: 'zonas del salón',
  },
  restaurantDiningTables: { title: 'Mesas', navLabel: 'Mesas', labelPlural: 'mesas' },
  occupationalHealthExams: {
    title: 'Medicina laboral',
    navLabel: 'Exámenes laborales',
    labelPlural: 'exámenes laborales',
  },
  ledgerAccounts: {
    title: 'Plan de cuentas',
    navLabel: 'Plan de cuentas',
    labelPlural: 'cuentas contables',
  },
};

const crudModuleMeta: Partial<Record<CrudModuleId, CrudModuleMeta>> = {
  parties: {
    group: 'commercial',
    icon: 'PT',
    summary: 'Vista transversal de personas y empresas con roles y relaciones.',
  },
  employees: {
    group: 'commercial',
    icon: 'EM',
    summary:
      'Entidades (parties) con rol empleado. El alta asigna automáticamente el rol «employee». Los usuarios miembros del tenant en la consola se administran aparte.',
  },
  customers: {
    group: 'commercial',
    icon: 'CL',
    summary: 'Base de clientes con historial comercial e importación/exportación CSV.',
  },
  invoices: {
    group: 'commercial',
    icon: 'FC',
    summary: 'Facturas, vista previa, edición y evolución del módulo de billing sobre la shell CRUD.',
    customRoute: '/invoices',
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
  services: {
    group: 'commercial',
    icon: 'SV',
    summary: 'Catálogo horizontal de servicios comerciales con precio y duración base.',
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
  returns: {
    group: 'commercial',
    icon: 'DV',
    summary: 'Devoluciones registradas. La anulación marca la devolución y la nota de crédito asociada.',
  },
  creditNotes: {
    group: 'commercial',
    icon: 'NC',
    summary: 'Notas de crédito emitidas y saldo disponible por documento.',
  },
  cashflow: {
    group: 'operations',
    icon: 'CJ',
    summary: 'Movimientos de caja manuales (ingreso/egreso). Resúmenes agregados en reportes.',
  },
  inventory: {
    group: 'operations',
    icon: 'ST',
    summary: 'Productos en inventario, cantidades, ajustes manuales y movimientos.',
  },
  payments: {
    group: 'commercial',
    icon: 'PG',
    summary: 'Cobros de una venta. Usá ?sale_id=<UUID> en la URL o registrá cobros desde el listado de ventas.',
    notes: ['No existe listado global de pagos en el API; cada listado es GET /v1/sales/:id/payments.'],
  },
  purchases: {
    group: 'operations',
    icon: 'CP',
    summary: 'Circuito de compras, recepciones y costos.',
  },
  procurementRequests: {
    group: 'operations',
    icon: 'SC',
    summary: 'Solicitudes internas adaptadas desde este frontend, con ownership de governance en Nexus.',
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
  accounts: {
    group: 'commercial',
    icon: 'CC',
    summary: 'Saldo por entidad, deuda y movimientos de cuentas.',
  },
  recurring: {
    group: 'operations',
    icon: 'GR',
    summary: 'Obligaciones periódicas, frecuencia y próximos vencimientos.',
  },
  roles: {
    group: 'control',
    icon: 'RB',
    title: 'Roles',
    navLabel: 'Roles',
    summary: 'Roles, permisos efectivos y asignación operativa como adaptador fino de la frontera Nexus/RBAC.',
  },
  webhooks: {
    group: 'integrations',
    icon: 'WH',
    summary: 'Endpoints, entregas y replay de eventos outbound.',
  },
  carWorkOrders: {
    group: 'operations',
    icon: 'OT',
    summary:
      'Taller auto-repair: tablero por estado y lista administrativa en /modules/carWorkOrders (selector Tablero / Lista).',
  },
  bikeWorkOrders: {
    group: 'operations',
    icon: 'BO',
    summary: 'Órdenes de trabajo de bicicletería: recepción, diagnóstico y entrega.',
  },
  occupationalHealthExams: {
    group: 'medical',
    icon: 'ML',
    summary: 'Exámenes preocupacionales, periódicos, reintegros y egresos para medicina laboral.',
  },
  ledgerAccounts: {
    group: 'accounting',
    icon: 'PC',
    summary: 'Plan de cuentas contable (activo/pasivo/patrimonio/ingreso/egreso). Base de las posting rules.',
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
      ...(meta?.customRoute ? { customRoute: meta.customRoute } : {}),
    };
    return [resourceId, definition];
  }),
);

export function hasCrudModule(resourceId: string): boolean {
  return resourceId in crudModuleCatalog;
}
