import { createElement } from 'react';
import type { CrudEditorModalFieldConfig } from '../components/CrudPage';
import { StandardCrudImageUrlsEditor } from './standardCrudMedia';

export function buildStandardCrudImageUrlsModalFieldConfig(
  overrides?: Partial<CrudEditorModalFieldConfig>,
): CrudEditorModalFieldConfig {
  return {
    fullWidth: true,
    editControl: ({ value, setValue }) => createElement(StandardCrudImageUrlsEditor, { value, setValue }),
    ...overrides,
  };
}
