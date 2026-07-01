import { mergeRecordsPreferOverride } from '@devpablocristo/platform-browser';
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
  { id: 'accounting', label: 'Contabilidad' },
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
        id: 'report-sales-service',
        title: 'Ventas por servicio',
        description: 'Ranking y performance por servicio.',
        path: '/v1/reports/sales-by-service',
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
      'Los CRUDs usan CSV como formato canónico; esta consola mantiene CSV y XLSX para operación avanzada.',
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
  ledger: {
    id: 'ledger',
    title: 'Libros contables',
    navLabel: 'Libros contables',
    summary: 'Libro Diario, Sumas y Saldos, asientos manuales y configuración del plan de cuentas.',
    group: 'accounting',
    icon: 'LB',
    datasets: [
      {
        id: 'ledger-journal',
        title: 'Libro Diario',
        description: 'Asientos del período (excluye reversados como líneas nuevas).',
        path: '/v1/ledger/journal',
        fields: [fromField, toField],
        autoLoad: true,
      },
      {
        id: 'ledger-trial-balance',
        title: 'Sumas y Saldos',
        description: 'Saldo por cuenta a una fecha de corte.',
        path: '/v1/ledger/trial-balance',
        fields: [{ ...toField, name: 'as_of', label: 'Al' }],
        autoLoad: true,
      },
      {
        id: 'ledger-account-links',
        title: 'Cuentas por rol',
        description: 'Mapeo rol funcional → cuenta contable (posting rules).',
        path: '/v1/ledger/account-links',
        autoLoad: true,
      },
      {
        id: 'ledger-health',
        title: 'Salud del posteo (outbox)',
        description: 'Estado de la cola contable: pending / failed / posted / skipped / dead.',
        path: '/v1/ledger/health',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'ledger-setup',
        title: 'Inicializar plan de cuentas',
        description: 'Siembra (idempotente) la plantilla de cuentas AR y los account-links por defecto.',
        path: '/v1/ledger/setup',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        submitLabel: 'Inicializar',
      },
      {
        id: 'ledger-account-ledger',
        title: 'Libro Mayor por cuenta',
        description: 'Movimientos y saldo acumulado de una cuenta en un rango.',
        path: '/v1/ledger/accounts/:accountId/ledger',
        method: 'GET',
        fields: [
          { name: 'accountId', label: 'ID de cuenta', location: 'path', required: true },
          { ...fromField, required: false },
          { ...toField, required: false },
        ],
      },
      {
        id: 'ledger-link-role',
        title: 'Vincular cuenta a un rol',
        description: 'Asigna la cuenta que usan las posting rules para un rol (revenue, cash, receivable, vat_payable_21…).',
        path: '/v1/ledger/account-links/:role',
        method: 'PUT',
        response: 'json',
        submitLabel: 'Vincular',
        fields: [
          {
            name: 'role',
            label: 'Rol',
            location: 'path',
            required: true,
            type: 'select',
            options: [
              { value: 'revenue', label: 'revenue (Ventas)' },
              { value: 'cash', label: 'cash (Caja)' },
              { value: 'bank', label: 'bank (Banco)' },
              { value: 'receivable', label: 'receivable (Deudores)' },
              { value: 'payable', label: 'payable (Proveedores)' },
              { value: 'vat_payable_21', label: 'vat_payable_21 (IVA débito 21%)' },
              { value: 'vat_payable_105', label: 'vat_payable_105 (IVA débito 10.5%)' },
              { value: 'vat_credit_21', label: 'vat_credit_21 (IVA crédito 21%)' },
              { value: 'vat_credit_105', label: 'vat_credit_105 (IVA crédito 10.5%)' },
              { value: 'card_clearing', label: 'card_clearing (Tarjetas a liquidar)' },
              { value: 'mp_clearing', label: 'mp_clearing (MercadoPago a liquidar)' },
              { value: 'inventory', label: 'inventory (Mercaderías)' },
              { value: 'credit_note_payable', label: 'credit_note_payable (NC a clientes)' },
              { value: 'retained_earnings', label: 'retained_earnings (Resultado)' },
              { value: 'cogs', label: 'cogs (Costo de venta)' },
              { value: 'purchase_expense', label: 'purchase_expense (Gastos/compras servicio)' },
            ],
          },
          { name: 'account_id', label: 'ID de cuenta', location: 'body', required: true },
        ],
      },
      {
        id: 'ledger-manual-entry',
        title: 'Asiento manual',
        description: 'Registra un asiento balanceado (Σ debe = Σ haber, ≥2 líneas). Cada línea: debe XOR haber.',
        path: '/v1/ledger/entries',
        method: 'POST',
        response: 'json',
        submitLabel: 'Registrar asiento',
        fields: [
          { name: 'entry_date', label: 'Fecha', location: 'body', type: 'date' },
          { name: 'description', label: 'Descripción', location: 'body', type: 'text' },
          { name: 'currency', label: 'Moneda', location: 'body', placeholder: 'ARS (default)' },
          {
            name: 'lines',
            label: 'Líneas (JSON)',
            location: 'body',
            type: 'json',
            required: true,
            placeholder:
              '[{"account_id":"<uuid>","debit":100,"credit":0},{"account_id":"<uuid>","debit":0,"credit":100}]',
          },
        ],
      },
    ],
    notes: [
      'El plan de cuentas se administra en el módulo «Plan de cuentas». Acá van los libros y el asiento manual.',
      'Los asientos automáticos (ventas, compras, cobros, devoluciones) se postean solos vía el outbox contable.',
    ],
  },
  fiscal: {
    id: 'fiscal',
    title: 'Fiscal (ARCA)',
    navLabel: 'Fiscal (ARCA)',
    summary: 'Configuración del certificado ARCA, emisión de comprobantes electrónicos (CAE+QR) y notas de crédito.',
    group: 'accounting',
    icon: 'AR',
    datasets: [
      {
        id: 'fiscal-settings',
        title: 'Configuración fiscal',
        description: 'CUIT, condición IVA, punto de venta, ambiente y si hay certificado cargado.',
        path: '/v1/fiscal/settings',
        autoLoad: true,
      },
      {
        id: 'fiscal-vouchers',
        title: 'Comprobantes emitidos',
        description: 'Últimos comprobantes con tipo, número, total, estado y CAE.',
        path: '/v1/fiscal/vouchers',
        autoLoad: true,
      },
    ],
    actions: [
      {
        id: 'fiscal-save-settings',
        title: 'Guardar configuración',
        description: 'Guarda CUIT/condición/punto de venta/ambiente. Para cargar el certificado, pegá el PEM del cert y de la clave (la clave se guarda cifrada).',
        path: '/v1/fiscal/settings',
        method: 'PUT',
        response: 'json',
        submitLabel: 'Guardar',
        fields: [
          { name: 'cuit', label: 'CUIT (sin guiones)', location: 'body', required: true },
          {
            name: 'environment',
            label: 'Ambiente',
            location: 'body',
            type: 'select',
            defaultValue: 'homologation',
            options: [
              { value: 'homologation', label: 'Homologación' },
              { value: 'production', label: 'Producción' },
            ],
          },
          {
            name: 'tax_condition',
            label: 'Condición IVA del emisor',
            location: 'body',
            type: 'select',
            defaultValue: 'responsable_inscripto',
            options: [
              { value: 'responsable_inscripto', label: 'Responsable Inscripto' },
              { value: 'monotributo', label: 'Monotributo' },
              { value: 'exento', label: 'Exento' },
            ],
          },
          { name: 'default_point_of_sale', label: 'Punto de venta', location: 'body', type: 'number', placeholder: '1' },
          { name: 'enabled', label: 'Habilitar emisión (true/false)', location: 'body', type: 'json', placeholder: 'true' },
          { name: 'cert_pem', label: 'Certificado (PEM)', location: 'body', type: 'textarea', placeholder: '-----BEGIN CERTIFICATE----- …' },
          { name: 'key_pem', label: 'Clave privada (PEM)', location: 'body', type: 'textarea', placeholder: '-----BEGIN PRIVATE KEY----- …' },
        ],
      },
      {
        id: 'fiscal-test-auth',
        title: 'Probar conexión (WSAA)',
        description: 'Autentica contra ARCA con el certificado configurado para validar la conexión.',
        path: '/v1/fiscal/test-auth',
        method: 'POST',
        response: 'json',
        sendEmptyBody: true,
        submitLabel: 'Probar',
      },
      {
        id: 'fiscal-emit-voucher',
        title: 'Emitir comprobante de una venta',
        description: 'Solicita el CAE del comprobante (tipo A/B/C automático) de una venta ya registrada.',
        path: '/v1/fiscal/vouchers',
        method: 'POST',
        response: 'json',
        submitLabel: 'Emitir',
        fields: [
          { name: 'sale_id', label: 'ID de venta', location: 'body', required: true },
          { name: 'point_of_sale', label: 'Punto de venta (opcional)', location: 'body', type: 'number' },
        ],
      },
      {
        id: 'fiscal-emit-credit-note',
        title: 'Emitir nota de crédito de una devolución',
        description: 'Emite la NC referenciando la factura original autorizada de la venta de la devolución.',
        path: '/v1/fiscal/credit-notes',
        method: 'POST',
        response: 'json',
        submitLabel: 'Emitir NC',
        fields: [{ name: 'return_id', label: 'ID de devolución', location: 'body', required: true }],
      },
      {
        id: 'fiscal-download-pdf',
        title: 'Descargar PDF de un comprobante',
        description: 'Descarga el PDF fiscal (con CAE y QR) de un comprobante autorizado.',
        path: '/v1/fiscal/vouchers/:voucherId/pdf',
        method: 'GET',
        response: 'download',
        fields: [{ name: 'voucherId', label: 'ID de comprobante', location: 'path', required: true }],
      },
    ],
    notes: [
      'La emisión real requiere certificado ARCA + punto de venta habilitado como «web services». En homologación se prueba con el certificado de homologación.',
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
