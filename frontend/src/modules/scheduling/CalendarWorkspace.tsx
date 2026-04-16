import { SchedulingCalendar, createSchedulingClient } from '@devpablocristo/modules-scheduling/next';
import '@devpablocristo/modules-scheduling/styles.next.css';
import { PageLayout } from '../../components/PageLayout';
import { usePageSearch } from '../../components/PageSearch';
import { apiRequest } from '../../lib/api';
import { useBranchSelection } from '../../lib/branchContext';
import { useI18n } from '../../lib/i18n';

const schedulingClient = createSchedulingClient(apiRequest);

export function CalendarWorkspace() {
  const { t, language } = useI18n();
  const search = usePageSearch();
  const { selectedBranchId } = useBranchSelection();

  return (
    <PageLayout className="calendar-page" title={t('calendar.pageTitle')} lead={t('calendar.pageLead')}>
      <SchedulingCalendar
        key={`scheduling-calendar:${selectedBranchId ?? 'default'}`}
        client={schedulingClient}
        locale={language === 'en' ? 'en' : 'es'}
        searchQuery={search}
        initialBranchId={selectedBranchId ?? undefined}
      />
    </PageLayout>
  );
}
