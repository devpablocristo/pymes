import type { AuditEntry } from '../lib/types';
import { formatDateTime } from './AdminPage.model';

type AdminAuditSectionProps = {
  activity: AuditEntry[];
  filteredActivity: AuditEntry[];
  auditExportBusy: boolean;
  onExportCsv: () => void;
};

export function AdminAuditSection({
  activity,
  filteredActivity,
  auditExportBusy,
  onExportCsv,
}: AdminAuditSectionProps) {
  return (
    <div className="card">
      <div className="card-header admin-card-header--wrap">
        <h2>Registro de auditoría</h2>
        <div className="admin-audit-header-actions">
          <span className="badge badge-neutral">{activity.length} eventos</span>
          <button type="button" className="btn-sm btn-secondary" disabled={auditExportBusy} onClick={onExportCsv}>
            {auditExportBusy ? 'Descargando…' : 'Descargar CSV'}
          </button>
        </div>
      </div>
      {activity.length === 0 ? (
        <div className="empty-state">
          <p>Sin eventos registrados</p>
        </div>
      ) : (
        <div className="admin-activity-wrap">
          <table className="admin-activity-table">
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Acción</th>
                <th>Recurso</th>
                <th>ID</th>
                <th>Actor</th>
              </tr>
            </thead>
            <tbody>
              {filteredActivity.slice(0, 50).map((row) => (
                <tr key={row.id}>
                  <td>{formatDateTime(row.created_at)}</td>
                  <td>
                    <code className="admin-code">{row.action}</code>
                  </td>
                  <td>
                    <code className="admin-code">{row.resource_type}</code>
                  </td>
                  <td className="admin-activity-id">{row.resource_id ?? '—'}</td>
                  <td>{row.actor ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
