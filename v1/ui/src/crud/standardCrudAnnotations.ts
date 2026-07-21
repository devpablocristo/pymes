import type { CrudColumn, CrudFormField } from '@devpablocristo/platform-crud-ui';
import type { CrudEditorModalFieldConfig, CrudPageConfig } from '../components/CrudPage';
import { extractCrudRecordImageUrls, formatCrudRecordImageUrlsToForm } from '../modules/crud/crudLinkedEntityImageUrls';
import { formatPartyTagList, parsePartyTagCsv } from '../modules/parties';
import { asBoolean, parseImageURLList } from './resourceConfigs.shared';
import { buildStandardCrudImageUrlsModalFieldConfig } from './standardCrudMediaFieldConfig';

/** Solo etiquetas + favorito: listados técnicos. El carrusel/imágenes usa `standardMedia: false` por recurso. */
const STANDARD_TAGS_FAVORITE_OPT_OUT = new Set(['audit', 'attachments', 'timeline', 'webhooks']);

function readFavoriteFromRecord(row: Record<string, unknown>): boolean {
  const meta = row.metadata;
  if (!meta || typeof meta !== 'object') return false;
  const m = meta as Record<string, unknown>;
  return (
    m.favorite === true || String(m.favorite ?? '').toLowerCase() === 'true' || m.favorite === 1 || m.favorite === '1'
  );
}

/**
 * Orden canónico del bloque estándar del modal CRUD (coincide con el layout de rejilla 2 columnas):
 * fila 1 — Favorito | Etiquetas internas; fila 2 — Imágenes a ancho completo; después el resto en el orden original.
 */
export function normalizeStandardCrudFormFieldOrder(fields: CrudFormField[]): CrudFormField[] {
  const byKey = new Map(fields.map((f) => [f.key, f]));
  const used = new Set<string>();
  const head: CrudFormField[] = [];

  const takeFirst = (keys: readonly string[]) => {
    for (const k of keys) {
      const f = byKey.get(k);
      if (f) {
        head.push(enhanceStandardHeadField(f));
        used.add(k);
        return;
      }
    }
  };

  takeFirst(['metadata_favorite', 'is_favorite']);
  takeFirst(['tags']);
  takeFirst(['image_urls', 'images', 'image_url']);

  const tail = fields.filter((f) => !used.has(f.key));
  return [...head, ...tail];
}

function enhanceStandardHeadField(field: CrudFormField): CrudFormField {
  if (field.key === 'image_urls' || field.key === 'images' || field.key === 'image_url') {
    return field.fullWidth === true ? field : { ...field, fullWidth: true };
  }
  return field;
}

function formFieldsOrderOrShapeChanged(before: CrudFormField[], after: CrudFormField[]): boolean {
  if (before.length !== after.length) return true;
  return before.some((field, i) => field !== after[i]);
}

/** Campos de imagen ya declarados en el formulario (no hace falta inyectar textarea). */
function pickExistingImageFormFieldKey(fields: CrudFormField[]): string | undefined {
  const keys = new Set(fields.map((f) => f.key));
  for (const id of ['image_urls', 'images', 'image_url'] as const) {
    if (keys.has(id)) return id;
  }
  return undefined;
}

function mergeEditorModalStandardFields<T extends { id: string }>(
  editorModal: CrudPageConfig<T>['editorModal'],
  injectImages: boolean,
  mergedFormFields: CrudFormField[],
): CrudPageConfig<T>['editorModal'] {
  const existingImageKey = pickExistingImageFormFieldKey(mergedFormFields);
  const base = editorModal ?? {};
  const mediaFieldKey = base.mediaFieldKey ?? existingImageKey ?? (injectImages ? 'image_urls' : undefined);

  if (!injectImages) {
    return mediaFieldKey ? { ...base, mediaFieldKey } : editorModal;
  }

  const fieldConfigExtra: Record<string, CrudEditorModalFieldConfig> = {
    image_urls: buildStandardCrudImageUrlsModalFieldConfig(),
  };

  return {
    ...base,
    mediaFieldKey,
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
  const skipTagsFavorite =
    config.featureFlags?.standardAnnotations === false || STANDARD_TAGS_FAVORITE_OPT_OUT.has(resourceId);

  const fields = config.formFields ?? [];
  const hasTags = fields.some((f) => f.key === 'tags');
  /** `is_favorite` (API first-class) cuenta como favorito estándar — no duplicar con `metadata_favorite`. */
  const hasFavorite = fields.some((f) => f.key === 'metadata_favorite' || f.key === 'is_favorite');
  const hasDeclarativeImageField = fields.some(
    (f) => f.key === 'image_urls' || f.key === 'images' || f.key === 'image_url',
  );

  const injectTags = !skipTagsFavorite && !hasTags;
  const injectFavorite = !skipTagsFavorite && !hasFavorite;
  const injectImages =
    !hasDeclarativeImageField && config.featureFlags?.standardMedia !== false;

  const existingImageFormKey = pickExistingImageFormFieldKey(fields);
  const wireModalMediaField =
    config.featureFlags?.standardMedia !== false &&
    Boolean(existingImageFormKey) &&
    !injectImages &&
    !config.editorModal?.mediaFieldKey;

  if (!injectTags && !injectFavorite && !injectImages && !wireModalMediaField) {
    const reordered = normalizeStandardCrudFormFieldOrder(fields);
    if (formFieldsOrderOrShapeChanged(fields, reordered)) {
      return { ...config, formFields: reordered };
    }
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

  const mergedFormFields = normalizeStandardCrudFormFieldOrder([...extraFields, ...fields]);

  if (!injectTags && !injectFavorite && !injectImages && wireModalMediaField) {
    return {
      ...config,
      formFields: mergedFormFields,
      editorModal: mergeEditorModalStandardFields(config.editorModal, false, mergedFormFields),
    };
  }

  return {
    ...config,
    formFields: mergedFormFields,
    columns:
      injectTags && config.columns ? ([...config.columns, tagColumn] as CrudPageConfig<T>['columns']) : config.columns,
    editorModal: mergeEditorModalStandardFields(config.editorModal, injectImages, mergedFormFields),
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
