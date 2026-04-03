import { QueueOperatorBoard, SchedulingCalendar, createSchedulingClient } from '@devpablocristo/modules-scheduling';
import '@devpablocristo/modules-scheduling/styles.css';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';

const schedulingClient = createSchedulingClient(apiRequest);

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
