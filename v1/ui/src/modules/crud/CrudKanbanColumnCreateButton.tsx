type Props = {
  label: string;
  onClick: () => void;
};

export function CrudKanbanColumnCreateButton({ label, onClick }: Props) {
  return (
    <button type="button" className="m-kanban__column-add crud-kanban__column-add" onClick={onClick}>
      <span className="m-kanban__column-add-icon crud-kanban__column-add-icon" aria-hidden="true">
        +
      </span>
      {label}
    </button>
  );
}
