/**
 * Facturación demo — 4 vistas (list, preview, add, edit) en una sola página
 * con navegación por estado interno. Inspirado en el template Wowdash.
 */
import { useState, useCallback, useMemo } from 'react';
import './InvoiceDemoPage.css';

// ─── Tipos ───

type InvoiceStatus = 'paid' | 'pending' | 'overdue';

type LineItem = {
  id: string;
  description: string;
  qty: number;
  unit: string;
  unitPrice: number;
};

type Invoice = {
  id: string;
  number: string;
  customer: string;
  initials: string;
  issuedDate: string;
  dueDate: string;
  status: InvoiceStatus;
  items: LineItem[];
  discount: number;
  tax: number;
};

type View = 'list' | 'preview' | 'add' | 'edit';

// ─── Demo data ───

let nextLineId = 200;
function lineUid() { return String(++nextLineId); }
let nextInvId = 20;
function invUid() { return String(++nextInvId); }

function initials(name: string): string {
  return name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2);
}

const DEMO_INVOICES: Invoice[] = [
  { id: '1', number: 'INV-3492', customer: 'María García', initials: 'MG', issuedDate: '2026-03-10', dueDate: '2026-04-10', status: 'paid', items: [
    { id: '1', description: 'Diseño de logo', qty: 1, unit: 'unidad', unitPrice: 15000 },
    { id: '2', description: 'Tarjetas de presentación', qty: 500, unit: 'unidades', unitPrice: 12 },
  ], discount: 0, tax: 21 },
  { id: '2', number: 'INV-3493', customer: 'Juan Pérez', initials: 'JP', issuedDate: '2026-03-12', dueDate: '2026-04-12', status: 'pending', items: [
    { id: '3', description: 'Desarrollo web', qty: 40, unit: 'horas', unitPrice: 5000 },
  ], discount: 5, tax: 21 },
  { id: '3', number: 'INV-3494', customer: 'Ana López', initials: 'AL', issuedDate: '2026-03-05', dueDate: '2026-03-20', status: 'overdue', items: [
    { id: '4', description: 'Consultoría SEO', qty: 10, unit: 'horas', unitPrice: 3500 },
    { id: '5', description: 'Auditoría técnica', qty: 1, unit: 'unidad', unitPrice: 25000 },
  ], discount: 10, tax: 21 },
  { id: '4', number: 'INV-3495', customer: 'Carlos Ruiz', initials: 'CR', issuedDate: '2026-03-15', dueDate: '2026-04-15', status: 'paid', items: [
    { id: '6', description: 'Hosting anual', qty: 1, unit: 'año', unitPrice: 48000 },
  ], discount: 0, tax: 21 },
  { id: '5', number: 'INV-3496', customer: 'Laura Díaz', initials: 'LD', issuedDate: '2026-03-18', dueDate: '2026-04-18', status: 'pending', items: [
    { id: '7', description: 'Mantenimiento mensual', qty: 3, unit: 'meses', unitPrice: 15000 },
    { id: '8', description: 'Soporte premium', qty: 3, unit: 'meses', unitPrice: 8000 },
  ], discount: 0, tax: 21 },
  { id: '6', number: 'INV-3497', customer: 'Pedro Sánchez', initials: 'PS', issuedDate: '2026-03-20', dueDate: '2026-04-20', status: 'paid', items: [
    { id: '9', description: 'App mobile MVP', qty: 1, unit: 'proyecto', unitPrice: 350000 },
  ], discount: 15, tax: 21 },
];

function calcSubtotal(items: LineItem[]): number {
  return items.reduce((sum, it) => sum + it.qty * it.unitPrice, 0);
}

function calcTotal(inv: Invoice): number {
  const sub = calcSubtotal(inv.items);
  const afterDiscount = sub * (1 - inv.discount / 100);
  return afterDiscount * (1 + inv.tax / 100);
}

