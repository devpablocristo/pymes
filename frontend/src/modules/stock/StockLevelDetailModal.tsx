import { useMemo } from 'react';
import { CrudResourceInventoryDetailModal } from '../crud/CrudResourceInventoryDetailModal';
import {
  buildStockInventoryDetailPorts,
  formatStockInventoryDateTime,
  formatStockInventoryMovementKind,
  stockInventoryDetailModalStringsEs,
} from './stockInventoryDetailModalAdapter';

export type StockLevelDetailModalProps = {
  productId: string | null;
  onClose: () => void;
  onAfterSave?: () => void;
};

/**
 * Detalle de nivel de inventario para el vertical actual: delega en el shell agnóstico
 * `CrudResourceInventoryDetailModal` + adaptador HTTP (`stockInventoryDetailModalAdapter`).
 */
export function StockLevelDetailModal({ productId, onClose, onAfterSave }: StockLevelDetailModalProps) {
  const strings = useMemo(() => stockInventoryDetailModalStringsEs, []);
  const ports = useMemo(() => buildStockInventoryDetailPorts(), []);

  return (
    <CrudResourceInventoryDetailModal
      linkedEntityId={productId}
      onClose={onClose}
      onAfterSave={onAfterSave}
      strings={strings}
      ports={ports}
      formatMovementKind={formatStockInventoryMovementKind}
      formatDateTime={formatStockInventoryDateTime}
      advancedSettingsHref="/modules/products/list"
    />
  );
}
