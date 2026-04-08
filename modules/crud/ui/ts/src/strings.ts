/**
 * Cadenas UI del CRUD: plantillas con `{{variable}}` y presets en inglés / español.
 * Los textos visibles al usuario no son lógica de negocio; son datos de presentación.
 */
/** Reemplaza `{{clave}}`; claves ausentes → cadena vacía. */
export function interpolate(template: string, vars: Record<string, string>): string {
  return template.replace(/\{\{(\w+)\}\}/g, (_, key: string) => vars[key] ?? "");
}

export type CrudStrings = {
  statusLoading: string;
  statusSaving: string;
  actionSave: string;
  actionCancel: string;
  actionEdit: string;
  actionDelete: string;
  actionArchive: string;
  actionRestore: string;
  actionConfirm: string;
  titleArchived: string;
  searchPlaceholder: string;
  selectPlaceholder: string;
  toggleShowActive: string;
  toggleShowArchived: string;
  emptySearch: string;
  emptyArchived: string;
  emptyActive: string;
  tableActions: string;
  buttonNew: string;
  buttonCreateFirst: string;
  formEdit: string;
  formCreate: string;
  confirmHint: string;
  confirmPlaceholder: string;
  confirmWord: string;
  loadMore: string;
};

export const defaultCrudStrings: CrudStrings = {
  statusLoading: "Loading…",
  statusSaving: "Saving…",
  actionSave: "Save",
  actionCancel: "Cancel",
  actionEdit: "Edit",
  actionDelete: "Delete",
  actionArchive: "Archive",
  actionRestore: "Restore",
  actionConfirm: "Confirm",
  titleArchived: "Archived {{labelPluralCap}}",
  searchPlaceholder: "Search {{labelPlural}}…",
  selectPlaceholder: "Select…",
  toggleShowActive: "Show active",
  toggleShowArchived: "Show archived",
  emptySearch: "No {{labelPlural}} match “{{search}}”.",
  emptyArchived: "No archived {{labelPlural}}.",
  emptyActive: "No {{labelPlural}} yet.",
  tableActions: "Actions",
  buttonNew: "New {{label}}",
  buttonCreateFirst: "Create first {{label}}",
  formEdit: "Edit {{label}}",
  formCreate: "New {{label}}",
  confirmHint: "Type {{word}} to confirm.",
  confirmPlaceholder: "Confirmation word",
  confirmWord: "delete",
  loadMore: "Load more",
};

export const crudStringsEs: CrudStrings = {
  statusLoading: "Cargando…",
  statusSaving: "Guardando…",
  actionSave: "Guardar",
  actionCancel: "Cancelar",
  actionEdit: "Editar",
  actionDelete: "Eliminar",
  actionArchive: "Archivar",
  actionRestore: "Restaurar",
  actionConfirm: "Confirmar",
  titleArchived: "{{labelPluralCap}} archivados",
  searchPlaceholder: "Buscar {{labelPlural}}…",
  selectPlaceholder: "Seleccionar…",
  toggleShowActive: "Ver activos",
  toggleShowArchived: "Ver archivados",
  emptySearch: "No hay {{labelPlural}} que coincidan con “{{search}}”.",
  emptyArchived: "No hay {{labelPlural}} archivados.",
  emptyActive: "Todavía no hay {{labelPlural}}.",
  tableActions: "Acciones",
  buttonNew: "Nuevo {{label}}",
  buttonCreateFirst: "Crear primer {{label}}",
  formEdit: "Editar {{label}}",
  formCreate: "Nuevo {{label}}",
  confirmHint: "Escribí {{word}} para confirmar.",
  confirmPlaceholder: "Palabra de confirmación",
  confirmWord: "eliminar",
  loadMore: "Cargar más",
};

export function mergeCrudStrings(base: CrudStrings, partial?: Partial<CrudStrings>): CrudStrings {
  return { ...base, ...partial };
}