function fmtMoney(n: number): string {
  return n.toLocaleString('es-AR', { style: 'currency', currency: 'ARS', minimumFractionDigits: 0 });
}

const STATUS_LABELS: Record<InvoiceStatus, string> = { paid: 'Pagada', pending: 'Pendiente', overdue: 'Vencida' };
const STATUS_CLASSES: Record<InvoiceStatus, string> = { paid: 'badge-success', pending: 'badge-warning', overdue: 'badge-danger' };

// ─── List View ───

function InvoiceList({
  invoices,
  onView,
  onEdit,
  onAdd,
  onDelete,
}: {
  invoices: Invoice[];
  onView: (id: string) => void;
  onEdit: (id: string) => void;
  onAdd: () => void;
  onDelete: (id: string) => void;
}) {
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<InvoiceStatus | 'all'>('all');

  const filtered = useMemo(() => {
    let result = invoices;
    if (statusFilter !== 'all') result = result.filter(i => i.status === statusFilter);
    const q = search.trim().toLowerCase();
    if (q) result = result.filter(i => i.number.toLowerCase().includes(q) || i.customer.toLowerCase().includes(q));
    return result;
  }, [invoices, search, statusFilter]);

  return (
    <div className="card">
      <div className="inv__toolbar">
        <div className="inv__toolbar-left">
          <input
            type="search"
            className="inv__search"
            placeholder="Buscar factura o cliente…"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
          <select value={statusFilter} onChange={e => setStatusFilter(e.target.value as InvoiceStatus | 'all')}>
            <option value="all">Todas</option>
            <option value="paid">Pagadas</option>
            <option value="pending">Pendientes</option>
            <option value="overdue">Vencidas</option>
          </select>
        </div>
        <button type="button" className="btn-primary btn-sm" onClick={onAdd}>+ Nueva factura</button>
      </div>

      <table className="inv__table">
        <thead>
          <tr>
            <th>N°</th>
            <th>Cliente</th>
            <th>Fecha</th>
            <th>Total</th>
            <th>Estado</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {filtered.map(inv => (
            <tr key={inv.id}>
              <td style={{ fontWeight: 600 }}>{inv.number}</td>
              <td>
                <div className="inv__customer">
                  <span className="inv__avatar">{inv.initials}</span>
                  {inv.customer}
                </div>
              </td>
              <td>{new Date(inv.issuedDate).toLocaleDateString('es-AR', { day: '2-digit', month: 'short', year: 'numeric' })}</td>
              <td style={{ fontWeight: 600 }}>{fmtMoney(calcTotal(inv))}</td>
              <td><span className={`badge ${STATUS_CLASSES[inv.status]}`}>{STATUS_LABELS[inv.status]}</span></td>
              <td>
                <div className="inv__actions">
                  <button type="button" className="inv__action inv__action--view" onClick={() => onView(inv.id)} title="Ver">👁</button>
                  <button type="button" className="inv__action inv__action--edit" onClick={() => onEdit(inv.id)} title="Editar">✏️</button>
                  <button type="button" className="inv__action inv__action--delete" onClick={() => onDelete(inv.id)} title="Eliminar">🗑️</button>
                </div>
              </td>
            </tr>
          ))}
          {filtered.length === 0 && (
            <tr><td colSpan={6} style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>Sin resultados</td></tr>
          )}
        </tbody>
      </table>

      <div className="inv__pagination">
        <span>Mostrando {filtered.length} de {invoices.length}</span>
        <div className="inv__page-btns">
          <button type="button" className="inv__page-btn">&larr;</button>
          <button type="button" className="inv__page-btn inv__page-btn--active">1</button>
          <button type="button" className="inv__page-btn">&rarr;</button>
        </div>
      </div>
    </div>
  );
}

// ─── Preview View ───

