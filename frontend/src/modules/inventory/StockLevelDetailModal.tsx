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
  strings?: CrudResourceInventoryDetailStrings;
  catalogHref?: string | null;
  inventoryHandlers?: Partial<StockInventoryDetailHandlers>;
  permissions?: CrudResourceInventoryDetailPermissions;
  onArchive?: (linkedEntityId: string) => Promise<void>;
  onCancelEdit?: () => void;
};

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
