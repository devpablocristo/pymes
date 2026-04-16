export type CommercialDocumentLine = {
  id: string;
  description: string;
  qty: number;
  unit: string;
  unitPrice: number;
};

export type CommercialDocumentRecord<TStatus extends string> = {
  id: string;
  number: string;
  customer: string;
  initials: string;
  issuedDate: string;
  dueDate: string;
  status: TStatus;
  items: CommercialDocumentLine[];
  discount: number;
  tax: number;
};

export function calcCommercialDocumentSubtotal(items: CommercialDocumentLine[]): number {
  return items.reduce((sum, item) => sum + item.qty * item.unitPrice, 0);
}

export function calcCommercialDocumentTotal<TStatus extends string>(
  document: Pick<CommercialDocumentRecord<TStatus>, 'items' | 'discount' | 'tax'>,
): number {
  const subtotal = calcCommercialDocumentSubtotal(document.items);
  const afterDiscount = subtotal * (1 - document.discount / 100);
  return afterDiscount * (1 + document.tax / 100);
}

export function calcCommercialDocumentTotals<TStatus extends string>(
  document: Pick<CommercialDocumentRecord<TStatus>, 'items' | 'discount' | 'tax'>,
) {
  const subtotal = calcCommercialDocumentSubtotal(document.items);
  const discountAmount = subtotal * (document.discount / 100);
  const afterDiscount = subtotal - discountAmount;
  const taxAmount = afterDiscount * (document.tax / 100);
  const total = afterDiscount + taxAmount;
  return { subtotal, discountAmount, afterDiscount, taxAmount, total };
}

export function formatCommercialDocumentMoney(value: number, currency = 'ARS'): string {
  return value.toLocaleString('es-AR', {
    style: 'currency',
    currency,
    minimumFractionDigits: 0,
  });
}

export function commercialDocumentInitials(name: string): string {
  return name
    .split(' ')
    .map((word) => word[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
}
