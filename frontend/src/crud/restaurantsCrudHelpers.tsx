export function renderRestaurantTableStatusBadge(value: unknown) {
  const status = String(value ?? '');
  const badgeClass =
    status === 'occupied'
      ? 'badge-warning'
      : status === 'reserved' || status === 'cleaning'
        ? 'badge-neutral'
        : 'badge-success';
  return <span className={`badge ${badgeClass}`}>{status || 'available'}</span>;
}
