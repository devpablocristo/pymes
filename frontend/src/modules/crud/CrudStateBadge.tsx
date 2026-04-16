import './CrudStateBadge.css';

type Props = {
  label: string;
  variant?: 'default' | 'info' | 'warning' | 'success' | 'danger';
};

export function CrudStateBadge({ label, variant = 'default' }: Props) {
  return <span className={`crud-state-badge crud-state-badge--${variant}`}>{label}</span>;
}
