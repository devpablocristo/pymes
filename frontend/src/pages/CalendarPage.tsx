import { QueueOperatorBoard, SchedulingCalendar, createSchedulingClient } from '@devpablocristo/modules-scheduling/next';
import '@devpablocristo/modules-scheduling/styles.next.css';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';

const schedulingClient = createSchedulingClient(apiRequest);

/** Agenda interna: calendario operativo y cola para usuarios de la org. El flujo para clientes finales está en `/web-clientes`. */
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
      <QueueOperatorBoard
        client={schedulingClient}
        locale={language === 'en' ? 'en' : 'es'}
        searchQuery={search}
      />
    </PageLayout>
  );
}

export default CalendarPage;
