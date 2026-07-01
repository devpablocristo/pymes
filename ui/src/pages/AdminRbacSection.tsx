import { useMemo, useState } from 'react';
import { CrudPage, type CrudHttpClient } from '../components/CrudPage';
import {
  apiRequest,
  createOrg,
  createOrgUser,
  listOrgUsers,
  listOrgs,
  listTenantMembers,
  removeTenantMember,
  updateOrgUser,
  type OrgMemberRow,
  type OrgSummary,
  type OrgUser,
} from '../lib/api';

type IAMTab = 'orgs' | 'users' | 'members';
type CrudLifecycleView = 'active' | 'archived';

type MemberRow = OrgMemberRow & {
  member_name: string;
  member_email: string;
  status: string;
};

type PymesIAMCrudPath = {
  resource: IAMTab;
  view: CrudLifecycleView;
  id: string;
  action: 'archive' | 'unarchive' | 'trash' | 'restore' | '';
  limit: number;
  cursor: number;
};

export function AdminRbacSection({ tenantId }: { tenantId: string }) {
  const [tab, setTab] = useState<IAMTab>('users');
  const httpClient = useMemo(() => createPymesIAMHttpClient(tenantId), [tenantId]);

  return (
    <section className="card admin-settings-section">
      <div className="card-header">
        <h2>IAM</h2>
      </div>

      <nav className="screen-nav" aria-label="IAM screens">
        {[
          ['orgs', 'Organizaciones'],
          ['users', 'Usuarios'],
          ['members', 'Miembros'],
        ].map(([id, label]) => (
          <button key={id} type="button" className={tab === id ? 'active' : ''} onClick={() => setTab(id as IAMTab)}>
            {label}
          </button>
        ))}
      </nav>

      {tab === 'orgs' && (
        <CrudPage<OrgSummary>
          key="pymes-iam-orgs"
          basePath="/v1/iam-crud/orgs"
          httpClient={httpClient}
          allowCreate
          allowEdit={false}
          allowDelete={false}
          allowRestore={false}
          allowHardDelete={false}
          supportsArchived={false}
          label="organización"
          labelPlural="organizaciones"
          labelPluralCap="Organizaciones"
          createLabel="Nueva organización"
          columns={[
            { key: 'name', header: 'Nombre' },
            { key: 'slug', header: 'Slug' },
            { key: 'role', header: 'Rol' },
          ]}
          formFields={[
            { key: 'name', label: 'Nombre', required: true },
            { key: 'slug', label: 'Slug' },
          ]}
          searchText={(row) => [row.name, row.slug, row.id].join(' ')}
          toFormValues={(row) => ({ name: row.name, slug: row.slug ?? '' })}
          toBody={(values) => ({ name: stringValue(values.name), slug: stringValue(values.slug) })}
          isValid={(values) => stringValue(values.name).length > 0}
          emptyState="Sin organizaciones"
          searchPlaceholder="Buscar organizaciones"
          featureFlags={{ csvToolbar: false }}
        />
      )}

      {tab === 'users' && (
        <CrudPage<OrgUser>
          key="pymes-iam-users"
          basePath="/v1/iam-crud/users"
          httpClient={httpClient}
          supportsArchived
          allowCreate
          allowEdit
          allowDelete
          allowRestore
          allowHardDelete={false}
          label="usuario"
          labelPlural="usuarios"
          labelPluralCap="Usuarios"
          createLabel="Nuevo usuario"
          columns={[
            { key: 'name', header: 'Nombre' },
            { key: 'email', header: 'Email' },
            { key: 'status', header: 'Estado', render: (value) => formatStatus(String(value ?? '')) },
          ]}
          formFields={[
            { key: 'email', label: 'Email', type: 'email', required: true },
            { key: 'name', label: 'Nombre' },
          ]}
          searchText={(row) => [row.name, row.email, row.external_id, row.id].join(' ')}
          toFormValues={(row) => ({ email: row.email, name: row.name })}
          toBody={(values) => ({ email: stringValue(values.email), name: stringValue(values.name) })}
          isValid={(values) => stringValue(values.email).length > 0}
          emptyState="Sin usuarios"
          archivedEmptyState="Sin usuarios archivados"
          searchPlaceholder="Buscar usuarios"
          featureFlags={{ csvToolbar: false }}
        />
      )}

      {tab === 'members' && (
        <CrudPage<MemberRow>
          key={`pymes-iam-members-${tenantId}`}
          basePath="/v1/iam-crud/members"
          httpClient={httpClient}
          supportsArchived={false}
          allowCreate={false}
          allowEdit
          allowDelete
          allowRestore={false}
          allowHardDelete={false}
          label="miembro"
          labelPlural="miembros"
          labelPluralCap="Miembros"
          columns={[
            { key: 'member_name', header: 'Miembro' },
            { key: 'member_email', header: 'Email' },
            { key: 'role', header: 'Rol' },
            { key: 'status', header: 'Estado', render: (value) => formatStatus(String(value ?? '')) },
          ]}
          formFields={[
            {
              key: 'role',
              label: 'Rol',
              type: 'select',
              required: true,
              options: [
                { label: 'owner', value: 'owner' },
                { label: 'admin', value: 'admin' },
                { label: 'member', value: 'member' },
              ],
            },
          ]}
          searchText={(row) => [row.member_name, row.member_email, row.role, row.user_id].join(' ')}
          toFormValues={(row) => ({ role: row.role ?? 'member' })}
          toBody={(values) => ({ role: stringValue(values.role) })}
          isValid={(values) => stringValue(values.role).length > 0}
          emptyState="Sin miembros"
          searchPlaceholder="Buscar miembros"
          featureFlags={{ csvToolbar: false }}
        />
      )}
    </section>
  );
}

