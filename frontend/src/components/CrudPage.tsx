import { useMemo, type ReactElement, type ReactNode } from 'react';
import {
  CrudPage as ModulesCrudPage,
  type CrudFeatureFlags as ModulesCrudFeatureFlags,
  type CrudFieldValue as ModulesCrudFieldValue,
  type CrudFormValues as ModulesCrudFormValues,
  type CrudPageConfig as ModulesCrudPageConfig,
  type CrudViewModeConfig as ModulesCrudViewModeConfig,
  type CrudViewModeId as ModulesCrudViewModeId,
} from '@devpablocristo/modules-crud-ui';
import { apiRequest } from '../lib/api';
import { buildPymesCrudStrings } from '../lib/crudModuleStrings';
import { useI18n } from '../lib/i18n';

export type CrudViewModeId = ModulesCrudViewModeId;
export type CrudViewModeConfig = ModulesCrudViewModeConfig & {
  /** Vista no-lista definida junto al recurso en `resourceConfigs.*`. */
  render?: () => ReactElement;
};

export type CrudFeatureFlags = ModulesCrudFeatureFlags & {
  /** Selector junto al buscador para filtrar por estado/etiqueta cuando aplique. */
  valueFilter?: boolean;
};

export type CrudExplorerMetricConfig<T> = {
  id: string;
  label: string;
  value: (items: T[]) => string;
  tone?: 'default' | 'success' | 'warning' | 'danger';
  helper?: string | ((items: T[]) => string | undefined);
};

export type CrudExplorerFilterConfig<T> = {
  id: string;
  label: string;
  predicate: (row: T) => boolean;
};

export type CrudExplorerDetailConfig<T extends { id: string }> = {
  title: string;
  emptyState?: string;
  metrics?: CrudExplorerMetricConfig<T>[];
  filters?: CrudExplorerFilterConfig<T>[];
  renderDetail: (row: T, ctx: { items: T[]; reload: () => Promise<void> }) => import('react').ReactNode;
};

export type CrudValueFilterOption<T extends { id: string }> = {
  value: string;
  label: string;
  matches: (row: T) => boolean;
};

export type CrudStateMachineStateConfig<Status extends string = string, ColumnId extends string = string> = {
  value: Status;
  label: string;
  columnId: ColumnId;
  badgeVariant?: 'default' | 'info' | 'warning' | 'success' | 'danger';
  terminal?: boolean;
};

export type CrudStateMachineColumnConfig<Status extends string = string, ColumnId extends string = string> = {
  id: ColumnId;
  label: string;
  defaultState: Status;
};

export type CrudStateMachineTransitionConfig<Status extends string = string> = {
  from: Status;
  to: Status[];
};

export type CrudStateMachineConfig<
  T extends { id: string },
  Status extends string = string,
  ColumnId extends string = string,
> = {
  field: keyof T & string;
  states: CrudStateMachineStateConfig<Status, ColumnId>[];
  columns: CrudStateMachineColumnConfig<Status, ColumnId>[];
  transitions?: CrudStateMachineTransitionConfig<Status>[];
};

export type CrudKanbanConfig<T extends { id: string }> = {
  /**
   * Contenido textual de la card del tablero. Si no se define, el runtime usa un fallback genérico.
   */
  card?: {
    title: (row: T) => string;
    subtitle?: (row: T) => string;
    meta?: (row: T) => string;
  };
  /**
   * CTA reusable al pie de cada columna para crear desde el tablero.
   */
  createFooterLabel?: string;
  /**
   * Persistencia dedicada del valor movido en el kanban.
   * Permite que el tablero actualice solo el campo de estado/valor sin reconstruir
   * un PUT completo del recurso desde una fila parcial del listado.
   */
  persistMove?: (args: {
    row: T;
    field: keyof T & string;
    nextValue: string;
  }) => Promise<T>;
};

export type CrudEditorModalSectionConfig = {
  id: string;
  title?: ReactNode;
  description?: ReactNode;
  fieldKeys?: string[];
};

export type CrudEditorModalBlockConfig<T extends { id: string }> = {
  id: string;
  kind: 'lineItems';
  field: string;
  sectionId: string;
  label?: ReactNode;
  required?: boolean;
  visible?: (ctx: {
    values: ModulesCrudFormValues;
    editing: boolean;
    row?: T;
  }) => boolean;
};

