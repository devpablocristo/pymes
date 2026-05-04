import type { CrudColumn, CrudFormField } from '@devpablocristo/modules-crud-ui';
import type { CrudEditorModalFieldConfig, CrudPageConfig } from '../components/CrudPage';
import { extractCrudRecordImageUrls, formatCrudRecordImageUrlsToForm } from '../modules/crud/crudLinkedEntityImageUrls';
import { formatPartyTagList, parsePartyTagCsv } from '../modules/parties';
import { asBoolean, parseImageURLList } from './resourceConfigs.shared';
import { buildStandardCrudImageUrlsModalFieldConfig } from './standardCrudMedia';

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

function mergeEditorModalStandardFields<T extends { id: string }>(
  editorModal: CrudPageConfig<T>['editorModal'],
  injectImages: boolean,
): CrudPageConfig<T>['editorModal'] {
  if (!injectImages) {
    return editorModal;
  }

  const base = editorModal ?? {};
  const fieldConfigExtra: Record<string, CrudEditorModalFieldConfig> = {
    image_urls: buildStandardCrudImageUrlsModalFieldConfig(),
  };

  return {
    ...base,
    mediaFieldKey: base.mediaFieldKey ?? (injectImages ? 'image_urls' : undefined),
    fieldConfig: {
      ...(base.fieldConfig ?? {}),
      ...fieldConfigExtra,
    },
  };
}

/**
 * Inyecta campos estándar (`tags`, `metadata_favorite`, `image_urls`) y fusiona modal/search/toBody cuando el recurso no los define ya.
 *
 * Las imágenes se guardan en **`metadata.image_urls`**. Los productos mantienen también `image_urls` en el JSON del API para la columna del catálogo; es el mismo conjunto de URLs.
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
  const hasImageUrls = fields.some((f) => f.key === 'image_urls');

  const injectTags = !hasTags;
  const injectFavorite = !hasFavorite;
  const injectImages =
    !hasImageUrls && config.featureFlags?.standardMedia !== false;

  if (!injectTags && !injectFavorite && !injectImages) {
    return config;
  }

  const extraFields: CrudFormField[] = [
    ...(injectFavorite ? [{ key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' as const }] : []),
    ...(injectTags
      ? [{ key: 'tags', label: 'Etiquetas Internas', placeholder: 'coma, separadas' }]
      : []),
    ...(injectImages
      ? [
          {
            key: 'image_urls',
            label: 'Imágenes',
            type: 'textarea' as const,
            fullWidth: true,
            placeholder: 'Subí archivos con el selector de abajo o pegá URLs / data URLs, una por línea.',
          },
        ]
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
    editorModal: mergeEditorModalStandardFields(config.editorModal, injectImages),
    searchText: (row) => {
      const rec = row as Record<string, unknown>;
      const parts: string[] = [];
      parts.push(prevSearchText(row));
      if (injectTags) {
        parts.push(formatPartyTagList(rec.tags as string[] | undefined));
      }
      if (injectImages) {
        parts.push(extractCrudRecordImageUrls(rec).join(' '));
      }
      return parts.filter(Boolean).join(' ');
    },
    toFormValues: (row) => ({
      ...prevToFormValues(row),
      ...(injectTags
        ? { tags: formatPartyTagList((row as Record<string, unknown>).tags as string[] | undefined) }
        : {}),
      ...(injectFavorite ? { metadata_favorite: readFavoriteFromRecord(row as Record<string, unknown>) } : {}),
      ...(injectImages ? { image_urls: formatCrudRecordImageUrlsToForm(row as Record<string, unknown>) } : {}),
    }),
    toBody: (values) => {
      const base = prevToBody ? prevToBody(values) : {};
      let body: Record<string, unknown> = { ...base };
      if (injectTags) {
        body = { ...body, tags: parsePartyTagCsv(values.tags) };
      }
      if (injectFavorite || injectImages) {
        const prevMeta =
          typeof body.metadata === 'object' && body.metadata !== null
            ? ({ ...(body.metadata as Record<string, unknown>) } as Record<string, unknown>)
            : {};
        if (injectFavorite) {
          if (asBoolean(values.metadata_favorite)) {
            prevMeta.favorite = true;
          } else {
            delete prevMeta.favorite;
          }
        }
        if (injectImages) {
          const urls = parseImageURLList(values.image_urls);
          if (urls.length > 0) {
            prevMeta.image_urls = urls;
          } else {
            delete prevMeta.image_urls;
          }
        }
        body = { ...body, metadata: prevMeta };
      }
      return body;
    },
  };
}
