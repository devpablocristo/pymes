import { Link } from 'react-router-dom';

export function SettingsDeepLink({ to, title }: { to: string; title: string; desc: string }) {
  return (
    <Link to={to} className="card stg__deep-link">
      <strong>{title}</strong>
    </Link>
  );
}

export function Toggle({
  checked,
  onChange,
  ariaLabel,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  ariaLabel?: string;
}) {
  return (
    <label className="stg__switch">
      <input type="checkbox" checked={checked} aria-label={ariaLabel} onChange={(e) => onChange(e.target.checked)} />
      <span className="stg__switch-slider" />
    </label>
  );
}
