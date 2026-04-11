import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ConfiguredCrudModePage } from '../crud/configuredCrudViews';

export function ProductsListPage() {
  const [searchParams] = useSearchParams();
  const mergeConfig = useMemo(() => {
    const raw = searchParams.get('archived')?.trim().toLowerCase();
    if (raw === '1' || raw === 'true' || raw === 'yes') {
      return { initialShowArchived: true };
    }
    return undefined;
  }, [searchParams]);

  return <ConfiguredCrudModePage resourceId="products" modeId="list" mergeConfig={mergeConfig} />;
}
