import { QueueOperatorBoard, SchedulingCalendar, createSchedulingClient } from '../../../../modules/scheduling/ts/src/next';
import '../../../../modules/scheduling/ts/src/styles.next.css';
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
