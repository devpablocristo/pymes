import { useQuery, useQueryClient } from "@tanstack/react-query";
import { DateTime } from "luxon";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useSchedulingGateway } from "../api/GatewayContext";
import { errorMessage } from "../api/errors";
import { useSession } from "../auth/AuthContext";
import { AvailabilityPanel } from "../components/AvailabilityPanel";
import { BlockDialog } from "../components/BlockDialog";
import { BookingDetails } from "../components/BookingDetails";
import { BookingDialog, type NewBookingDraft } from "../components/BookingDialog";
import { CalendarBoard } from "../components/CalendarBoard";
import { CancellationDialog } from "../components/CancellationDialog";
import { DayRail } from "../components/DayRail";
import { QueuePanel } from "../components/QueuePanel";
import { Toast, type ToastMessage } from "../components/Toast";
import { WaitlistPanel } from "../components/WaitlistPanel";
import type {
  Allocation,
  AvailabilityBlockInput,
  Booking,
  BookingAction,
  BookingInput,
  DateRange,
  QueueTicket,
  WaitlistInput,
} from "../domain/scheduling";

type Tab = "agenda" | "availability" | "waitlist" | "queue";

const initialRange = {
  from: DateTime.now().minus({ days: 7 }).startOf("day").toUTC().toISO()!,
  until: DateTime.now().plus({ days: 45 }).endOf("day").toUTC().toISO()!,
};

function NavIcon({ name }: { name: "calendar" | "clock" | "waitlist" | "queue" }) {
  const paths = {
    calendar: <path d="M4 7h16M7 3v4m10-4v4M5 5h14a1 1 0 0 1 1 1v13H4V6a1 1 0 0 1 1-1Zm3 6h3v3H8v-3Z" />,
    clock: <path d="M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16Zm0 4v5l3 2" />,
    waitlist: <path d="M8 7h12M8 12h12M8 17h7M4 7h.01M4 12h.01M4 17h.01" />,
    queue: <path d="M5 6h14v4H5V6Zm0 8h9v4H5v-4Zm12 0h2v4h-2v-4Z" />,
  };
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      {paths[name]}
    </svg>
  );
}

