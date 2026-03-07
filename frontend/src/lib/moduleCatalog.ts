export type ModuleRuntimeContext = {
  orgId: string;
  today: string;
  monthStart: string;
};

type ValueResolver = string | ((ctx: ModuleRuntimeContext) => string);

export type ModuleField = {
  name: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  location?: 'path' | 'query' | 'body';
  type?: 'text' | 'number' | 'textarea' | 'date' | 'select';
  defaultValue?: ValueResolver;
  options?: Array<{ label: string; value: string }>;
};

export type ModuleDataset = {
  id: string;
  title: string;
  description: string;
  path: string;
  fields?: ModuleField[];
  autoLoad?: boolean;
};

export type ModuleAction = {
  id: string;
  title: string;
  description: string;
  path: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  fields?: ModuleField[];
  response?: 'json' | 'download' | 'none';
  submitLabel?: string;
  sendEmptyBody?: boolean;
};

export type ModuleDefinition = {
  id: string;
  title: string;
  navLabel: string;
  summary: string;
  group: string;
  icon: string;
  badge?: string;
  datasets?: ModuleDataset[];
  actions?: ModuleAction[];
  notes?: string[];
};

export const moduleGroups = [
  { id: 'commercial', label: 'Comercial' },
  { id: 'operations', label: 'Operaciones' },
  { id: 'control', label: 'Control' },
  { id: 'integrations', label: 'Integraciones' },
] as const;

const fromField: ModuleField = {
  name: 'from',
  label: 'Desde',
  location: 'query',
  type: 'date',
  defaultValue: (ctx) => ctx.monthStart,
};

const toField: ModuleField = {
  name: 'to',
  label: 'Hasta',
  location: 'query',
  type: 'date',
  defaultValue: (ctx) => ctx.today,
};

