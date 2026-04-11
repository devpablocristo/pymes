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

export function formatCrudPercent(value: unknown): string {
  return `${Number(value ?? 0).toFixed(2)}%`;
}