export function AdminSchedulingPage({ publicSlug }: { publicSlug: string }) {
  const gateway = useSchedulingGateway();
  const { identity, organizationName, accountControls } = useSession();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>("agenda");
  const [branchId, setBranchId] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [resourceId, setResourceId] = useState("");
  const [range, setRange] = useState<DateRange>(initialRange);
  const [selectedBookingId, setSelectedBookingId] = useState<string | null>(null);
  const [bookingDraft, setBookingDraft] = useState<NewBookingDraft | null>(null);
  const [reschedulingBooking, setReschedulingBooking] = useState<Booking | null>(null);
  const [cancellingBooking, setCancellingBooking] = useState<Booking | null>(null);
  const [blockOpen, setBlockOpen] = useState(false);
  const [pending, setPending] = useState<string | null>(null);
  const [toast, setToast] = useState<ToastMessage | null>(null);

  const branchesQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "branches"],
    queryFn: () => gateway.listBranches(identity),
  });
  const servicesQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "services"],
    queryFn: () => gateway.listServices(identity),
  });

  useEffect(() => {
    if (!branchId && branchesQuery.data?.[0]) {
      setBranchId(branchesQuery.data[0].id);
    }
  }, [branchId, branchesQuery.data]);

  const resourcesQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "resources", branchId],
    queryFn: () => gateway.listResources(identity, branchId),
    enabled: Boolean(branchId),
  });
  const bookingsQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "bookings", branchId, range.from, range.until],
    queryFn: () => gateway.listBookings(identity, branchId, range),
    enabled: Boolean(branchId),
  });
  const blocksQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "blocks", branchId, range.from, range.until],
    queryFn: () => gateway.listBlocks(identity, branchId, range),
    enabled: Boolean(branchId),
  });
  const rulesQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "rules", branchId],
    queryFn: () => gateway.listAvailabilityRules(identity, branchId),
    enabled: Boolean(branchId),
  });
  const waitlistQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "waitlist", branchId],
    queryFn: () => gateway.listWaitlist(identity, branchId),
    enabled: Boolean(branchId),
  });
  const queueQuery = useQuery({
    queryKey: ["scheduling", identity.organizationId, "queue", branchId],
    queryFn: () => gateway.listQueue(identity, branchId),
    enabled: Boolean(branchId),
  });

  const branches = branchesQuery.data ?? [];
  const services = servicesQuery.data ?? [];
  const resources = resourcesQuery.data ?? [];
  const bookings = bookingsQuery.data ?? [];
  const currentBranch = branches.find((branch) => branch.id === branchId) ?? null;
  const timezone = currentBranch?.timezone ?? "America/Argentina/Buenos_Aires";
  const filteredBookings = useMemo(
    () =>
      bookings.filter(
        (booking) =>
          (!serviceId || booking.service_id === serviceId) &&
          (!resourceId || booking.allocations.some((allocation) => allocation.resource_id === resourceId)),
      ),
    [bookings, resourceId, serviceId],
  );
  const selectedBooking =
    bookings.find((booking) => booking.id === selectedBookingId) ??
    (reschedulingBooking?.id === selectedBookingId ? reschedulingBooking : null);

  const notify = useCallback((tone: ToastMessage["tone"], text: string) => {
    setToast({ id: crypto.randomUUID(), tone, text });
  }, []);

  const invalidateAgenda = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ["scheduling", identity.organizationId] });
  }, [identity.organizationId, queryClient]);

  async function createBooking(input: BookingInput) {
    setPending("booking");
    try {
      const created = await gateway.createBooking(identity, input);
      setBookingDraft(null);
      setSelectedBookingId(null);
      await invalidateAgenda();
      notify("success", created.length > 1 ? `Se crearon ${created.length} turnos.` : "Turno creado.");
    } catch (error) {
      notify("error", errorMessage(error));
      throw error;
    } finally {
      setPending(null);
    }
  }

  async function rescheduleBooking(
    booking: Booking,
    startAt: string,
    endAt: string,
    durationMinutes: number,
    allocations: Allocation[],
  ) {
    setPending(`booking:${booking.id}`);
    try {
      const replacement = await gateway.rescheduleBooking(
        identity,
        booking.id,
        booking.version,
        startAt,
        durationMinutes,
        allocations,
      );
      setReschedulingBooking(null);
      setSelectedBookingId(replacement.id);
      await invalidateAgenda();
      notify(
        "success",
        `Reprogramado para ${DateTime.fromISO(startAt).setZone(timezone).toFormat("dd LLL · HH:mm", {
          locale: "es",
        })}.`,
      );
    } catch (error) {
      await invalidateAgenda();
      notify("error", errorMessage(error));
      throw error;
    } finally {
      setPending(null);
    }
    void endAt;
  }

  async function calendarReschedule(booking: Booking, startAt: string, endAt: string) {
    const duration = Math.max(1, Math.round(DateTime.fromISO(endAt).diff(DateTime.fromISO(startAt), "minutes").minutes));
    await rescheduleBooking(booking, startAt, endAt, duration, booking.allocations);
  }

  async function transitionBooking(booking: Booking, action: BookingAction, reason?: string) {
    if (action === "cancel" && reason === undefined) {
      setCancellingBooking(booking);
      return;
    }
    setPending(`booking:${booking.id}`);
    try {
      const updated = await gateway.transitionBooking(identity, booking.id, action, booking.version, reason);
      setSelectedBookingId(updated.id);
      setCancellingBooking(null);
      await invalidateAgenda();
      notify("success", action === "cancel" ? "Turno cancelado." : "Estado actualizado.");
    } catch (error) {
      await invalidateAgenda();
      notify("error", errorMessage(error));
    } finally {
      setPending(null);
    }
  }

  async function createBlock(input: AvailabilityBlockInput) {
    setPending("block");
    try {
      await gateway.createBlock(identity, input);
      setBlockOpen(false);
      await invalidateAgenda();
      notify("success", "Bloqueo creado.");
    } catch (error) {
      notify("error", errorMessage(error));
      throw error;
    } finally {
      setPending(null);
    }
  }

  async function createWaitlist(input: WaitlistInput) {
    setPending("waitlist");
    try {
      await gateway.createWaitlistEntry(identity, input);
      await invalidateAgenda();
      notify("success", "Solicitud agregada a la lista de espera.");
    } catch (error) {
      notify("error", errorMessage(error));
    } finally {
      setPending(null);
    }
  }

  async function advanceQueue(
    ticket: QueueTicket,
    status: "called" | "serving" | "completed" | "no_show" | "cancelled",
  ) {
    setPending(`queue:${ticket.id}`);
    try {
      await gateway.advanceQueueTicket(identity, ticket.id, {
        expected_version: ticket.version,
        status,
      });
      await invalidateAgenda();
    } catch (error) {
      notify("error", errorMessage(error));
    } finally {
      setPending(null);
    }
  }

  function updateRange(next: DateRange) {
    setRange((current) => (current.from === next.from && current.until === next.until ? current : next));
  }

  if (branchesQuery.isError || servicesQuery.isError) {
    return (
      <main className="fatal-state" id="main-content">
        <p className="eyebrow">Agenda no disponible</p>
        <h1>No pudimos cargar la configuración</h1>
        <p>{errorMessage(branchesQuery.error ?? servicesQuery.error)}</p>
        <button type="button" className="button button--primary" onClick={() => window.location.reload()}>
          Reintentar
        </button>
      </main>
    );
  }

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <a className="brand" href="/app/agenda" aria-label="Pymes">
          <span className="brand__mark">P</span>
          <span>
            <strong>Pymes</strong>
            <small>Operación</small>
          </span>
        </a>
        <nav aria-label="Agenda">
          <button type="button" className={tab === "agenda" ? "active" : ""} onClick={() => setTab("agenda")}>
            <NavIcon name="calendar" />
            Agenda
          </button>
          <button
            type="button"
            className={tab === "availability" ? "active" : ""}
            onClick={() => setTab("availability")}
          >
            <NavIcon name="clock" />
            Disponibilidad
          </button>
          <button type="button" className={tab === "waitlist" ? "active" : ""} onClick={() => setTab("waitlist")}>
            <NavIcon name="waitlist" />
            Lista de espera
          </button>
          <button type="button" className={tab === "queue" ? "active" : ""} onClick={() => setTab("queue")}>
            <NavIcon name="queue" />
            Cola
          </button>
        </nav>
        <div className="app-sidebar__public">
          <span>Booking público</span>
          <a href={`/reservar/${publicSlug}`} target="_blank" rel="noreferrer">
            Ver página ↗
          </a>
        </div>
      </aside>

      <main className="app-main" id="main-content">
        <header className="app-header">
          <div>
            <p className="eyebrow">{organizationName}</p>
            <h1>
              {tab === "agenda"
                ? "Agenda"
                : tab === "availability"
                  ? "Disponibilidad"
                  : tab === "waitlist"
                    ? "Lista de espera"
                    : "Cola de atención"}
            </h1>
          </div>
          <div className="app-header__actions">
            {tab === "agenda" ? (
              <button
                type="button"
                className="button button--primary"
                onClick={() => {
                  const start = DateTime.now().setZone(timezone).plus({ days: 1 }).set({ hour: 9, minute: 0 });
                  setBookingDraft({
                    startAt: start.toUTC().toISO()!,
                    endAt: start.plus({ minutes: 30 }).toUTC().toISO()!,
                  });
                }}
              >
                + Nuevo turno
              </button>
            ) : null}
            {accountControls}
          </div>
        </header>

        <section className="filter-bar" aria-label="Filtros de agenda">
          <label>
            <span>Sucursal</span>
            <select
              value={branchId}
              onChange={(event) => {
                setBranchId(event.target.value);
                setResourceId("");
              }}
            >
              {branches.map((branch) => (
                <option key={branch.id} value={branch.id}>
                  {branch.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Servicio</span>
            <select value={serviceId} onChange={(event) => setServiceId(event.target.value)}>
              <option value="">Todos los servicios</option>
              {services.map((service) => (
                <option key={service.id} value={service.id}>
                  {service.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Profesional o recurso</span>
            <select value={resourceId} onChange={(event) => setResourceId(event.target.value)}>
              <option value="">Todos los recursos</option>
              {resources.map((resource) => (
                <option key={resource.id} value={resource.id}>
                  {resource.name}
                </option>
              ))}
            </select>
          </label>
          <div className="timezone-indicator">
            <span className="timezone-indicator__pulse" />
            {timezone}
          </div>
        </section>

        {tab === "agenda" ? (
          <div className="agenda-layout">
            <div className="agenda-layout__calendar">
              <CalendarBoard
                bookings={filteredBookings}
                blocks={blocksQuery.data ?? []}
                loaded={!bookingsQuery.isLoading}
                timezone={timezone}
                selectedBookingId={selectedBookingId}
                onRangeChange={updateRange}
                onCreateAt={(startAt, endAt) => setBookingDraft({ startAt, endAt })}
                onSelectBooking={(booking) => setSelectedBookingId(booking.id)}
                onReschedule={calendarReschedule}
                onMutationError={(error) => notify("error", errorMessage(error))}
              />
            </div>
            <DayRail bookings={bookings} timezone={timezone} onSelect={(booking) => setSelectedBookingId(booking.id)} />
            <BookingDetails
              booking={selectedBooking}
              resources={resources}
              timezone={timezone}
              pending={pending?.startsWith("booking:") ?? false}
              onClose={() => setSelectedBookingId(null)}
              onReschedule={setReschedulingBooking}
              onAction={transitionBooking}
            />
          </div>
        ) : null}

        {tab === "availability" ? (
          <AvailabilityPanel
            rules={rulesQuery.data ?? []}
            blocks={blocksQuery.data ?? []}
            resources={resources}
            timezone={timezone}
            onCreateBlock={() => setBlockOpen(true)}
          />
        ) : null}

        {tab === "waitlist" ? (
          <WaitlistPanel
            entries={waitlistQuery.data ?? []}
            branchId={branchId}
            services={services}
            timezone={timezone}
            pending={pending === "waitlist"}
            onCreate={createWaitlist}
          />
        ) : null}

        {tab === "queue" ? (
          <QueuePanel
            tickets={queueQuery.data ?? []}
            services={services}
            pendingTicketId={pending?.startsWith("queue:") ? pending.slice(6) : null}
            onAdvance={(ticket, status) => void advanceQueue(ticket, status)}
          />
        ) : null}
      </main>

      <BookingDialog
        open={Boolean(bookingDraft || reschedulingBooking)}
        draft={bookingDraft}
        booking={reschedulingBooking}
        branchId={branchId}
        timezone={timezone}
        branches={branches}
        services={services}
        resources={resources}
        preferredServiceId={serviceId}
        preferredResourceId={resourceId}
        pending={pending === "booking" || Boolean(pending?.startsWith("booking:"))}
        onClose={() => {
          setBookingDraft(null);
          setReschedulingBooking(null);
        }}
        onCreate={createBooking}
        onReschedule={rescheduleBooking}
      />
      <BlockDialog
        open={blockOpen}
        branch={currentBranch}
        resources={resources}
        pending={pending === "block"}
        onClose={() => setBlockOpen(false)}
        onSave={createBlock}
      />
      <CancellationDialog
        booking={cancellingBooking}
        pending={pending?.startsWith("booking:") ?? false}
        onClose={() => setCancellingBooking(null)}
        onConfirm={async (reason) => {
          if (cancellingBooking) await transitionBooking(cancellingBooking, "cancel", reason);
        }}
      />
      <Toast message={toast} onDismiss={() => setToast(null)} />
    </div>
  );
}