export type CrudEditorModalStatConfig<T extends { id: string }> = {
  id: string;
  label: ReactNode;
  value: (ctx: {
    row?: T;
    values: ModulesCrudFormValues;
    editing: boolean;
  }) => ReactNode;
  tone?: 'default' | 'info' | 'warning' | 'success' | 'danger';
};

export type CrudEditorModalFieldConfig = {
  sectionId?: string;
  helperText?: ReactNode;
  fullWidth?: boolean;
  hidden?: boolean;
  readOnly?: boolean;
  editControl?: (ctx: {
    value: ModulesCrudFieldValue | undefined;
    values: ModulesCrudFormValues;
    setValue: (nextValue: ModulesCrudFieldValue) => void;
  }) => ReactNode;
  visible?: (ctx: {
    value: ModulesCrudFieldValue | undefined;
    values: ModulesCrudFormValues;
    editing: boolean;
  }) => boolean;
  readValue?: (ctx: {
    value: ModulesCrudFieldValue | undefined;
    values: ModulesCrudFormValues;
  }) => ReactNode;
};

export type CrudEditorModalConfig<T extends { id: string }> = {
  eyebrow?: ReactNode;
  loadRecord?: (row: T) => Promise<T>;
  canEdit?: (row: T) => boolean;
  mediaFieldKey?: string;
  disableBuiltInMedia?: boolean;
  blocks?: CrudEditorModalBlockConfig<T>[];
  sections?: CrudEditorModalSectionConfig[];
  fieldConfig?: Record<string, CrudEditorModalFieldConfig>;
  stats?: CrudEditorModalStatConfig<T>[];
  confirmDiscard?: {
    title: string;
    description: string;
    confirmLabel?: string;
    cancelLabel?: string;
  };
};

export type {
  CrudColumn,
  CrudDataSource,
  CrudFieldValue,
  CrudFormField,
  CrudFormValues,
  CrudHelpers,
  CrudHttpClient,
  CrudListHeaderSlotContext,
  CrudRowAction,
  CrudToolbarAction,
} from '@devpablocristo/modules-crud-ui';

export type CrudPageConfig<T extends { id: string }> = Omit<ModulesCrudPageConfig<T>, 'featureFlags'> & {
  featureFlags?: CrudFeatureFlags;
  viewModes?: CrudViewModeConfig[];
  explorerDetail?: CrudExplorerDetailConfig<T>;
  /** Extensión Pymes: render de celda tags cuando el módulo CRUD lo soporta vía CSV/flags. */
  renderTagsCell?: (row: T) => import('react').ReactNode;
  /** Máquina de estados canónica del recurso. */
  stateMachine?: CrudStateMachineConfig<T>;
  /** Configuración reusable del kanban genérico del recurso. */
  kanban?: CrudKanbanConfig<T>;
  /** Configuración declarativa del modal base de create/edit. */
  editorModal?: CrudEditorModalConfig<T>;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- mapa heterogéneo: cada config tiene su propio tipo de record, TS no soporta tipos existenciales
export type CrudResourceConfigMap = Record<string, CrudPageConfig<any>>;

/**
 * CRUD de consola Pymes: motor en `@devpablocristo/modules-crud-ui`, textos vía i18n y API vía `apiRequest`.
 */
export function CrudPage<T extends { id: string }>(props: CrudPageConfig<T>) {
  const { localizeText, sentenceCase, language } = useI18n();
  const stringsBase = useMemo(() => buildPymesCrudStrings(language), [language]);

  const httpClient = useMemo(
    () =>
      props.basePath
        ? {
            json: <R,>(path: string, init?: { method?: string; body?: Record<string, unknown> }): Promise<R> =>
              apiRequest<R>(path, {
                method: init?.method as 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | undefined,
                body: init?.body,
              }),
          }
        : undefined,
    [props.basePath],
  );

  return (
    <div className="crud-page-host">
      <ModulesCrudPage
        {...props}
        stringsBase={stringsBase}
        formatFieldText={localizeText}
        sentenceCase={sentenceCase}
        httpClient={props.httpClient ?? httpClient}
      />
    </div>
  );
}
