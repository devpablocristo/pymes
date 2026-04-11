import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import { StockLevelDetailModal } from '../modules/stock';

/** Lista administrativa; «Ver detalle» abre modal (ajuste, mínimo, movimientos, archivar). */
export function StockListPage() {
  const queryClient = useQueryClient();
  const [detailProductId, setDetailProductId] = useState<string | null>(null);
  const [listKey, setListKey] = useState(0);

  const bumpList = useCallback(() => {
    setListKey((k) => k + 1);
    void queryClient.invalidateQueries({ queryKey: ['stock'] });
  }, [queryClient]);

  return (
    <>
      <LazyConfiguredCrudPage
        key={listKey}
        resourceId="stock"
        mergeConfig={{
          onRowClick: (row: { id: string }) => setDetailProductId(row.id),
        }}
      />
      <StockLevelDetailModal productId={detailProductId} onClose={() => setDetailProductId(null)} onAfterSave={bumpList} />
    </>
  );
}
