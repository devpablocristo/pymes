export function crudButtonClass(
  kind?: 'primary' | 'secondary' | 'danger' | 'success',
  size: 'sm' | 'md' = 'sm',
): string {
  const prefix = size === 'md' ? 'btn' : `btn-${size}`;
  switch (kind) {
    case 'primary':
      return `${prefix} btn-primary`;
    case 'danger':
      return `${prefix} btn-danger`;
    case 'success':
      return `${prefix} btn-success`;
    default:
      return `${prefix} btn-secondary`;
  }
}