export const moduleCatalog: Record<string, ModuleDefinition> = {
  parties: {
    id: 'parties',
    title: 'Modelo de entidades',
    navLabel: 'Entidades',
    summary: 'Vista transversal de personas y organizaciones con roles y relaciones.',
    group: 'commercial',
    icon: 'PT',
    datasets: [
      {
        id: 'parties-list',
        title: 'Entidades',
        description: 'Listado consolidado del modelo de entidades.',
        path: '/v1/parties',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'party-relationships',
        title: 'Relaciones de entidad',
        description: 'Consulta relaciones por ID de entidad.',
        path: '/v1/parties/:partyId/relationships',
        method: 'GET',
        fields: [{ name: 'partyId', label: 'ID de entidad', location: 'path', required: true }],
      },
    ],
  },
  customers: {
    id: 'customers',
    title: 'Clientes',
    navLabel: 'Clientes',
    summary: 'Base de clientes con historial comercial y exportación.',
    group: 'commercial',
    icon: 'CL',
    datasets: [
      { id: 'customers-list', title: 'Clientes', description: 'Listado principal de clientes.', path: '/v1/customers', autoLoad: true },
    ],
    actions: [
      {
        id: 'customer-sales',
        title: 'Historial de ventas',
        description: 'Consulta ventas asociadas a un cliente.',
        path: '/v1/customers/:customerId/sales',
        method: 'GET',
        fields: [{ name: 'customerId', label: 'Cliente ID', location: 'path', required: true }],
      },
    ],
  },
  suppliers: {
    id: 'suppliers',
    title: 'Proveedores',
    navLabel: 'Proveedores',
    summary: 'Catálogo de proveedores y datos de abastecimiento.',
    group: 'commercial',
    icon: 'PR',
    datasets: [
      { id: 'suppliers-list', title: 'Proveedores', description: 'Listado principal de proveedores.', path: '/v1/suppliers', autoLoad: true },
    ],
  },
  products: {
    id: 'products',
    title: 'Productos',
    navLabel: 'Productos',
    summary: 'Catálogo de productos, precios, costos y stock.',
    group: 'commercial',
    icon: 'PD',
    datasets: [
      { id: 'products-list', title: 'Productos', description: 'Catálogo actual de productos.', path: '/v1/products', autoLoad: true },
    ],
  },
  inventory: {
    id: 'inventory',
    title: 'Inventario',
    navLabel: 'Inventario',
    summary: 'Stock actual, movimientos y alertas de bajo stock.',
    group: 'operations',
    icon: 'IN',
    datasets: [
      { id: 'inventory-list', title: 'Stock', description: 'Stock consolidado por producto.', path: '/v1/inventory', autoLoad: true },
      { id: 'inventory-low-stock', title: 'Bajo stock', description: 'Productos por debajo del mínimo.', path: '/v1/inventory/low-stock', autoLoad: true },
      { id: 'inventory-movements', title: 'Movimientos', description: 'Kardex y ajustes recientes.', path: '/v1/inventory/movements', autoLoad: true },
    ],
    actions: [
      {
        id: 'inventory-product',
        title: 'Detalle por producto',
        description: 'Consulta stock de un producto puntual.',
        path: '/v1/inventory/:productId',
        method: 'GET',
        fields: [{ name: 'productId', label: 'Producto ID', location: 'path', required: true }],
      },
    ],
  },
  priceLists: {
    id: 'priceLists',
    title: 'Listas de precios',
    navLabel: 'Listas de precios',
    summary: 'Manejo de listas activas, default y markups.',
    group: 'commercial',
    icon: 'LP',
    datasets: [
      { id: 'price-lists', title: 'Listas activas', description: 'Listas de precios configuradas.', path: '/v1/price-lists', autoLoad: true },
    ],
  },
  quotes: {
    id: 'quotes',
    title: 'Presupuestos',
    navLabel: 'Presupuestos',
    summary: 'Embudo comercial, conversiones y documentos PDF.',
    group: 'commercial',
    icon: 'QT',
    datasets: [
      { id: 'quotes-list', title: 'Presupuestos', description: 'Listado de presupuestos vigentes.', path: '/v1/quotes', autoLoad: true },
    ],
    actions: [
      {
        id: 'quote-detail',
        title: 'Detalle de presupuesto',
        description: 'Consulta un presupuesto puntual.',
        path: '/v1/quotes/:quoteId',
        method: 'GET',
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
      {
        id: 'quote-send',
        title: 'Marcar como enviado',
        description: 'Dispara el flujo de envío del presupuesto.',
        path: '/v1/quotes/:quoteId/send',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
      {
        id: 'quote-accept',
        title: 'Aceptar presupuesto',
        description: 'Cambia el estado del presupuesto a aceptado.',
        path: '/v1/quotes/:quoteId/accept',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
      {
        id: 'quote-pdf',
        title: 'Descargar PDF',
        description: 'Genera el PDF del presupuesto.',
        path: '/v1/quotes/:quoteId/pdf',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
    ],
  },
  sales: {
    id: 'sales',
    title: 'Ventas',
    navLabel: 'Ventas',
    summary: 'Ventas, comprobantes, cobros y documentos asociados.',
    group: 'commercial',
    icon: 'VT',
    datasets: [
      { id: 'sales-list', title: 'Ventas', description: 'Ventas registradas en la organización.', path: '/v1/sales', autoLoad: true },
    ],
    actions: [
      {
        id: 'sale-detail',
        title: 'Detalle de venta',
        description: 'Consulta una venta puntual.',
        path: '/v1/sales/:saleId',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'sale-void',
        title: 'Anular venta',
        description: 'Anula la venta indicada.',
        path: '/v1/sales/:saleId/void',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'sale-receipt',
        title: 'Descargar comprobante',
        description: 'Descarga el comprobante PDF de la venta.',
        path: '/v1/sales/:saleId/receipt',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
    ],
  },
  cashflow: {
    id: 'cashflow',
    title: 'Caja',
    navLabel: 'Caja',
    summary: 'Movimientos, cierres y resumen diario de caja.',
    group: 'operations',
    icon: 'CJ',
    datasets: [
      { id: 'cashflow-list', title: 'Movimientos', description: 'Entradas y salidas de caja.', path: '/v1/cashflow', autoLoad: true },
      { id: 'cashflow-summary', title: 'Resumen', description: 'Resumen consolidado de caja.', path: '/v1/cashflow/summary', autoLoad: true },
      { id: 'cashflow-daily', title: 'Resumen diario', description: 'Evolución diaria del cashflow.', path: '/v1/cashflow/summary/daily', autoLoad: true },
    ],
  },
  purchases: {
    id: 'purchases',
    title: 'Compras',
    navLabel: 'Compras',
    summary: 'Circuito de compras, recepciones y costos.',
    group: 'operations',
    icon: 'CP',
    datasets: [
      { id: 'purchases-list', title: 'Compras', description: 'Listado de compras.', path: '/v1/purchases', autoLoad: true },
    ],
    actions: [
      {
        id: 'purchase-detail',
        title: 'Detalle de compra',
        description: 'Consulta una compra por ID.',
        path: '/v1/purchases/:purchaseId',
        method: 'GET',
        fields: [{ name: 'purchaseId', label: 'ID de compra', location: 'path', required: true }],
      },
    ],
  },
  accounts: {
    id: 'accounts',
    title: 'Cuentas corrientes',
    navLabel: 'Cuentas corrientes',
    summary: 'Saldo por entidad, deuda y movimientos de cuentas.',
    group: 'commercial',
    icon: 'CC',
    datasets: [
      { id: 'accounts-list', title: 'Cuentas', description: 'Estado de cuentas corrientes.', path: '/v1/accounts', autoLoad: true },
      { id: 'accounts-debtors', title: 'Deudores', description: 'Top de cuentas con saldo pendiente.', path: '/v1/accounts/debtors', autoLoad: true },
    ],
    actions: [
      {
        id: 'account-movements',
        title: 'Movimientos de cuenta',
        description: 'Consulta movimientos por cuenta.',
        path: '/v1/accounts/:accountId/movements',
        method: 'GET',
        fields: [{ name: 'accountId', label: 'Cuenta ID', location: 'path', required: true }],
      },
    ],
  },
  payments: {
    id: 'payments',
    title: 'Pagos',
    navLabel: 'Pagos',
    summary: 'Pagos aplicados a ventas y cobranzas manuales.',
    group: 'commercial',
    icon: 'PG',
    actions: [
      {
        id: 'sale-payments',
        title: 'Pagos por venta',
        description: 'Lista pagos asociados a una venta.',
        path: '/v1/sales/:saleId/payments',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'sale-payment-create',
        title: 'Registrar pago manual',
        description: 'Carga un pago sobre una venta existente.',
        path: '/v1/sales/:saleId/payments',
        method: 'POST',
        fields: [
          { name: 'saleId', label: 'Sale ID', location: 'path', required: true },
          { name: 'method', label: 'Método', location: 'body', required: true, placeholder: 'efectivo, transferencia, tarjeta' },
          { name: 'amount', label: 'Importe', location: 'body', type: 'number', required: true },
          { name: 'received_at', label: 'Recibido en', location: 'body', placeholder: '2026-03-06T10:00:00' },
          { name: 'notes', label: 'Notas', location: 'body', type: 'textarea' },
        ],
      },
    ],
  },
  returns: {
    id: 'returns',
    title: 'Devoluciones',
    navLabel: 'Devoluciones',
    summary: 'Devoluciones, notas de credito y aplicaciones de saldo.',
    group: 'commercial',
    icon: 'DV',
    datasets: [
      { id: 'returns-list', title: 'Devoluciones', description: 'Devoluciones registradas.', path: '/v1/returns', autoLoad: true },
      { id: 'credit-notes', title: 'Notas de credito', description: 'Notas de credito emitidas.', path: '/v1/credit-notes', autoLoad: true },
    ],
    actions: [
      {
        id: 'return-detail',
        title: 'Detalle de devolucion',
        description: 'Consulta una devolucion puntual.',
        path: '/v1/returns/:returnId',
        method: 'GET',
        fields: [{ name: 'returnId', label: 'ID de devolución', location: 'path', required: true }],
      },
      {
        id: 'credit-note-detail',
        title: 'Detalle de nota de credito',
        description: 'Consulta una nota de credito puntual.',
        path: '/v1/credit-notes/:creditNoteId',
        method: 'GET',
        fields: [{ name: 'creditNoteId', label: 'ID de nota de crédito', location: 'path', required: true }],
      },
      {
        id: 'party-credit-notes',
        title: 'Notas por party',
        description: 'Lista notas de credito por party.',
        path: '/v1/parties/:partyId/credit-notes',
        method: 'GET',
        fields: [{ name: 'partyId', label: 'ID de entidad', location: 'path', required: true }],
      },
    ],
  },
  recurring: {
    id: 'recurring',
    title: 'Gastos recurrentes',
    navLabel: 'Gastos recurrentes',
    summary: 'Obligaciones periódicas, frecuencia y próximos vencimientos.',
    group: 'operations',
    icon: 'GR',
    datasets: [
      { id: 'recurring-list', title: 'Gastos recurrentes', description: 'Listado de gastos programados.', path: '/v1/recurring-expenses', autoLoad: true },
    ],
  },
  appointments: {
    id: 'appointments',
    title: 'Turnos',
    navLabel: 'Turnos',
    summary: 'Agenda operativa con filtros por fecha, estado y asignación.',
    group: 'operations',
    icon: 'TR',
    datasets: [
      {
        id: 'appointments-list',
        title: 'Agenda',
        description: 'Turnos del periodo actual.',
        path: '/v1/appointments',
        autoLoad: true,
        fields: [fromField, toField],
      },
    ],
    actions: [
      {
        id: 'appointment-detail',
        title: 'Detalle de turno',
        description: 'Consulta un turno puntual.',
        path: '/v1/appointments/:appointmentId',
        method: 'GET',
        fields: [{ name: 'appointmentId', label: 'Appointment ID', location: 'path', required: true }],
      },
    ],
  },
  reports: {
    id: 'reports',
    title: 'Reportes',
    navLabel: 'Reportes',
    summary: 'Panel analítico con ventas, inventario, margen y cashflow.',
    group: 'operations',
    icon: 'RP',
    datasets: [
      { id: 'report-sales-summary', title: 'Resumen de ventas', description: 'KPI agregados de ventas.', path: '/v1/reports/sales-summary', fields: [fromField, toField], autoLoad: true },
      { id: 'report-sales-product', title: 'Ventas por producto', description: 'Ranking y performance por producto.', path: '/v1/reports/sales-by-product', fields: [fromField, toField], autoLoad: true },
      { id: 'report-sales-customer', title: 'Ventas por cliente', description: 'Concentración y recurrencia de clientes.', path: '/v1/reports/sales-by-customer', fields: [fromField, toField], autoLoad: true },
      { id: 'report-sales-payment', title: 'Ventas por medio de pago', description: 'Mix de cobros y canales.', path: '/v1/reports/sales-by-payment', fields: [fromField, toField], autoLoad: true },
      { id: 'report-inventory-valuation', title: 'Valuacion de inventario', description: 'Valuación total y por producto.', path: '/v1/reports/inventory-valuation', autoLoad: true },
      { id: 'report-low-stock', title: 'Alerta de stock', description: 'SKU por debajo del mínimo.', path: '/v1/reports/low-stock', autoLoad: true },
      { id: 'report-cashflow-summary', title: 'Resumen de cashflow', description: 'Entradas, salidas y balance del periodo.', path: '/v1/reports/cashflow-summary', fields: [fromField, toField], autoLoad: true },
      { id: 'report-profit-margin', title: 'Margen de rentabilidad', description: 'Margen bruto consolidado.', path: '/v1/reports/profit-margin', fields: [fromField, toField], autoLoad: true },
    ],
  },
  audit: {
    id: 'audit',
    title: 'Auditoria',
    navLabel: 'Auditoria',
    summary: 'Trazabilidad de actividad, eventos y exportaciones.',
    group: 'control',
    icon: 'AU',
    datasets: [
      { id: 'audit-list', title: 'Actividad', description: 'Eventos recientes de auditoría.', path: '/v1/audit', autoLoad: true },
    ],
    actions: [
      {
        id: 'audit-export',
        title: 'Exportar auditoria',
        description: 'Descarga la auditoría en el formato del backend.',
        path: '/v1/audit/export',
        method: 'GET',
        response: 'download',
      },
    ],
  },
  roles: {
    id: 'roles',
    title: 'RBAC',
    navLabel: 'RBAC',
    summary: 'Roles, permisos efectivos y asignación operativa.',
    group: 'control',
    icon: 'RB',
    datasets: [
      { id: 'roles-list', title: 'Roles', description: 'Roles existentes en la organización.', path: '/v1/roles', autoLoad: true },
    ],
    actions: [
      {
        id: 'role-detail',
        title: 'Detalle de rol',
        description: 'Consulta permisos y metadata de un rol.',
        path: '/v1/roles/:roleId',
        method: 'GET',
        fields: [{ name: 'roleId', label: 'ID de rol', location: 'path', required: true }],
      },
      {
        id: 'user-permissions',
        title: 'Permisos efectivos de usuario',
        description: 'Consulta permisos resueltos para un usuario.',
        path: '/v1/users/:userId/permissions',
        method: 'GET',
        fields: [{ name: 'userId', label: 'ID de usuario', location: 'path', required: true }],
      },
    ],
  },
  timeline: {
    id: 'timeline',
    title: 'Historial',
    navLabel: 'Historial',
    summary: 'Timeline por entidad y notas operativas manuales.',
    group: 'control',
    icon: 'TL',
    actions: [
      {
        id: 'timeline-list',
        title: 'Timeline por entidad',
        description: 'Consulta historial de una entidad específica.',
        path: '/v1/:entity/:entityId/timeline',
        method: 'GET',
        fields: [
          { name: 'entity', label: 'Entidad', location: 'path', required: true, placeholder: 'ventas, presupuestos, productos' },
          { name: 'entityId', label: 'ID de entidad', location: 'path', required: true },
        ],
      },
      {
        id: 'timeline-note',
        title: 'Agregar nota manual',
        description: 'Registra una nota sobre cualquier entidad.',
        path: '/v1/:entity/:entityId/notes',
        method: 'POST',
        fields: [
          { name: 'entity', label: 'Entidad', location: 'path', required: true, placeholder: 'ventas, presupuestos, productos' },
          { name: 'entityId', label: 'ID de entidad', location: 'path', required: true },
          { name: 'title', label: 'Título', location: 'body', placeholder: 'Nota manual' },
          { name: 'note', label: 'Nota', location: 'body', type: 'textarea', required: true },
        ],
      },
    ],
  },
  documents: {
    id: 'documents',
    title: 'PDFs y comprobantes',
    navLabel: 'PDFs',
    summary: 'Descarga directa de PDFs comerciales generados por backend.',
    group: 'control',
    icon: 'PDF',
    actions: [
      {
        id: 'documents-quote-pdf',
        title: 'PDF de presupuesto',
        description: 'Genera el documento PDF de un presupuesto.',
        path: '/v1/quotes/:quoteId/pdf',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
      {
        id: 'documents-sale-receipt',
        title: 'Comprobante de venta',
        description: 'Descarga el comprobante PDF de una venta.',
        path: '/v1/sales/:saleId/receipt',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
    ],
  },
  attachments: {
    id: 'attachments',
    title: 'Adjuntos',
    navLabel: 'Adjuntos',
    summary: 'Adjuntos por entidad, enlaces firmados y descargas.',
    group: 'integrations',
    icon: 'AD',
    actions: [
      {
        id: 'attachments-list',
        title: 'Adjuntos por entidad',
        description: 'Lista adjuntos asociados a una entidad.',
        path: '/v1/:entity/:entityId/attachments',
        method: 'GET',
        fields: [
          { name: 'entity', label: 'Entidad', location: 'path', required: true, placeholder: 'ventas, presupuestos, compras' },
          { name: 'entityId', label: 'ID de entidad', location: 'path', required: true },
        ],
      },
      {
        id: 'attachment-url',
        title: 'Obtener URL firmada',
        description: 'Devuelve URL temporal de descarga.',
        path: '/v1/attachments/:attachmentId/url',
        method: 'GET',
        fields: [{ name: 'attachmentId', label: 'ID de adjunto', location: 'path', required: true }],
      },
      {
        id: 'attachment-download',
        title: 'Descargar adjunto',
        description: 'Descarga directa del archivo almacenado.',
        path: '/v1/attachments/:attachmentId/download',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'attachmentId', label: 'ID de adjunto', location: 'path', required: true }],
      },
    ],
  },
  webhooks: {
    id: 'webhooks',
    title: 'Webhooks salientes',
    navLabel: 'Webhooks',
    summary: 'Endpoints, entregas y replay de eventos outbound.',
    group: 'integrations',
    icon: 'WH',
    datasets: [
      { id: 'webhook-endpoints', title: 'Endpoints', description: 'Endpoints configurados para webhooks salientes.', path: '/v1/webhook-endpoints', autoLoad: true },
    ],
    actions: [
      {
        id: 'webhook-deliveries',
        title: 'Entregas por endpoint',
        description: 'Consulta entregas asociadas a un endpoint.',
        path: '/v1/webhook-endpoints/:endpointId/deliveries',
        method: 'GET',
        fields: [{ name: 'endpointId', label: 'ID de endpoint', location: 'path', required: true }],
      },
      {
        id: 'webhook-test',
        title: 'Enviar webhook de prueba',
        description: 'Dispara un evento de prueba hacia un endpoint.',
        path: '/v1/webhook-endpoints/:endpointId/test',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'endpointId', label: 'ID de endpoint', location: 'path', required: true }],
      },
      {
        id: 'webhook-replay',
        title: 'Reintentar entrega',
        description: 'Reprocesa una entrega fallida.',
        path: '/v1/webhook-deliveries/:deliveryId/replay',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        fields: [{ name: 'deliveryId', label: 'ID de entrega', location: 'path', required: true }],
      },
    ],
  },
  whatsapp: {
    id: 'whatsapp',
    title: 'WhatsApp',
    navLabel: 'WhatsApp',
    summary: 'Links de contacto, comprobantes y cobro asistido por WhatsApp.',
    group: 'integrations',
    icon: 'WA',
    actions: [
      {
        id: 'whatsapp-quote',
        title: 'Link de presupuesto',
        description: 'Genera mensaje de WhatsApp para un presupuesto.',
        path: '/v1/whatsapp/quote/:quoteId',
        method: 'GET',
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
      {
        id: 'whatsapp-sale-receipt',
        title: 'Link de comprobante',
        description: 'Genera mensaje de WhatsApp para un comprobante.',
        path: '/v1/whatsapp/sale/:saleId/receipt',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'whatsapp-customer-message',
        title: 'Mensaje libre a cliente',
        description: 'Genera un link de WhatsApp con mensaje personalizado.',
        path: '/v1/whatsapp/customer/:customerId/message',
        method: 'GET',
        fields: [
          { name: 'customerId', label: 'Customer ID', location: 'path', required: true },
          { name: 'message', label: 'Mensaje', location: 'query', required: true, type: 'textarea' },
        ],
      },
      {
        id: 'whatsapp-payment-info',
        title: 'Info de pago de venta',
        description: 'Genera el texto de pago para WhatsApp.',
        path: '/v1/whatsapp/sale/:saleId/payment-info',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'whatsapp-payment-link',
        title: 'Link de cobro por venta',
        description: 'Genera link de cobro de una venta para WhatsApp.',
        path: '/v1/whatsapp/sale/:saleId/payment-link',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
    ],
  },
  paymentGateway: {
    id: 'paymentGateway',
    title: 'Pasarela de pago',
    navLabel: 'Pasarela de pago',
    summary: 'Estado de conexión, QR estático y payment links.',
    group: 'integrations',
    icon: 'MP',
    datasets: [
      { id: 'gateway-status', title: 'Estado de conexion', description: 'Estado actual de la pasarela.', path: '/v1/payment-gateway/status', autoLoad: true },
    ],
    actions: [
      {
        id: 'gateway-static-qr',
        title: 'Descargar QR estatico',
        description: 'Descarga PNG del QR estático configurado.',
        path: '/v1/payment-methods/qr-static/download',
        method: 'GET',
        response: 'download',
      },
      {
        id: 'gateway-sale-link',
        title: 'Link de pago de venta',
        description: 'Consulta payment link generado para una venta.',
        path: '/v1/sales/:saleId/payment-link',
        method: 'GET',
        fields: [{ name: 'saleId', label: 'ID de venta', location: 'path', required: true }],
      },
      {
        id: 'gateway-quote-link',
        title: 'Link de pago de presupuesto',
        description: 'Consulta payment link de un presupuesto.',
        path: '/v1/quotes/:quoteId/payment-link',
        method: 'GET',
        fields: [{ name: 'quoteId', label: 'ID de presupuesto', location: 'path', required: true }],
      },
    ],
    notes: [
      'La conexión OAuth del gateway sigue siendo un flujo operativo del backend y no es navegable con headers autenticados estándar desde el browser.',
    ],
  },
  dataIO: {
    id: 'dataIO',
    title: 'Import / Export',
    navLabel: 'Import / Export',
    summary: 'Templates y exportaciones masivas para entidades del core.',
    group: 'control',
    icon: 'IO',
    actions: [
      {
        id: 'dataio-template',
        title: 'Descargar template',
        description: 'Obtiene template de importación por entidad.',
        path: '/v1/import/templates/:entity',
        method: 'GET',
        response: 'download',
        fields: [
          {
            name: 'entity',
            label: 'Entidad',
            location: 'path',
            required: true,
            type: 'select',
            options: [
              { value: 'customers', label: 'Clientes' },
              { value: 'products', label: 'Productos' },
              { value: 'suppliers', label: 'Proveedores' },
            ],
          },
        ],
      },
      {
        id: 'dataio-export',
        title: 'Exportar entidad',
        description: 'Exporta datos en CSV o XLSX.',
        path: '/v1/export/:entity',
        method: 'GET',
        response: 'download',
        fields: [
          {
            name: 'entity',
            label: 'Entidad',
            location: 'path',
            required: true,
            type: 'select',
            options: [
              { value: 'customers', label: 'Clientes' },
              { value: 'products', label: 'Productos' },
              { value: 'suppliers', label: 'Proveedores' },
              { value: 'sales', label: 'Ventas' },
              { value: 'cashflow', label: 'Flujo de caja' },
            ],
          },
          {
            name: 'format',
            label: 'Formato',
            location: 'query',
            required: true,
            type: 'select',
            defaultValue: 'xlsx',
            options: [
              { value: 'xlsx', label: 'xlsx' },
              { value: 'csv', label: 'csv' },
            ],
          },
          { ...fromField, required: false },
          { ...toField, required: false },
        ],
      },
    ],
    notes: [
      'El flujo de preview/confirm de importación sigue expuesto por API, pero en esta pasada del FE se priorizó la operación segura de templates y exportaciones.',
    ],
  },
  scheduler: {
    id: 'scheduler',
    title: 'Planificador',
    navLabel: 'Planificador',
    summary: 'Superficie operativa del scheduler del control plane.',
    group: 'control',
    icon: 'SC',
    notes: [
      'El scheduler del backend corre como endpoint técnico protegido por secret y no expone acciones autenticadas de usuario final.',
      'La presencia en FE sirve para documentar su rol dentro del monorepo y evitar el desfasaje entre README y navegación.',
    ],
  },
};

export const moduleList = Object.values(moduleCatalog);

export function resolveModuleDefault(value: ValueResolver | undefined, ctx: ModuleRuntimeContext): string {
  if (typeof value === 'function') {
    return value(ctx);
  }
  return value ?? '';
}
