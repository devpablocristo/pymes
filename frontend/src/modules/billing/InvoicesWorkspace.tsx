import { useEffect, useState } from 'react';
import { CommercialDocumentWorkspace, type CommercialDocumentStatusOption } from './CommercialDocumentWorkspace';
import { commercialDocumentInitials } from './commercialDocumentMath';
import { buildCommercialDocumentStatusOptions, createInvoicesShellConfig } from './billingHelpers';
import { archiveInvoice, createEmptyInvoiceLine, INVOICE_STATUS_BADGE_CLASS, INVOICE_STATUS_LABELS, nextInvoiceUid, readDemoInvoices, restoreInvoice, type InvoiceRecord, type InvoiceStatus, writeDemoInvoices } from './invoicesDemo';

const INVOICE_STATUS_OPTIONS: Array<CommercialDocumentStatusOption<InvoiceStatus>> = buildCommercialDocumentStatusOptions(
  INVOICE_STATUS_LABELS,
  INVOICE_STATUS_BADGE_CLASS,
);

const INVOICES_SHELL_CONFIG = createInvoicesShellConfig<InvoiceRecord>();

export function InvoicesWorkspace() {
  const [invoices, setInvoices] = useState<InvoiceRecord[]>(() => readDemoInvoices());

  useEffect(() => {
    writeDemoInvoices(invoices);
  }, [invoices]);

  const reload = async () => {
    setInvoices(readDemoInvoices());
  };

  return (
    <CommercialDocumentWorkspace<InvoiceStatus, InvoiceRecord>
      resourceId="invoices"
      documents={invoices}
      onDocumentsChange={setInvoices}
      statusOptions={INVOICE_STATUS_OPTIONS}
      createLabel="+ Nueva factura"
      createEmptyLine={createEmptyInvoiceLine}
      shellConfig={INVOICES_SHELL_CONFIG}
      isArchived={(invoice) => Boolean(invoice.archived_at)}
      archiveDocument={archiveInvoice}
      restoreDocument={restoreInvoice}
      reload={reload}
      createDocument={(draft) => ({
        id: nextInvoiceUid(),
        number: `INV-${3500 + Math.floor(Math.random() * 100)}`,
        initials: commercialDocumentInitials(draft.customer),
        ...draft,
        archived_at: null,
      })}
    />
  );
}
