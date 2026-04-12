import type { CrudPageConfig } from '../../components/CrudPage';

/**
 * Contrato estándar de modos para un CRUD reutilizable:
 * el recurso declara explícitamente lista, galería y tablero.
 * La lista puede tener render propio; galería y tablero usan el fallback genérico del runtime.
 */
export function buildStandardCrudViewModes<T extends { id: string }>(
  renderList: NonNullable<CrudPageConfig<T>['viewModes']>[number]['render'],
): NonNullable<CrudPageConfig<T>['viewModes']> {
  return [
    { id: 'list', label: 'Lista', path: 'list', isDefault: true, render: renderList },
    { id: 'gallery', label: 'Galería', path: 'gallery' },
    { id: 'kanban', label: 'Tablero', path: 'board' },
  ];
}
