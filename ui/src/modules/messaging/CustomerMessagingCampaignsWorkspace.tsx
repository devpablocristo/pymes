import { useMemo, useState } from 'react';
import { confirmAction } from '@devpablocristo/core-browser';
import { useSearch } from '@devpablocristo/modules-search';
import { DataTable, type DataTableColumn } from '@devpablocristo/modules-ui-data-display';
import { PageLayout } from '../../components/PageLayout';
import { usePageSearch } from '../../components/PageSearch';
import { formatDate } from '../../crud/resourceConfigs.shared';
import { formatFetchErrorForUser } from '../../lib/formatFetchError';
import type { CustomerMessagingCampaign } from '../../lib/api';
import {
  buildMessagingCampaignsSummary,
  renderMessagingCampaignStatusBadge,
} from './messagingHelpers';
import {
  initialMessagingCampaignDraft,
  useCustomerMessagingCampaigns,
} from './useCustomerMessagingCampaigns';

export function CustomerMessagingCampaignsWorkspace() {
  const search = usePageSearch();
  const [draft, setDraft] = useState(initialMessagingCampaignDraft);
  const { campaignsQuery, createMutation, sendMutation } = useCustomerMessagingCampaigns(draft, () =>
    setDraft(initialMessagingCampaignDraft),
  );

  const campaigns = campaignsQuery.data?.items ?? [];
  const filtered = useSearch(
    campaigns,
    (row) => [row.name, row.template_name, row.tag_filter, row.status, row.created_by].filter(Boolean).join(' '),
    search,
  );
  const error = campaignsQuery.error
    ? formatFetchErrorForUser(campaignsQuery.error, 'No se pudo cargar la lista de campañas.')
    : createMutation.error
      ? formatFetchErrorForUser(createMutation.error, 'No se pudo crear la campaña.')
      : sendMutation.error
        ? formatFetchErrorForUser(sendMutation.error, 'No se pudo enviar la campaña.')
        : '';

  const columns = useMemo<DataTableColumn<CustomerMessagingCampaign>[]>(
    () => [
      {
        key: 'name',
        header: 'Campaña',
        render: (_value, row) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">
              {row.template_name} · {row.tag_filter || 'Todos'}
            </div>
          </>
        ),
      },
      {
        key: 'status',
        header: 'Estado',
        render: (value) => renderMessagingCampaignStatusBadge(String(value ?? 'draft')),
      },
      { key: 'total_recipients', header: 'Destinatarios' },
      {
        key: 'sent_count',
        header: 'Resultado',
        render: (_value, row) => `${row.sent_count}/${row.total_recipients} (${row.failed_count} fallos)`,
      },
      {
        key: 'created_at',
        header: 'Creada',
        render: (value) => formatDate(String(value ?? '')),
      },
      {
        key: 'id',
        header: 'Acción',
        render: (_value, row) =>
          row.status === 'draft' || row.status === 'scheduled' ? (
            <button
              type="button"
              className="btn-primary btn-sm"
              disabled={sendMutation.isPending}
              onClick={async () => {
                const confirmed = await confirmAction({
                  title: 'Enviar campaña',
                  description: `¿Enviar campaña "${row.name}" a ${row.total_recipients} destinatarios?`,
                  confirmLabel: 'Enviar',
                  cancelLabel: 'Cancelar',
                  tone: 'danger',
                });
                if (!confirmed) return;
                await sendMutation.mutateAsync(row.id);
              }}
            >
              Enviar
            </button>
          ) : (
            '—'
          ),
      },
    ],
    [sendMutation],
  );

  const summary = buildMessagingCampaignsSummary(campaigns, filtered.length);
  const isValid = draft.name.trim().length >= 2 && draft.template_name.trim().length >= 2;

  return (
    <PageLayout
      title="Campañas de Mensajería"
      lead="Templates salientes, segmentación por tag y envío masivo sobre contactos con consentimiento."
      actions={
        <button
          type="button"
          className="btn-secondary btn-sm"
          onClick={() => void campaignsQuery.refetch()}
          disabled={campaignsQuery.isFetching}
        >
          Recargar
        </button>
      }
    >
      {error ? <div className="alert alert-error">{error}</div> : null}

      <section className="crud-form-card">
        <form
          className="crud-form"
          onSubmit={(event) => {
            event.preventDefault();
            if (!isValid) return;
            void createMutation.mutateAsync();
          }}
        >
          <div className="crud-form-grid">
            <label>
              <span>Nombre</span>
              <input
                className="input"
                value={draft.name}
                onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
                placeholder="Promo Mendoza Marzo"
              />
            </label>
            <label>
              <span>Template</span>
              <input
                className="input"
                value={draft.template_name}
                onChange={(event) => setDraft((current) => ({ ...current, template_name: event.target.value }))}
                placeholder="promo_marzo_2026"
              />
            </label>
            <label>
              <span>Idioma</span>
              <input
                className="input"
                value={draft.template_language}
                onChange={(event) => setDraft((current) => ({ ...current, template_language: event.target.value }))}
                placeholder="es"
              />
            </label>
            <label>
              <span>Tag filtro</span>
              <input
                className="input"
                value={draft.tag_filter}
                onChange={(event) => setDraft((current) => ({ ...current, tag_filter: event.target.value }))}
                placeholder="mendoza"
              />
            </label>
            <label className="full-width">
              <span>Parámetros</span>
              <input
                className="input"
                value={draft.template_params}
                onChange={(event) => setDraft((current) => ({ ...current, template_params: event.target.value }))}
                placeholder="valor1, valor2"
              />
            </label>
          </div>
          <div className="m-notification-feed__actions">
            <button type="submit" className="btn-primary" disabled={!isValid || createMutation.isPending}>
              Crear campaña
            </button>
          </div>
        </form>
      </section>

      {campaignsQuery.isLoading ? (
        <div className="empty-state">
          <p>Cargando campañas…</p>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty-state">
          <p>No hay campañas para mostrar.</p>
        </div>
      ) : (
        <DataTable
          data={filtered}
          columns={columns}
          message="No hay campañas para mostrar."
          headerComponent={<div className="card__toolbar">{summary}</div>}
        />
      )}
    </PageLayout>
  );
}
