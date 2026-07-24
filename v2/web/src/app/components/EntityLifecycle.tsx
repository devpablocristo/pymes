import type { ResourceState } from "@devpablocristo/platform-access-management";
import {
  LifecycleActionToolbar,
  type LifecycleBulkAction,
} from "@devpablocristo/platform-lifecycle";

export type EntityLifecycleState = ResourceState;
export type EntityLifecycleAction =
  | "archive"
  | "unarchive"
  | "trash"
  | "restore"
  | "purge";

export const entityLifecycleLabels: Record<EntityLifecycleState, string> = {
  active: "Activos",
  archived: "Archivados",
  trash: "Papelera",
};

export function toApiLifecycleState(state: EntityLifecycleState) {
  return state === "trash" ? "trashed" : state;
}

export function EntityLifecycleTabs({
  label,
  onChange,
  state,
}: {
  label: string;
  onChange: (state: EntityLifecycleState) => void;
  state: EntityLifecycleState;
}) {
  return (
    <div className="lifecycle-tabs" role="tablist" aria-label={label}>
      {(Object.keys(entityLifecycleLabels) as EntityLifecycleState[]).map(
        (candidate) => (
          <button
            aria-selected={state === candidate}
            className={state === candidate ? "is-active" : ""}
            key={candidate}
            onClick={() => onChange(candidate)}
            role="tab"
            type="button"
          >
            {entityLifecycleLabels[candidate]}
          </button>
        ),
      )}
    </div>
  );
}

export function EntityLifecycleBulkToolbar({
  busy,
  createLabel,
  editOpen = false,
  onClear,
  onCreate,
  onEdit,
  onAction,
  selectedCount,
  state,
}: {
  busy: boolean;
  createLabel?: string;
  editOpen?: boolean;
  onClear: () => void;
  onCreate?: () => void;
  onEdit?: () => void;
  onAction: (action: EntityLifecycleAction) => void;
  selectedCount: number;
  state: EntityLifecycleState;
}) {
  function apply(action: LifecycleBulkAction) {
    onAction(action === "restore" && state === "archived" ? "unarchive" : action);
  }

  return (
    <LifecycleActionToolbar
      busy={busy}
      createOpen={false}
      editOpen={editOpen}
      onBulkAction={apply}
      onClear={onClear}
      onCreate={onCreate ?? (() => undefined)}
      onEdit={onEdit}
      selectedCount={selectedCount}
      view={state}
      labels={{
        newButton: createLabel ?? "Nuevo",
        editButton: "Editar",
        clearButton: "Limpiar",
        archiveButton: "Archivar",
        trashButton: "Papelera",
        restoreButton: state === "archived" ? "Desarchivar" : "Restaurar",
        deleteButton: "Eliminar definitivamente",
        selectedSuffix: selectedCount === 1 ? "seleccionado" : "seleccionados",
      }}
      classNames={{
        root: "entity-bulk-toolbar",
        buttons: "entity-bulk-toolbar__buttons",
        group: "entity-bulk-toolbar__group",
        lifecycleGroup: "entity-bulk-toolbar__group--lifecycle",
        buttonBase: "entity-bulk-toolbar__button",
        primaryButton: "is-primary",
        secondaryButton: "is-secondary",
        dangerButton: "is-danger",
        newButton: onCreate ? "" : "entity-bulk-toolbar__new--hidden",
        selectedCount: "entity-bulk-toolbar__count",
        inlineNote: "entity-bulk-toolbar__note",
      }}
    />
  );
}

export type EntitySelectionAction = {
  id: string;
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onClick: () => void;
};

export function EntitySelectionToolbar({
  actions,
  busy,
  createLabel,
  onClear,
  onCreate,
  selectedCount,
}: {
  actions: EntitySelectionAction[];
  busy: boolean;
  createLabel?: string;
  onClear: () => void;
  onCreate?: () => void;
  selectedCount: number;
}) {
  const disabled = busy || selectedCount === 0;
  return (
    <div className="entity-bulk-toolbar">
      <div className="entity-bulk-toolbar__buttons">
        {onCreate ? (
          <div className="entity-bulk-toolbar__group">
            <button
              className="entity-bulk-toolbar__button is-primary"
              disabled={busy}
              onClick={onCreate}
              type="button"
            >
              {createLabel ?? "Nuevo"}
            </button>
          </div>
        ) : null}
        <div className="entity-bulk-toolbar__group">
          <button
            className="entity-bulk-toolbar__button is-secondary"
            disabled={disabled}
            onClick={onClear}
            type="button"
          >
            Limpiar
          </button>
        </div>
        <div className="entity-bulk-toolbar__group entity-bulk-toolbar__group--lifecycle">
          {actions.map((action) => (
            <button
              className={`entity-bulk-toolbar__button ${action.danger ? "is-danger" : "is-primary"}`}
              disabled={disabled || action.disabled}
              key={action.id}
              onClick={action.onClick}
              type="button"
            >
              {action.label}
            </button>
          ))}
        </div>
      </div>
      <span className="entity-bulk-toolbar__count">
        {selectedCount} {selectedCount === 1 ? "seleccionado" : "seleccionados"}
      </span>
    </div>
  );
}
