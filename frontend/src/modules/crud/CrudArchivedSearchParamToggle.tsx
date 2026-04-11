import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';

export function CrudArchivedSearchParamToggle({
  archivedValue = '1',
  paramName = 'archived',
  showArchivedLabel = 'Ver archivadas',
  showActiveLabel = 'Ver activas',
  className = 'btn-secondary btn-sm',
  onToggle,
}: {
  archivedValue?: string;
  paramName?: string;
  showArchivedLabel?: string;
  showActiveLabel?: string;
  className?: string;
  onToggle?: (nextArchived: boolean) => void;
}) {
  const { archived: showArchived, toggleArchived } = useCrudArchivedSearchParam({ paramName, archivedValue });

  return (
    <button
      type="button"
      className={className}
      onClick={() => {
        const nextArchived = !showArchived;
        toggleArchived();
        onToggle?.(nextArchived);
      }}
    >
      {showArchived ? showActiveLabel : showArchivedLabel}
    </button>
  );
}
