import type { CrudPageConfig } from '../../components/CrudPage';

type BuildStandardCrudViewModesOptions<T extends { id: string }> = {
  defaultModeId?: 'list' | 'gallery' | 'kanban';
  renderGallery?: NonNullable<CrudPageConfig<T>['viewModes']>[number]['render'];
  renderKanban?: NonNullable<CrudPageConfig<T>['viewModes']>[number]['render'];
  ariaLabel?: string;
};

/**
 * Contrato estándar de modos para un CRUD reutilizable:
 * el recurso declara explícitamente lista, galería y tablero.
 * La lista puede tener render propio; galería y tablero usan el fallback genérico del runtime.
 */
export function buildStandardCrudViewModes<T extends { id: string }>(
  renderList: NonNullable<CrudPageConfig<T>['viewModes']>[number]['render'],
  options: BuildStandardCrudViewModesOptions<T> = {},
): NonNullable<CrudPageConfig<T>['viewModes']> {
  const defaultModeId = options.defaultModeId ?? 'list';
  const ariaLabel = options.ariaLabel;
  return [
    { id: 'list', label: 'Lista', path: 'list', isDefault: defaultModeId === 'list', render: renderList, ariaLabel },
    { id: 'gallery', label: 'Galería', path: 'gallery', isDefault: defaultModeId === 'gallery', render: options.renderGallery, ariaLabel },
    { id: 'kanban', label: 'Tablero', path: 'board', isDefault: defaultModeId === 'kanban', render: options.renderKanban, ariaLabel },
  ];
}

export function buildStandardListGalleryViewModes<T extends { id: string }>(
  renderList: NonNullable<CrudPageConfig<T>['viewModes']>[number]['render'],
): NonNullable<CrudPageConfig<T>['viewModes']> {
  return buildStandardCrudViewModes(renderList);
}
