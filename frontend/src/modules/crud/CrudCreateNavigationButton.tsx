import { useNavigate } from 'react-router-dom';
import { crudButtonClass } from './crudButtonClass';

export function CrudCreateNavigationButton({
  to,
  enabled = true,
  label,
  className = crudButtonClass('primary'),
}: {
  to: string;
  enabled?: boolean;
  label: string;
  className?: string;
}) {
  const navigate = useNavigate();

  if (!enabled) {
    return null;
  }

  return (
    <button
      type="button"
      className={className}
      onClick={() => {
        void navigate(to);
      }}
    >
      {label}
    </button>
  );
}
