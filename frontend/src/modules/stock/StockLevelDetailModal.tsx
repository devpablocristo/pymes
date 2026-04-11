import { useMemo } from 'react';
import {
  CrudResourceInventoryDetailModal,
  type CrudResourceInventoryDetailPermissions,
  type CrudResourceInventoryDetailStrings,
} from '../crud';
import {
  buildStockInventoryDetailPorts,
  formatStockInventoryDateTime,
  formatStockInventoryMovementKind,
  stockInventoryDetailModalStringsEs,
  type StockInventoryDetailHandlers,
} from './stockInventoryDetailModalAdapter';

const defaultCatalogHref = '/modules/products/list';

export type StockLevelDetailModalProps = {
  productId: string | null;
  onClose: () => void;
  onAfterSave?: () => void;
  /** Textos del detalle; por defecto `stockInventoryDetailModalStringsEs` (ES). */
  strings?: CrudResourceInventoryDetailStrings;
  /**
   * Enlace al catálogo u otra vista avanzada.
   * - `undefined`: se usa la ruta por defecto del vertical (`/modules/products/list`).
   * - `null`: no se muestra el enlace (aunque `strings.linkToAdvancedSettings` exista).
   * - `string`: URL/path personalizado.
   */
  catalogHref?: string | null;
  /**
   * Handlers HTTP del vertical (`fetchLevel`, `fetchMovements`, `patchEntity`, `postAdjust`, …).
   * Si no se pasa, se usan los defaults del adaptador (`defaultFetch*` / `defaultPost*`).
   */
  inventoryHandlers?: Partial<StockInventoryDetailHandlers>;
  /** Permisos del shell genérico (p. ej. RBAC); si no se pasan, el modal usa su comportamiento por defecto. */
  permissions?: CrudResourceInventoryDetailPermissions;
  /** Si se define, sustituye al archivado del adaptador por defecto (misma firma que el shell CRUD). */
  onArchive?: (linkedEntityId: string) => Promise<void>;
  onCancelEdit?: () => void;
};

/**
 * Wrapper fino del vertical inventario: `productId`, strings (ES por defecto),
 * handlers opcionales (`fetchLevel`, `fetchEntity`, `fetchMovements`, `patchEntity`, `postAdjust`, `archiveEntity`)
 * y enlace opcional al catálogo (`catalogHref`).
 */
export function StockLevelDetailModal({
  productId,
  onClose,
  onAfterSave,
  strings: stringsProp,
  catalogHref,
  inventoryHandlers,
  permissions,
  onArchive,
  onCancelEdit,
}: StockLevelDetailModalProps) {
  const strings = useMemo(() => stringsProp ?? stockInventoryDetailModalStringsEs, [stringsProp]);
  const ports = useMemo(() => buildStockInventoryDetailPorts(inventoryHandlers), [inventoryHandlers]);
  const advancedSettingsHref = catalogHref === null ? undefined : (catalogHref ?? defaultCatalogHref);

  return (
    <CrudResourceInventoryDetailModal
      linkedEntityId={productId}
      onClose={onClose}
      onAfterSave={onAfterSave}
      strings={strings}
      ports={ports}
      formatMovementKind={formatStockInventoryMovementKind}
      formatDateTime={formatStockInventoryDateTime}
      advancedSettingsHref={advancedSettingsHref}
      permissions={permissions}
      onArchive={onArchive}
      onCancelEdit={onCancelEdit}
    />
  );
}