function InvoicePreview({ invoice, onBack, onEdit }: { invoice: Invoice; onBack: () => void; onEdit: () => void }) {
  const sub = calcSubtotal(invoice.items);
  const discountAmt = sub * (invoice.discount / 100);
  const afterDiscount = sub - discountAmt;
  const taxAmt = afterDiscount * (invoice.tax / 100);
  const total = afterDiscount + taxAmt;

  return (
    <div className="card">
      <div style={{ display: 'flex', gap: '0.4rem', marginBottom: 'var(--space-4)' }}>
        <button type="button" className="btn-secondary btn-sm" onClick={onBack}>&larr; Volver</button>
        <button type="button" className="btn-secondary btn-sm" onClick={onEdit}>Editar</button>
        <button type="button" className="btn-secondary btn-sm" onClick={() => window.print()}>Imprimir</button>
      </div>

      <div className="inv__preview">
        <div className="inv__preview-header">
          <div>
            <h2 className="inv__preview-number">{invoice.number}</h2>
            <span className={`badge ${STATUS_CLASSES[invoice.status]}`}>{STATUS_LABELS[invoice.status]}</span>
          </div>
          <div style={{ textAlign: 'right', fontSize: '0.85rem' }}>
            <strong>Mi Empresa S.R.L.</strong><br />
            Av. Corrientes 1234, CABA<br />
            info@miempresa.com
          </div>
        </div>

        <div className="inv__detail-grid">
          <div>
            <div className="inv__detail-label">Cliente</div>
            <div className="inv__detail-value">{invoice.customer}</div>
          </div>
          <div>
            <div className="inv__detail-label">Fecha emisión</div>
            <div className="inv__detail-value">{new Date(invoice.issuedDate).toLocaleDateString('es-AR')}</div>
          </div>
          <div>
            <div className="inv__detail-label">Vencimiento</div>
            <div className="inv__detail-value">{new Date(invoice.dueDate).toLocaleDateString('es-AR')}</div>
          </div>
          <div>
            <div className="inv__detail-label">N° Factura</div>
            <div className="inv__detail-value">{invoice.number}</div>
          </div>
        </div>

        <table className="inv__items-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Descripción</th>
              <th>Cant.</th>
              <th>Unidad</th>
              <th className="text-right">Precio unit.</th>
              <th className="text-right">Subtotal</th>
            </tr>
          </thead>
          <tbody>
            {invoice.items.map((item, i) => (
              <tr key={item.id}>
                <td>{i + 1}</td>
                <td>{item.description}</td>
                <td>{item.qty}</td>
                <td>{item.unit}</td>
                <td className="text-right">{fmtMoney(item.unitPrice)}</td>
                <td className="text-right">{fmtMoney(item.qty * item.unitPrice)}</td>
              </tr>
            ))}
          </tbody>
        </table>

        <div className="inv__totals">
          <div className="inv__totals-row"><span>Subtotal</span><span>{fmtMoney(sub)}</span></div>
          {invoice.discount > 0 && <div className="inv__totals-row"><span>Descuento ({invoice.discount}%)</span><span>-{fmtMoney(discountAmt)}</span></div>}
          <div className="inv__totals-row"><span>IVA ({invoice.tax}%)</span><span>{fmtMoney(taxAmt)}</span></div>
          <div className="inv__totals-row inv__totals-row--total"><span>Total</span><span>{fmtMoney(total)}</span></div>
        </div>

        <div className="inv__signature">
          <div>________________<br />Cliente</div>
          <div>________________<br />Autorizado</div>
        </div>
      </div>
    </div>
  );
}

// ─── Form (Add / Edit) ───

