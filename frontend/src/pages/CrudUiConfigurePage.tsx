import { useMemo } from 'react';
import { NavLink, useParams } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { loadLazyCrudPageConfig } from '../crud/lazyCrudPage';
import { CrudUiPreferencesPanel } from '../modules/crud';
import { CRUD_UI_CHANGE_EVENT, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';
import { crudModuleCatalog } from '../crud/crudModuleCatalog';
import './CrudUiConfigurePage.css';

const FEATURE_KEYS = [
  ['searchBar', 'Buscador'],
  ['creatorFilter', 'Filtro de responsable'],
  ['archivedToggle', 'Ver archivados'],
  ['createAction', 'Acción crear'],
  ['csvToolbar', 'Acciones CSV'],
  ['pagination', 'Paginación'],
] as const;

export function CrudUiConfigurePage() {
  const { moduleId = '' } = useParams();
  const moduleDefinition = crudModuleCatalog[moduleId];
  const title = moduleDefinition?.title ?? moduleId;
  const resources = useMemo(() => [{ resourceId: moduleId, label: title }], [moduleId, title]);

  return (
    <div className="crud-configure-page">
      <div className="crud-configure-page__back">
        <NavLink className="wo-mod-orders__action" to={`/modules/${moduleId}`}>
          Volver a {title.toLowerCase()}
        </NavLink>
      </div>
    <PageLayout
      title={`Vistas de ${title.toLowerCase()}`}
      lead={`Solo afecta a ${title.toLowerCase()} en este navegador.`}
    >
      <CrudUiPreferencesPanel
        storageKey={CRUD_UI_STORAGE_KEY}
        resources={resources}
        changeEventName={CRUD_UI_CHANGE_EVENT}
        loadPageConfig={loadLazyCrudPageConfig}
        copy={{}}
        hideResourceCardHeader
        hideDefaultViewSelector
        featureKeys={FEATURE_KEYS}
        classes={{
          section: 'admin-settings-section stock-crud-prefs',
          hint: 'admin-settings-hint',
          stack: 'stg__gateway-stack stock-crud-prefs__stack',
          grid: 'stock-crud-prefs__grid',
          checkboxRow: 'admin-checkbox-row stock-crud-prefs__row',
        }}
      />
    </PageLayout>
    </div>
  );
}

export default CrudUiConfigurePage;