function createPymesIAMHttpClient(tenantId: string): CrudHttpClient {
  return {
    json: async <TResponse,>(path: string, init: { method?: string; body?: Record<string, unknown> } = {}) => {
      const parsed = parsePymesIAMCrudPath(path);
      const method = init.method ?? 'GET';
      const body = init.body ?? {};

      if (method === 'GET') {
        return listPymesIAMRows(parsed, tenantId) as Promise<TResponse>;
      }
      return mutatePymesIAMRow(parsed, tenantId, method, body) as Promise<TResponse>;
    },
  };
}

async function listPymesIAMRows(parsed: PymesIAMCrudPath, tenantId: string): Promise<{ items: Array<OrgSummary | OrgUser | MemberRow>; has_more: boolean; next_cursor: string }> {
  const allRows = await loadPymesIAMRows(parsed.resource, tenantId);
  const rows = allRows.filter((row) => lifecycleBucket('status' in row ? String(row.status ?? 'active') : 'active') === parsed.view);
  const page = rows.slice(parsed.cursor, parsed.cursor + parsed.limit);
  const nextCursor = parsed.cursor + parsed.limit;
  return {
    items: page,
    has_more: nextCursor < rows.length,
    next_cursor: nextCursor < rows.length ? String(nextCursor) : '',
  };
}

async function loadPymesIAMRows(resource: IAMTab, tenantId: string): Promise<Array<OrgSummary | OrgUser | MemberRow>> {
  if (resource === 'orgs') {
    const response = await listOrgs();
    return response.items;
  }
  if (resource === 'users') {
    const response = await listOrgUsers();
    return response.items;
  }
  const response = await listTenantMembers(tenantId);
  return response.items.map(toMemberRow);
}

async function mutatePymesIAMRow(parsed: PymesIAMCrudPath, tenantId: string, method: string, body: Record<string, unknown>): Promise<unknown> {
  if (parsed.resource === 'orgs') {
    if (!parsed.id) {
      return createOrg({ name: stringValue(body.name), slug: stringValue(body.slug) });
    }
    return { ok: true };
  }

  if (parsed.resource === 'users') {
    if (!parsed.id) {
      return createOrgUser({ email: stringValue(body.email), name: stringValue(body.name) });
    }
    const status = method === 'DELETE' ? 'archived' : statusForAction(parsed.action, 'archived');
    return updateOrgUser(parsed.id, status ? { status } : { email: stringValue(body.email), name: stringValue(body.name) });
  }

  if (method === 'DELETE') {
    await removeTenantMember(tenantId, parsed.id);
    return { ok: true };
  }
  return apiRequest(`/v1/orgs/${encodeURIComponent(tenantId)}/members/${encodeURIComponent(parsed.id)}`, {
    method: 'PATCH',
    body: { role: stringValue(body.role) },
  });
}

function parsePymesIAMCrudPath(path: string): PymesIAMCrudPath {
  const url = new URL(path, window.location.origin);
  const segments = url.pathname.split('/').filter(Boolean);
  const rootIndex = segments.indexOf('iam-crud');
  const resource = segments[rootIndex + 1] as IAMTab;
  const rest = segments.slice(rootIndex + 2);
  const view = rest[0] === 'archived' ? 'archived' : 'active';
  const id = view === 'active' ? rest[0] ?? '' : '';
  const action = view === 'active' ? rest[1] ?? '' : '';
  const rawLimit = Number.parseInt(url.searchParams.get('limit') || '100', 10);
  const rawCursor = Number.parseInt(url.searchParams.get('cursor') || '0', 10);
  return {
    resource,
    view,
    id: decodeURIComponent(id),
    action: action as PymesIAMCrudPath['action'],
    limit: Number.isFinite(rawLimit) && rawLimit > 0 ? rawLimit : 100,
    cursor: Number.isFinite(rawCursor) && rawCursor > 0 ? rawCursor : 0,
  };
}

function toMemberRow(member: OrgMemberRow): MemberRow {
  const name = member.user?.name?.trim() || [member.user?.given_name, member.user?.family_name].filter(Boolean).join(' ').trim();
  const email = member.user?.email?.trim() || '';
  return {
    ...member,
    member_name: name || email || member.user_id,
    member_email: email || '-',
    status: member.status || 'active',
  };
}

function lifecycleBucket(status: string): CrudLifecycleView {
  const normalized = status.trim().toLowerCase();
  if (normalized === 'archived') return 'archived';
  if (normalized === 'deleted' || normalized === 'removed' || normalized === 'disabled') return 'archived';
  return 'active';
}

function statusForAction(action: PymesIAMCrudPath['action'], trashStatus: string): string {
  if (action === 'archive') return 'archived';
  if (action === 'trash') return trashStatus;
  if (action === 'unarchive' || action === 'restore') return 'active';
  return '';
}

function formatStatus(status: string): string {
  switch (status.trim().toLowerCase()) {
    case 'active':
      return 'activo';
    case 'archived':
      return 'archivado';
    case 'deleted':
    case 'removed':
    case 'disabled':
      return 'papelera';
    default:
      return status || '-';
  }
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}