function InvoiceForm({
  invoice,
  onSave,
  onBack,
}: {
  invoice: Invoice | null;
  onSave: (inv: Invoice) => void;
  onBack: () => void;
}) {
  const isEdit = invoice !== null;
  const [customer, setCustomer] = useState(invoice?.customer ?? '');
  const [issuedDate, setIssuedDate] = useState(invoice?.issuedDate ?? new Date().toISOString().slice(0, 10));
  const [dueDate, setDueDate] = useState(invoice?.dueDate ?? '');
  const [discount, setDiscount] = useState(invoice?.discount ?? 0);
  const [tax, setTax] = useState(invoice?.tax ?? 21);
  const [status, setStatus] = useState<InvoiceStatus>(invoice?.status ?? 'pending');
  const [items, setItems] = useState<LineItem[]>(
    invoice?.items ?? [{ id: lineUid(), description: '', qty: 1, unit: 'unidad', unitPrice: 0 }],
  );

  const updateItem = (id: string, field: keyof LineItem, value: string | number) => {
    setItems(prev => prev.map(it => it.id === id ? { ...it, [field]: value } : it));
  };

  const removeItem = (id: string) => {
    if (items.length <= 1) return;
    setItems(prev => prev.filter(it => it.id !== id));
  };

  const addItem = () => {
    setItems(prev => [...prev, { id: lineUid(), description: '', qty: 1, unit: 'unidad', unitPrice: 0 }]);
  };

  const sub = calcSubtotal(items);
  const afterDiscount = sub * (1 - discount / 100);
  const total = afterDiscount * (1 + tax / 100);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!customer.trim()) return;
    onSave({
      id: invoice?.id ?? invUid(),
      number: invoice?.number ?? `INV-${3500 + Math.floor(Math.random() * 100)}`,
      customer: customer.trim(),
      initials: initials(customer.trim()),
      issuedDate,
      dueDate: dueDate || issuedDate,
      status,
      items: items.filter(it => it.description.trim()),
      discount,
      tax,
    });
  };

  return (
    <div className="card">
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 'var(--space-4)' }}>
        <button type="button" className="btn-secondary btn-sm" onClick={onBack}>&larr; Volver</button>
        <h2 style={{ margin: 0, fontSize: '1.1rem' }}>{isEdit ? 'Editar factura' : 'Nueva factura'}</h2>
      </div>

      <form onSubmit={handleSubmit}>
        <div className="inv__form-grid">
          <div className="form-group">
            <label htmlFor="inv-customer">Cliente</label>
            <input id="inv-customer" type="text" value={customer} onChange={e => setCustomer(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="inv-status">Estado</label>
            <select id="inv-status" value={status} onChange={e => setStatus(e.target.value as InvoiceStatus)}>
              <option value="pending">Pendiente</option>
              <option value="paid">Pagada</option>
              <option value="overdue">Vencida</option>
            </select>
          </div>
          <div className="form-group">
            <label htmlFor="inv-issued">Fecha emisión</label>
            <input id="inv-issued" type="date" value={issuedDate} onChange={e => setIssuedDate(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="inv-due">Vencimiento</label>
            <input id="inv-due" type="date" value={dueDate} onChange={e => setDueDate(e.target.value)} />
          </div>
          <div className="form-group">
            <label htmlFor="inv-discount">Descuento (%)</label>
            <input id="inv-discount" type="number" min={0} max={100} value={discount} onChange={e => setDiscount(Number(e.target.value))} />
          </div>
          <div className="form-group">
            <label htmlFor="inv-tax">IVA (%)</label>
            <input id="inv-tax" type="number" min={0} max={100} value={tax} onChange={e => setTax(Number(e.target.value))} />
          </div>
        </div>

        <div className="inv__line-items">
          <label style={{ marginBottom: '0.5rem', display: 'block' }}>Ítems</label>
          {items.map((item, i) => (
            <div key={item.id} className="inv__line-row">
              <div className="form-group">
                <input placeholder="Descripción" value={item.description} onChange={e => updateItem(item.id, 'description', e.target.value)} />
              </div>
              <div className="form-group" style={{ maxWidth: 80 }}>
                <input type="number" min={1} placeholder="Cant." value={item.qty} onChange={e => updateItem(item.id, 'qty', Number(e.target.value))} />
              </div>
              <div className="form-group" style={{ maxWidth: 100 }}>
                <input placeholder="Unidad" value={item.unit} onChange={e => updateItem(item.id, 'unit', e.target.value)} />
              </div>
              <div className="form-group" style={{ maxWidth: 120 }}>
                <input type="number" min={0} placeholder="Precio" value={item.unitPrice} onChange={e => updateItem(item.id, 'unitPrice', Number(e.target.value))} />
              </div>
              <button type="button" className="inv__remove-line" onClick={() => removeItem(item.id)} title="Quitar">✕</button>
            </div>
          ))}
          <button type="button" className="btn-secondary btn-sm inv__add-line" onClick={addItem}>+ Agregar ítem</button>
        </div>

        <div className="inv__totals" style={{ marginTop: 'var(--space-4)' }}>
          <div className="inv__totals-row"><span>Subtotal</span><span>{fmtMoney(sub)}</span></div>
          {discount > 0 && <div className="inv__totals-row"><span>Descuento ({discount}%)</span><span>-{fmtMoney(sub * discount / 100)}</span></div>}
          <div className="inv__totals-row"><span>IVA ({tax}%)</span><span>{fmtMoney(afterDiscount * tax / 100)}</span></div>
          <div className="inv__totals-row inv__totals-row--total"><span>Total</span><span>{fmtMoney(total)}</span></div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: 'var(--space-4)' }}>
          <button type="button" className="btn-secondary btn-sm" onClick={onBack}>Cancelar</button>
          <button type="submit" className="btn-primary btn-sm">{isEdit ? 'Guardar' : 'Crear factura'}</button>
        </div>
      </form>
    </div>
  );
}

