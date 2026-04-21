import { asString } from '../../crud/resourceConfigs.shared';
import type { CrudFieldValue, CrudFormField } from '../../components/CrudPage';

export const INTERNAL_NOTES_FIELD_KEY = 'notes';
export const INTERNAL_TAGS_FIELD_KEY = 'tags';
export const INTERNAL_FAVORITE_FIELD_KEY = 'is_favorite';

export const INTERNAL_NOTES_LABEL = 'Notas internas';
export const INTERNAL_TAGS_LABEL = 'Etiquetas internas';
export const INTERNAL_FAVORITE_LABEL = 'Agregar a favoritos';

export const buildInternalNotesField = (): CrudFormField => ({
  key: INTERNAL_NOTES_FIELD_KEY,
  label: INTERNAL_NOTES_LABEL,
  type: 'textarea',
  fullWidth: true,
});

export const buildInternalTagsField = (placeholder: string): CrudFormField => ({
  key: INTERNAL_TAGS_FIELD_KEY,
  label: INTERNAL_TAGS_LABEL,
  placeholder,
});

export const buildInternalFavoriteField = (): CrudFormField => ({
  key: INTERNAL_FAVORITE_FIELD_KEY,
  label: INTERNAL_FAVORITE_LABEL,
  type: 'checkbox',
});

export function buildStandardInternalFields(options: {
  tagsPlaceholder: string;
  includeFavorite?: boolean;
  includeNotes?: boolean;
}): CrudFormField[] {
  const fields: CrudFormField[] = [];
  if (options.includeFavorite !== false) {
    fields.push(buildInternalFavoriteField());
  }
  fields.push(buildInternalTagsField(options.tagsPlaceholder));
  if (options.includeNotes !== false) {
    fields.push(buildInternalNotesField());
  }
  return fields;
}

export function parseTagCsv(value: CrudFieldValue | undefined): string[] {
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatTagCsv(tags?: string[]): string {
  return (tags ?? []).join(', ');
}
