export function renderProfessionalsBooleanBadge(
  value: boolean,
  trueLabel = 'Si',
  falseLabel = 'No',
) {
  return <span className={`badge ${value ? 'badge-success' : 'badge-neutral'}`}>{value ? trueLabel : falseLabel}</span>;
}

export function renderProfessionalsStatusBadge(value: unknown) {
  const status = String(value ?? '');
  const badgeClass =
    status === 'completed'
      ? 'badge-success'
      : status === 'reviewed'
        ? 'badge-success'
        : status === 'submitted' || status === 'active'
          ? 'badge-warning'
          : 'badge-neutral';
  return <span className={`badge ${badgeClass}`}>{status}</span>;
}

export function teacherSpecialtiesToText(
  specialties?: Array<string | { name?: string }>,
): string {
  return specialties?.map((item) => (typeof item === 'string' ? item : item.name)).filter(Boolean).join(', ') || '---';
}
