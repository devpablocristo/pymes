import { confirmAction } from '@devpablocristo/core-browser';
import { IconClose } from '@devpablocristo/modules-ui-data-display/icons';
import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { CrudTableSurface, type CrudTableSurfaceColumn, type CrudTableSurfaceRowAction } from '../crud';
import { useCrudArchivedSearchParam } from '../crud';
import type { CrudResourceShellHeaderConfigLike } from '../crud/CrudResourceShellHeader';
import {
  calcCommercialDocumentTotal,
  calcCommercialDocumentTotals,
  formatCommercialDocumentMoney,
  type CommercialDocumentLine,
  type CommercialDocumentRecord,
} from './commercialDocumentMath';
import './CommercialDocumentWorkspace.css';

export type CommercialDocumentStatusOption<TStatus extends string> = {
  value: TStatus;
  label: string;
  badgeClass: string;
};

type Props<TStatus extends string, TRecord extends CommercialDocumentRecord<TStatus>> = {
  resourceId: string;
  documents: TRecord[];
  onDocumentsChange: (documents: TRecord[]) => void;
  statusOptions: Array<CommercialDocumentStatusOption<TStatus>>;
  createLabel: string;
  createDocument: (draft: CommercialDocumentDraft<TStatus, TRecord>) => TRecord;
  createEmptyLine: () => CommercialDocumentLine;
  isArchived?: (document: TRecord) => boolean;
  archiveDocument?: (document: TRecord) => TRecord;
  restoreDocument?: (document: TRecord) => TRecord;
  reload?: () => Promise<void>;
  companyBlock?: ReactNode;
  shellConfig?: CrudResourceShellHeaderConfigLike<TRecord> | null;
};

type CommercialDocumentDraft<TStatus extends string, TRecord extends CommercialDocumentRecord<TStatus>> = Omit<
  TRecord,
  'id' | 'number' | 'initials'
> & { customer: string };

type WorkspaceView = 'list' | 'preview' | 'create' | 'edit';

function DocumentStatusBadge<TStatus extends string>({
  status,
  statusOptions,
}: {
  status: TStatus;
  statusOptions: Array<CommercialDocumentStatusOption<TStatus>>;
}) {
  const option = statusOptions.find((entry) => entry.value === status);
  if (!option) return <span className="badge">{status}</span>;
  return <span className={`badge ${option.badgeClass}`}>{option.label}</span>;
}

