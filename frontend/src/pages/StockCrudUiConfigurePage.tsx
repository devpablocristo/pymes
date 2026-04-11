import { CrudUiPreferencesPanel } from '@devpablocristo/modules-crud-ui';
import { PageLayout } from '../components/PageLayout';
import { loadLazyCrudPageConfig } from '../crud/lazyCrudPage';
import { CRUD_UI_CHANGE_EVENT, CRUD_UI_RESOURCES, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';
import './StockCrudUiConfigurePage.css';

const STOCK_CRUD_UI_RESOURCES = CRUD_UI_RESOURCES.filter((r) => r.resourceId === 'stock');

/** Preferencias de vistas y plantilla solo para el módulo Inventario (no vive en Ajustes). */
export function StockCrudUiConfigurePage() {
  return (
    <PageLayout
      title="Vistas del inventario"
      lead="Solo afecta a lista, galería y tablero de inventario en este navegador."
    >
      <CrudUiPreferencesPanel
        storageKey={CRUD_UI_STORAGE_KEY}
        resources={STOCK_CRUD_UI_RESOURCES}
        changeEventName={CRUD_UI_CHANGE_EVENT}
        loadPageConfig={loadLazyCrudPageConfig}
        hideResourceCardHeader
        copy={{
          defaultViewLabel: 'Vista por defecto',
        }}
        classes={{
          section: 'admin-settings-section stock-crud-prefs',
          hint: 'admin-settings-hint',
          stack: 'stg__gateway-stack stock-crud-prefs__stack',
          grid: 'stock-crud-prefs__grid',
          checkboxRow: 'admin-checkbox-row stock-crud-prefs__row',
        }}
      />
    </PageLayout>
  );
}
