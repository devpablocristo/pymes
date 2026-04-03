import { mergeRecordsPreferOverride } from '@devpablocristo/core-browser';
import { crudModuleCatalog } from '../crud/crudModuleCatalog';
import type { ModuleDefinition, ModuleField, ModuleRuntimeContext, ValueResolver } from './moduleCatalog.types';

export type {
  ModuleAction,
  ModuleDataset,
  ModuleDefinition,
  ModuleField,
  ModuleRuntimeContext,
} from './moduleCatalog.types';

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

const staticModuleCatalog: Record<string, ModuleDefinition> = {
  reports: {
    id: 'reports',
    title: 'Reportes',
    navLabel: 'Reportes',
    summary: 'Panel analítico con ventas, inventario, margen y cashflow.',
    group: 'operations',
    icon: 'RP',
    datasets: [
      {
        id: 'report-sales-summary',
        title: 'Resumen de ventas',
        description: 'KPI agregados de ventas.',
        path: '/v1/reports/sales-summary',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'report-sales-product',
        title: 'Ventas por producto',
        description: 'Ranking y performance por producto.',
        path: '/v1/reports/sales-by-product',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'report-sales-customer',
        title: 'Ventas por cliente',
        description: 'Concentración y recurrencia de clientes.',
        path: '/v1/reports/sales-by-customer',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'report-sales-payment',
        title: 'Ventas por medio de pago',
        description: 'Mix de cobros y canales.',
        path: '/v1/reports/sales-by-payment',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'report-inventory-valuation',
        title: 'Valuacion de inventario',
        description: 'Valuación total y por producto.',
        path: '/v1/reports/inventory-valuation',
        autoLoad: true,
      },
      {
        id: 'report-low-stock',
        title: 'Alerta de stock',
        description: 'SKU por debajo del mínimo.',
        path: '/v1/reports/low-stock',
        autoLoad: true,
      },
      {
        id: 'report-cashflow-summary',
        title: 'Resumen de cashflow',
        description: 'Entradas, salidas y balance del periodo.',
        path: '/v1/reports/cashflow-summary',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'report-profit-margin',
        title: 'Margen de rentabilidad',
        description: 'Margen bruto consolidado.',
        path: '/v1/reports/profit-margin',
        fields: [fromField, toField],
        autoLoad: true,
      },
    ],
  },
  whatsapp: {
    id: 'whatsapp',
    title: 'WhatsApp',
    navLabel: 'WhatsApp',
    summary: '',
    group: 'integrations',
    icon: 'WA',
  },
  paymentGateway: {
    id: 'paymentGateway',
    title: 'Pasarela de pago',
    navLabel: 'Pasarela de pago',
    summary: 'Estado de conexión, QR estático y payment links.',
    group: 'integrations',
    icon: 'MP',
    datasets: [
      {
        id: 'gateway-status',
        title: 'Estado de conexion',
        description: 'Estado actual de la pasarela.',
        path: '/v1/payment-gateway/status',
        autoLoad: true,
      },
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
        description: 'Obtiene template de importación por entidad en CSV o XLSX.',
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
          {
            name: 'format',
            label: 'Formato',
            location: 'query',
            required: true,
            type: 'select',
            defaultValue: 'csv',
            options: [
              { value: 'csv', label: 'csv' },
              { value: 'xlsx', label: 'xlsx' },
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
            defaultValue: 'csv',
            options: [
              { value: 'csv', label: 'csv' },
              { value: 'xlsx', label: 'xlsx' },
            ],
          },
          { ...fromField, required: false },
          { ...toField, required: false },
        ],
      },
    ],
    notes: [
      'Los CRUDs usan CSV como formato canónico; esta consola mantiene CSV y XLSX para operación avanzada y compatibilidad.',
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

export const moduleCatalog: Record<string, ModuleDefinition> = mergeRecordsPreferOverride(
  staticModuleCatalog,
  crudModuleCatalog,
);

export const moduleList = Object.values(moduleCatalog);

export function resolveModuleDefault(value: ValueResolver | undefined, ctx: ModuleRuntimeContext): string {
  if (typeof value === 'function') {
    return value(ctx);
  }
  return value ?? '';
}
