import type { CrudColumn, CrudFormField } from '@devpablocristo/modules-crud-ui';
import type { CrudPageConfig } from '../components/CrudPage';
import { formatPartyTagList, parsePartyTagCsv } from '../modules/parties';
import { asBoolean } from './resourceConfigs.shared';

/** Recursos donde las anotaciones estándar no aplican (listados técnicos o backend incompleto). */
const STANDARD_ANNOTATIONS_OPT_OUT = new Set(['audit', 'attachments', 'timeline', 'webhooks']);

function readFavoriteFromRecord(row: Record<string, unknown>): boolean {
  const meta = row.metadata;
  if (!meta || typeof meta !== 'object') return false;
  const m = meta as Record<string, unknown>;
  return (
    m.favorite === true || String(m.favorite ?? '').toLowerCase() === 'true' || m.favorite === 1 || m.favorite === '1'
  );
}

function mergeEditorModalForAnnotations<T extends { id: string }>(
  editorModal: CrudPageConfig<T>['editorModal'],
  injectTags: boolean,
  injectFavorite: boolean,
): CrudPageConfig<T>['editorModal'] {
  if (!editorModal) return undefined;
  if (!injectTags && !injectFavorite) return editorModal;

  const fieldConfigExtra: Record<string, { helperText?: string; fullWidth?: boolean }> = {
    ...(injectTags ? { tags: { helperText: 'Etiquetas libres para agrupar y filtrar.' } } : {}),
    ...(injectFavorite
      ? {
          metadata_favorite: {
            helperText: 'Marcá registros destacados cuando la vista muestre el indicador.',
          },
        }
      : {}),
  };

  return {
    ...editorModal,
    fieldConfig: {
      ...(editorModal.fieldConfig ?? {}),
      ...fieldConfigExtra,
    },
  };
}

/**
 * Inyecta campos `tags` + `metadata_favorite` y fusiona toBody/search/columnas cuando el recurso no los define ya.
 */
export function applyStandardCrudAnnotations<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CrudPageConfig<T> {
  if (config.featureFlags?.standardAnnotations === false || STANDARD_ANNOTATIONS_OPT_OUT.has(resourceId)) {
    return config;
  }

  const fields = config.formFields ?? [];
  const hasTags = fields.some((f) => f.key === 'tags');
  const hasFavorite = fields.some((f) => f.key === 'metadata_favorite');
  if (hasTags && hasFavorite) {
    return config;
  }

  const injectTags = !hasTags;
  const injectFavorite = !hasFavorite;

  const extraFields: CrudFormField[] = [
    ...(injectFavorite ? [{ key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' as const }] : []),
    ...(injectTags
      ? [{ key: 'tags', label: 'Etiquetas Internas', placeholder: 'coma, separadas' }]
      : []),
  ];

  const tagColumn: CrudColumn<T> = {
    key: 'tags' as keyof T & string,
    header: 'Etiquetas Internas',
    className: 'cell-tags',
    render: (_value, row) => {
      const raw = (row as Record<string, unknown>).tags;
      if (!Array.isArray(raw) || raw.length === 0) return '—';
      return raw.map(String).join(', ');
    },
  };

  const prevSearchText = config.searchText;
  const prevToFormValues = config.toFormValues;
  const prevToBody = config.toBody;

  return {
    ...config,
    formFields: [...extraFields, ...fields],
    columns:
      injectTags && config.columns ? ([...config.columns, tagColumn] as CrudPageConfig<T>['columns']) : config.columns,
    editorModal: mergeEditorModalForAnnotations(config.editorModal, injectTags, injectFavorite),
    searchText: (row) => {
      const base = prevSearchText(row);
      if (!injectTags) return base;
      const tagStr = formatPartyTagList((row as Record<string, unknown>).tags as string[] | undefined);
      return [base, tagStr].filter(Boolean).join(' ');
    },
    toFormValues: (row) => ({
      ...prevToFormValues(row),
      ...(injectTags
        ? { tags: formatPartyTagList((row as Record<string, unknown>).tags as string[] | undefined) }
        : {}),
      ...(injectFavorite ? { metadata_favorite: readFavoriteFromRecord(row as Record<string, unknown>) } : {}),
    }),
    toBody: (values) => {
      const base = prevToBody ? prevToBody(values) : {};
      let body: Record<string, unknown> = { ...base };
      if (injectTags) {
        body = { ...body, tags: parsePartyTagCsv(values.tags) };
      }
      if (injectFavorite) {
        const prevMeta =
          typeof body.metadata === 'object' && body.metadata !== null
            ? ({ ...(body.metadata as Record<string, unknown>) } as Record<string, unknown>)
            : {};
        if (asBoolean(values.metadata_favorite)) {
          prevMeta.favorite = true;
        } else {
          delete prevMeta.favorite;
        }
        body = { ...body, metadata: prevMeta };
      }
      return body;
    },
  };
}
