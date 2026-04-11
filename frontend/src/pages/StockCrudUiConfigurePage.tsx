import { CrudUiPreferencesPanel } from '@devpablocristo/modules-crud-ui';
import { Link } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { loadLazyCrudPageConfig } from '../crud/lazyCrudPage';
import { CRUD_UI_RESOURCES, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';

const STOCK_CRUD_UI_RESOURCES = CRUD_UI_RESOURCES.filter((r) => r.resourceId === 'stock');

/** Preferencias de vistas y plantilla solo para el módulo Inventario (no vive en Ajustes). */
export function StockCrudUiConfigurePage() {
  return (
    <PageLayout
      title="Vistas del inventario"
      lead="Solo afecta a lista, galería y tablero de inventario en este navegador."
    >
      <p className="u-mb-3">
        <Link to="/modules/stock/list">← Volver al inventario</Link>
      </p>
      <CrudUiPreferencesPanel
        storageKey={CRUD_UI_STORAGE_KEY}
        resources={STOCK_CRUD_UI_RESOURCES}
        changeEventName="pymes:crud-ui-config-changed"
        loadPageConfig={loadLazyCrudPageConfig}
        hideResourceCardHeader
        copy={{
          defaultViewLabel: 'Vista por defecto',
        }}
        classes={{
          section: 'admin-settings-section',
          hint: 'admin-settings-hint',
          stack: 'stg__gateway-stack',
          grid: 'admin-settings-grid',
          checkboxRow: 'admin-checkbox-row',
        }}
      />
    </PageLayout>
  );
}