// ─── Página principal ───

export function InvoiceDemoPage() {
  const [invoices, setInvoices] = useState<Invoice[]>(DEMO_INVOICES);
  const [view, setView] = useState<View>('list');
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const selectedInvoice = selectedId ? invoices.find(i => i.id === selectedId) ?? null : null;

  const handleView = useCallback((id: string) => { setSelectedId(id); setView('preview'); }, []);
  const handleEdit = useCallback((id: string) => { setSelectedId(id); setView('edit'); }, []);
  const handleAdd = useCallback(() => { setSelectedId(null); setView('add'); }, []);
  const handleBack = useCallback(() => { setSelectedId(null); setView('list'); }, []);

  const handleDelete = useCallback((id: string) => {
    if (!window.confirm('¿Eliminar esta factura?')) return;
    setInvoices(prev => prev.filter(i => i.id !== id));
  }, []);

  const handleSave = useCallback((inv: Invoice) => {
    setInvoices(prev => {
      const idx = prev.findIndex(i => i.id === inv.id);
      if (idx >= 0) {
        const next = [...prev];
        next[idx] = inv;
        return next;
      }
      return [inv, ...prev];
    });
    setView('list');
    setSelectedId(null);
  }, []);

  return (
    <div className="inv">
      <div className="page-header">
        <h1>Facturación</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>
          Gestión de facturas — listado, vista previa, creación y edición
        </p>
      </div>

      {view === 'list' && (
        <InvoiceList invoices={invoices} onView={handleView} onEdit={handleEdit} onAdd={handleAdd} onDelete={handleDelete} />
      )}
      {view === 'preview' && selectedInvoice && (
        <InvoicePreview invoice={selectedInvoice} onBack={handleBack} onEdit={() => setView('edit')} />
      )}
      {view === 'add' && (
        <InvoiceForm invoice={null} onSave={handleSave} onBack={handleBack} />
      )}
      {view === 'edit' && selectedInvoice && (
        <InvoiceForm invoice={selectedInvoice} onSave={handleSave} onBack={handleBack} />
      )}
    </div>
  );
}

export default InvoiceDemoPage;