export function CommercialDocumentWorkspace<TStatus extends string, TRecord extends CommercialDocumentRecord<TStatus>>({
  resourceId,
  documents,
  onDocumentsChange,
  statusOptions,
  createLabel,
  createDocument,
  createEmptyLine,
  isArchived = () => false,
  archiveDocument,
  restoreDocument,
  reload = async () => {},
  shellConfig = null,
  companyBlock = (
    <>
      <strong>Mi Empresa S.R.L.</strong>
      <br />
      Av. Corrientes 1234, CABA
      <br />
      info@miempresa.com
    </>
  ),
}: Props<TStatus, TRecord>) {
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const [view, setView] = useState<WorkspaceView>('list');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<TStatus | 'all'>('all');

  const selectedDocument = useMemo(
    () => (selectedId ? documents.find((document) => document.id === selectedId) ?? null : null),
    [documents, selectedId],
  );

  const visibleDocuments = useMemo(() => {
    const query = search.trim().toLowerCase();
    return documents.filter((document) => {
      if (showArchived !== isArchived(document)) return false;
      if (statusFilter !== 'all' && document.status !== statusFilter) return false;
      if (!query) return true;
      return [document.number, document.customer, document.status].join(' ').toLowerCase().includes(query);
    });
  }, [documents, isArchived, search, showArchived, statusFilter]);
  const showStatusSelector = shellConfig?.featureFlags?.statusSelector !== false;

  const columns = useMemo<CrudTableSurfaceColumn<TRecord>[]>(
    () => [
      {
        id: 'number',
        header: 'N°',
        render: (row) => <strong>{row.number}</strong>,
      },
      {
        id: 'customer',
        header: 'Cliente',
        render: (row) => (
          <div className="commercial-document__customer">
            <span className="commercial-document__avatar">{row.initials}</span>
            {row.customer}
          </div>
        ),
      },
      {
        id: 'issuedDate',
        header: 'Fecha',
        render: (row) =>
          new Date(row.issuedDate).toLocaleDateString('es-AR', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          }),
      },
      {
        id: 'total',
        header: 'Total',
        render: (row) => <strong>{formatCommercialDocumentMoney(calcCommercialDocumentTotal(row))}</strong>,
      },
      {
        id: 'status',
        header: 'Estado',
        render: (row) => <DocumentStatusBadge status={row.status} statusOptions={statusOptions} />,
      },
    ],
    [statusOptions],
  );

  const rowActions = useMemo<CrudTableSurfaceRowAction<TRecord>[]>(
    () => [
      {
        id: 'preview',
        label: 'Ver',
        onClick: (row) => {
          setSelectedId(row.id);
          setView('preview');
        },
      },
      {
        id: showArchived ? 'restore' : 'edit',
        label: showArchived ? 'Restaurar' : 'Editar',
        kind: 'secondary',
        onClick: (row) => {
          if (showArchived) {
            if (!restoreDocument) return;
            onDocumentsChange(documents.map((document) => (document.id === row.id ? restoreDocument(document) : document)));
            setSelectedId((current) => (current === row.id ? null : current));
            setView('list');
            return;
          }
          setSelectedId(row.id);
          setView('edit');
        },
      },
      {
        id: showArchived ? 'delete' : 'archive',
        label: showArchived ? 'Eliminar' : 'Archivar',
        kind: 'danger',
        onClick: async (row) => {
          const confirmed = await confirmAction({
            title: `${showArchived ? 'Eliminar' : 'Archivar'} ${resourceId === 'invoices' ? 'factura' : 'documento'}`,
            description: showArchived ? '¿Eliminar este documento?' : '¿Archivar este documento?',
            confirmLabel: showArchived ? 'Eliminar' : 'Archivar',
            cancelLabel: 'Cancelar',
            tone: 'danger',
          });
          if (!confirmed) return;
          if (!showArchived && archiveDocument) {
            onDocumentsChange(documents.map((document) => (document.id === row.id ? archiveDocument(document) : document)));
            setSelectedId((current) => (current === row.id ? null : current));
            setView('list');
            return;
          }
          onDocumentsChange(documents.filter((document) => document.id !== row.id));
          setSelectedId((current) => (current === row.id ? null : current));
          setView('list');
        },
      },
    ],
    [archiveDocument, documents, onDocumentsChange, resourceId, restoreDocument, showArchived],
  );

  return (
    <div className="commercial-document">
      <PymesCrudResourceShellHeader<TRecord>
        resourceId={resourceId}
        crudConfigOverride={shellConfig}
        preserveCsvToolbar
        items={visibleDocuments}
        subtitleCount={visibleDocuments.length}
        loading={false}
        error={null}
        setError={() => {}}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        searchInlineActions={
          showStatusSelector ? (
            <select
              className="commercial-document__status-filter"
              aria-label="Filtrar documentos por estado"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as TStatus | 'all')}
            >
              <option value="all">Todas</option>
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          ) : null
        }
        extraHeaderActions={
          <>
            <button
              type="button"
              className="btn-primary btn-sm"
              disabled={showArchived}
              onClick={() => {
                setSelectedId(null);
                setView('create');
              }}
            >
              {createLabel}
            </button>
          </>
        }
      />

      {view === 'list' ? (
        <div className="card">
          <CrudTableSurface
            items={visibleDocuments}
            columns={columns}
            rowActions={rowActions}
            onRowClick={(row) => {
              setSelectedId(row.id);
              setView('preview');
            }}
            selectedId={selectedId}
          />
          <div className="commercial-document__pagination">
            <span>
              Mostrando {visibleDocuments.length} de {documents.length}
            </span>
          </div>
        </div>
      ) : null}

      {view === 'preview' && selectedDocument ? (
        <CommercialDocumentPreviewCard
          document={selectedDocument}
          statusOptions={statusOptions}
          companyBlock={companyBlock}
          onBack={() => {
            setView('list');
            setSelectedId(null);
          }}
          onEdit={() => setView('edit')}
          showEdit={!showArchived}
        />
      ) : null}

      {view === 'create' && !showArchived ? (
        <CommercialDocumentFormCard<TStatus, TRecord>
          document={null}
          statusOptions={statusOptions}
          createEmptyLine={createEmptyLine}
          onBack={() => setView('list')}
          onSave={(draft) => {
            const next = createDocument(draft);
            onDocumentsChange([next, ...documents]);
            setSelectedId(next.id);
            setView('preview');
          }}
        />
      ) : null}

      {view === 'edit' && selectedDocument && !showArchived ? (
        <CommercialDocumentFormCard<TStatus, TRecord>
          document={selectedDocument}
          statusOptions={statusOptions}
          createEmptyLine={createEmptyLine}
          onBack={() => setView('preview')}
          onSave={(draft) => {
            const next = { ...selectedDocument, ...draft } as TRecord;
            onDocumentsChange(documents.map((document) => (document.id === next.id ? next : document)));
            setSelectedId(next.id);
            setView('preview');
          }}
        />
      ) : null}
    </div>
  );
}

