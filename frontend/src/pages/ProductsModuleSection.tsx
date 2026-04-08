import { Outlet } from 'react-router-dom';
import { WorkOrdersHeaderLead } from '../components/WorkOrdersHeaderLead';
import { useI18n } from '../lib/i18n';
import './WorkOrdersModuleSection.css';

const GALLERY_PATH = '/modules/products/gallery';
const LIST_PATH = '/modules/products/list';

/**
 * Mismo patrón que órdenes de trabajo: selector arriba + Outlet (galería / lista).
 */
export function ProductsModuleSection() {
  const { t } = useI18n();
  return (
    <div className="wo-mod-orders">
      <WorkOrdersHeaderLead
        boardPath={GALLERY_PATH}
        listPath={LIST_PATH}
        leftLabel={t('crud.viewMode.gallery')}
        rightLabel={t('crud.viewMode.table')}
        groupAriaLabel={t('crud.viewMode.aria')}
      />
      <Outlet />
    </div>
  );
}
