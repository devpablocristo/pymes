import type { CrudHelpers, CrudToolbarAction } from '@devpablocristo/modules-crud-ui';
import { crudButtonClass } from './crudButtonClass';

export function CrudToolbarActionButtons<T extends { id: string }>({
  actions,
  items,
  archived,
  reload,
  setError,
  formatLabel = (label) => label,
  buttonClassName = crudButtonClass,
}: {
  actions?: CrudToolbarAction<T>[];
  items: T[];
  archived: boolean;
  reload: () => Promise<void>;
  setError: (message: string | null) => void;
  formatLabel?: (label: string) => string;
  buttonClassName?: (kind?: 'primary' | 'secondary' | 'danger' | 'success') => string;
}) {
  const visibleActions = (actions ?? []).filter((action) => action.isVisible?.({ archived, items }) ?? true);
  const helpers: CrudHelpers<T> = {
    items,
    reload,
    setError: (message: string) => setError(message),
  };

  return (
    <>
      {visibleActions.map((action) => (
        <button
          key={action.id}
          type="button"
          className={buttonClassName(action.kind)}
          onClick={() => {
            void action.onClick(helpers);
          }}
        >
          {formatLabel(action.label)}
        </button>
      ))}
    </>
  );
}
