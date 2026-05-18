import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  formatSchedulingDateTime,
  resolveSchedulingCopyLocale,
  schedulingDaySummaryCopyPresets,
  type DashboardStats,
  type DayAgendaItem,
  type SchedulingClient,
} from '@devpablocristo/modules-scheduling';
import { useBranchSelection } from '../../lib/useBranchSelection';

const summaryKeys = {
  dashboard: (branchId: string | null, date: string) => ['scheduling-summary', 'dashboard', branchId ?? 'all', date] as const,
  day: (branchId: string | null, date: string) => ['scheduling-summary', 'day', branchId ?? 'all', date] as const,
};

export type BranchSchedulingDaySummaryProps = {
  client: SchedulingClient;
  locale?: string;
  className?: string;
  initialDate?: string;
};

function todayValue(): string {
  return new Date().toISOString().slice(0, 10);
}

function isUpcomingBooking(item: DayAgendaItem): boolean {
  if (item.type !== 'booking' || !item.start_at) {
    return false;
  }
  return new Date(item.start_at).getTime() >= Date.now();
}

function isQueueItem(item: DayAgendaItem): boolean {
  return item.type === 'queue_ticket';
}

export function BranchSchedulingDaySummary({
  client,
  locale = 'en',
  className = '',
  initialDate,
}: BranchSchedulingDaySummaryProps) {
  const copy = schedulingDaySummaryCopyPresets[resolveSchedulingCopyLocale(locale)];
  const statusLabel = (status: string) => copy.statuses[status as keyof typeof copy.statuses] ?? status;
  const [selectedDate, setSelectedDate] = useState(initialDate ?? todayValue());
  const { availableBranches, selectedBranch, selectedBranchId, isLoading: branchLoading } = useBranchSelection();

  const dashboardQuery = useQuery<DashboardStats>({
    queryKey: summaryKeys.dashboard(selectedBranchId, selectedDate),
    queryFn: () => client.getDashboard(selectedBranchId, selectedDate),
    enabled: !branchLoading,
    staleTime: 20_000,
  });

  const dayQuery = useQuery<DayAgendaItem[]>({
    queryKey: summaryKeys.day(selectedBranchId, selectedDate),
    queryFn: () => client.getDayAgenda(selectedBranchId, selectedDate),
    enabled: !branchLoading,
    staleTime: 20_000,
  });

  const nextBooking = useMemo(
    () => (dayQuery.data ?? []).filter(isUpcomingBooking).sort((a, b) => (a.start_at ?? '').localeCompare(b.start_at ?? ''))[0],
    [dayQuery.data],
  );
  const queueFlow = useMemo(() => (dayQuery.data ?? []).filter(isQueueItem).slice(0, 6), [dayQuery.data]);
  const branchCaption = availableBranches.length > 1 ? selectedBranch?.name?.trim() || null : null;

  return (
    <div className={`card modules-scheduling__day-summary ${className}`.trim()}>
      <div className="card-header">
        <div>
          <h2>{copy.title}</h2>
          <p className="text-secondary">
            {copy.description}
            {branchCaption ? ` ${branchCaption}.` : ''}
          </p>
        </div>
        <div className="form-group">
          <label htmlFor="branch-scheduling-day-summary-date">{copy.dateLabel}</label>
          <input
            id="branch-scheduling-day-summary-date"
            type="date"
            value={selectedDate}
            onChange={(event) => setSelectedDate(event.target.value)}
          />
        </div>
      </div>

      {branchLoading || dashboardQuery.isLoading || dayQuery.isLoading ? (
        <div className="modules-scheduling__empty">
          <div className="spinner" />
          <p>{copy.loading}</p>
        </div>
      ) : !dashboardQuery.data ? (
        <div className="modules-scheduling__empty">{copy.empty}</div>
      ) : (
        <>
          <div className="modules-scheduling__summary stats-grid">
            <article className="stat-card">
              <div className="stat-label">{copy.bookings}</div>
              <div className="stat-value">{dashboardQuery.data.bookings_today}</div>
            </article>
            <article className="stat-card">
              <div className="stat-label">{copy.confirmed}</div>
              <div className="stat-value">{dashboardQuery.data.confirmed_bookings_today}</div>
            </article>
            <article className="stat-card">
              <div className="stat-label">{copy.activeQueues}</div>
              <div className="stat-value">{dashboardQuery.data.active_queues}</div>
            </article>
            <article className="stat-card">
              <div className="stat-label">{copy.waiting}</div>
              <div className="stat-value">{dashboardQuery.data.waiting_tickets}</div>
            </article>
          </div>

          <div className="modules-scheduling__day-summary-grid">
            <section className="modules-scheduling__day-summary-section">
              <div className="modules-scheduling__day-summary-title">{copy.nextBooking}</div>
              {nextBooking ? (
                <div className="modules-scheduling__day-summary-item">
                  <strong>{nextBooking.label}</strong>
                  <span>{formatSchedulingDateTime(nextBooking.start_at, locale)}</span>
                  <span>{statusLabel(nextBooking.status)}</span>
                </div>
              ) : (
                <div className="modules-scheduling__queue-empty">{copy.noUpcoming}</div>
              )}
            </section>
            <section className="modules-scheduling__day-summary-section">
              <div className="modules-scheduling__day-summary-title">{copy.queueFlow}</div>
              {queueFlow.length === 0 ? (
                <div className="modules-scheduling__queue-empty">{copy.noQueueFlow}</div>
              ) : (
                queueFlow.map((item) => (
                  <div key={item.id} className="modules-scheduling__day-summary-item">
                    <strong>{item.label}</strong>
                    <span>{statusLabel(item.status)}</span>
                  </div>
                ))
              )}
            </section>
          </div>
        </>
      )}
    </div>
  );
}
