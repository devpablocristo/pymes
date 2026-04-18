export function renderCrudBooleanBadge(
  value: boolean,
  trueLabel = 'Sí',
  falseLabel = 'No',
  trueClassName = 'badge-success',
  falseClassName = 'badge-neutral',
) {
  return <span className={`badge ${value ? trueClassName : falseClassName}`}>{value ? trueLabel : falseLabel}</span>;
}

export function renderCrudActiveBadge(
  value: boolean,
  activeLabel = 'Activo',
  inactiveLabel = 'Inactivo',
) {
  return renderCrudBooleanBadge(value, activeLabel, inactiveLabel);
}

export function formatCrudMoney(value: unknown, currency?: string): string {
  return `${currency || 'ARS'} ${Number(value ?? 0).toFixed(2)}`;
}

export function formatCrudLocalizedMoney(
  value: unknown,
  currency = 'ARS',
  locale = 'es-AR',
  minimumFractionDigits = 0,
) {
  return Number(value ?? 0).toLocaleString(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits,
  });
}

export function formatCrudPercent(value: unknown): string {
  return `${Number(value ?? 0).toFixed(2)}%`;
}

export function hasReadableCrudValue(value: unknown): boolean {
  return String(value ?? '').trim().length > 0;
}
