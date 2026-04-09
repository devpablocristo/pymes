import { SchedulingCalendar, createSchedulingClient } from '@devpablocristo/modules-scheduling/next';
import '@devpablocristo/modules-scheduling/styles.next.css';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';

const schedulingClient = createSchedulingClient(apiRequest);

/**
 * Agenda interna: vista calendario tipo Google Calendar para los usuarios de
 * la org (turnos cliente + eventos internos + bloqueos). El flujo público
 * para clientes finales se sirve desde su URL real (`/v1/public/:org_id/...`),
 * no embebido en consola. Esta página queda 100% calendario, sin widgets de
 * operación de cola ni stats embebidas.
 */
export function CalendarPage() {
  const { t, language } = useI18n();
  const search = usePageSearch();

  return (
    <PageLayout className="calendar-page" title={t('calendar.pageTitle')} lead={t('calendar.pageLead')}>
      <SchedulingCalendar
        client={schedulingClient}
        locale={language === 'en' ? 'en' : 'es'}
        searchQuery={search}
      />
    </PageLayout>
  );
}

export default CalendarPage;