function CommercialDocumentPreviewCard<TStatus extends string, TRecord extends CommercialDocumentRecord<TStatus>>({
  document,
  statusOptions,
  companyBlock,
  onBack,
  onEdit,
  showEdit,
}: {
  document: TRecord;
  statusOptions: Array<CommercialDocumentStatusOption<TStatus>>;
  companyBlock: ReactNode;
  onBack: () => void;
  onEdit: () => void;
  showEdit: boolean;
}) {
  const totals = calcCommercialDocumentTotals(document);

  return (
    <div className="card">
      <div className="commercial-document__preview-toolbar">
        <button type="button" className="btn-secondary btn-sm" onClick={onBack}>
          Volver
        </button>
        {showEdit ? (
          <button type="button" className="btn-secondary btn-sm" onClick={onEdit}>
            Editar
          </button>
        ) : null}
        <button type="button" className="btn-secondary btn-sm" onClick={() => window.print()}>
          Imprimir
        </button>
      </div>

      <div className="commercial-document__preview">
        <div className="commercial-document__preview-header">
          <div>
            <h2 className="commercial-document__preview-number">{document.number}</h2>
            <DocumentStatusBadge status={document.status} statusOptions={statusOptions} />
          </div>
          <div className="commercial-document__preview-company">{companyBlock}</div>
        </div>

        <div className="commercial-document__detail-grid">
          <div>
            <div className="commercial-document__detail-label">Cliente</div>
            <div className="commercial-document__detail-value">{document.customer}</div>
          </div>
          <div>
            <div className="commercial-document__detail-label">Fecha emisión</div>
            <div className="commercial-document__detail-value">{new Date(document.issuedDate).toLocaleDateString('es-AR')}</div>
          </div>
          <div>
            <div className="commercial-document__detail-label">Vencimiento</div>
            <div className="commercial-document__detail-value">{new Date(document.dueDate).toLocaleDateString('es-AR')}</div>
          </div>
          <div>
            <div className="commercial-document__detail-label">N° Documento</div>
            <div className="commercial-document__detail-value">{document.number}</div>
          </div>
        </div>

        <table className="commercial-document__items-table">
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
            {document.items.map((item, index) => (
              <tr key={item.id}>
                <td>{index + 1}</td>
                <td>{item.description}</td>
                <td>{item.qty}</td>
                <td>{item.unit}</td>
                <td className="text-right">{formatCommercialDocumentMoney(item.unitPrice)}</td>
                <td className="text-right">{formatCommercialDocumentMoney(item.qty * item.unitPrice)}</td>
              </tr>
            ))}
          </tbody>
        </table>

        <div className="commercial-document__totals">
          <div className="commercial-document__totals-row">
            <span>Subtotal</span>
            <span>{formatCommercialDocumentMoney(totals.subtotal)}</span>
          </div>
          {document.discount > 0 ? (
            <div className="commercial-document__totals-row">
              <span>Descuento ({document.discount}%)</span>
              <span>-{formatCommercialDocumentMoney(totals.discountAmount)}</span>
            </div>
          ) : null}
          <div className="commercial-document__totals-row">
            <span>IVA ({document.tax}%)</span>
            <span>{formatCommercialDocumentMoney(totals.taxAmount)}</span>
          </div>
          <div className="commercial-document__totals-row commercial-document__totals-row--total">
            <span>Total</span>
            <span>{formatCommercialDocumentMoney(totals.total)}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function CommercialDocumentFormCard<TStatus extends string, TRecord extends CommercialDocumentRecord<TStatus>>({
  document,
  statusOptions,
  createEmptyLine,
  onBack,
  onSave,
}: {
  document: TRecord | null;
  statusOptions: Array<CommercialDocumentStatusOption<TStatus>>;
  createEmptyLine: () => CommercialDocumentLine;
  onBack: () => void;
  onSave: (document: CommercialDocumentDraft<TStatus, TRecord>) => void;
}) {
  const isEdit = document != null;
  const [customer, setCustomer] = useState(document?.customer ?? '');
  const [issuedDate, setIssuedDate] = useState(document?.issuedDate ?? new Date().toISOString().slice(0, 10));
  const [dueDate, setDueDate] = useState(document?.dueDate ?? '');
  const [discount, setDiscount] = useState(document?.discount ?? 0);
  const [tax, setTax] = useState(document?.tax ?? 21);
  const [status, setStatus] = useState<TStatus>(document?.status ?? statusOptions[0].value);
  const [items, setItems] = useState<CommercialDocumentLine[]>(document?.items.length ? document.items : [createEmptyLine()]);

  const updateItem = useCallback((id: string, field: keyof CommercialDocumentLine, value: string | number) => {
    setItems((current) => current.map((item) => (item.id === id ? { ...item, [field]: value } : item)));
  }, []);

  const removeItem = useCallback((id: string) => {
    setItems((current) => (current.length <= 1 ? current : current.filter((item) => item.id !== id)));
  }, []);

  const addItem = useCallback(() => {
    setItems((current) => [...current, createEmptyLine()]);
  }, [createEmptyLine]);

  const totals = calcCommercialDocumentTotals({ items, discount, tax });

  return (
    <div className="card">
      <div className="commercial-document__form-back">
        <button type="button" className="btn-secondary btn-sm" onClick={onBack}>
          Volver
        </button>
      </div>

      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!customer.trim()) return;
          onSave({
            customer: customer.trim(),
            issuedDate,
            dueDate: dueDate || issuedDate,
            status,
            items: items.filter((item) => item.description.trim()),
            discount,
            tax,
          } as CommercialDocumentDraft<TStatus, TRecord>);
        }}
      >
        <div className="commercial-document__form-grid">
          <div className="form-group">
            <label htmlFor="commercial-document-customer">Cliente</label>
            <input id="commercial-document-customer" value={customer} onChange={(e) => setCustomer(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="commercial-document-status">Estado</label>
            <select id="commercial-document-status" value={status} onChange={(e) => setStatus(e.target.value as TStatus)}>
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label htmlFor="commercial-document-issued">Fecha emisión</label>
            <input id="commercial-document-issued" type="date" value={issuedDate} onChange={(e) => setIssuedDate(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="commercial-document-due">Vencimiento</label>
            <input id="commercial-document-due" type="date" value={dueDate} onChange={(e) => setDueDate(e.target.value)} />
          </div>
          <div className="form-group">
            <label htmlFor="commercial-document-discount">Descuento (%)</label>
            <input id="commercial-document-discount" type="number" min={0} max={100} value={discount} onChange={(e) => setDiscount(Number(e.target.value))} />
          </div>
          <div className="form-group">
            <label htmlFor="commercial-document-tax">IVA (%)</label>
            <input id="commercial-document-tax" type="number" min={0} max={100} value={tax} onChange={(e) => setTax(Number(e.target.value))} />
          </div>
        </div>

        <div className="commercial-document__line-items">
          <label className="commercial-document__line-items-label">Ítems</label>
          {items.map((item) => (
            <div key={item.id} className="commercial-document__line-row">
              <div className="form-group">
                <input
                  aria-label="Descripción del ítem"
                  placeholder="Descripción"
                  value={item.description}
                  onChange={(e) => updateItem(item.id, 'description', e.target.value)}
                />
              </div>
              <div className="form-group commercial-document__line-qty">
                <input aria-label="Cantidad" type="number" min={1} value={item.qty} onChange={(e) => updateItem(item.id, 'qty', Number(e.target.value))} />
              </div>
              <div className="form-group commercial-document__line-unit">
                <input aria-label="Unidad" value={item.unit} onChange={(e) => updateItem(item.id, 'unit', e.target.value)} />
              </div>
              <div className="form-group commercial-document__line-price">
                <input
                  aria-label="Precio unitario"
                  type="number"
                  min={0}
                  value={item.unitPrice}
                  onChange={(e) => updateItem(item.id, 'unitPrice', Number(e.target.value))}
                />
              </div>
              <button
                type="button"
                className="commercial-document__remove-line"
                onClick={() => removeItem(item.id)}
                aria-label="Quitar ítem"
              >
                <IconClose />
              </button>
            </div>
          ))}
          <button type="button" className="btn-secondary btn-sm commercial-document__add-line" onClick={addItem}>
            + Agregar ítem
          </button>
        </div>

        <div className="commercial-document__totals commercial-document__totals--form">
          <div className="commercial-document__totals-row">
            <span>Subtotal</span>
            <span>{formatCommercialDocumentMoney(totals.subtotal)}</span>
          </div>
          {discount > 0 ? (
            <div className="commercial-document__totals-row">
              <span>Descuento ({discount}%)</span>
              <span>-{formatCommercialDocumentMoney(totals.discountAmount)}</span>
            </div>
          ) : null}
          <div className="commercial-document__totals-row">
            <span>IVA ({tax}%)</span>
            <span>{formatCommercialDocumentMoney(totals.taxAmount)}</span>
          </div>
          <div className="commercial-document__totals-row commercial-document__totals-row--total">
            <span>Total</span>
            <span>{formatCommercialDocumentMoney(totals.total)}</span>
          </div>
        </div>

        <div className="commercial-document__form-actions">
          <button type="button" className="btn-secondary btn-sm" onClick={onBack}>
            Cancelar
          </button>
          <button type="submit" className="btn-primary btn-sm">
            {isEdit ? 'Guardar documento' : 'Crear documento'}
          </button>
        </div>
      </form>
    </div>
  );
}
