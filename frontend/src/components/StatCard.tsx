import type { ReactNode } from 'react';

export type StatCardTone = 'blue' | 'green' | 'purple' | 'red' | 'amber';

type StatCardProps = {
  /** Etiqueta corta (ej. "Ventas", "Ticket promedio"). */
  label: string;
  /** Valor principal a mostrar. Aceptamos `ReactNode` para iconos o badges inline. */
  value: ReactNode;
  /** Línea secundaria opcional (ej. período, conteo). */
  sub?: ReactNode;
  /** Tono cromático del icono. Mapea a `--color-{tone}-subtle/--color-{tone}`. */
  tone?: StatCardTone;
  /** Icono custom; si no se provee, usa la primera letra del label como fallback. */
  icon?: ReactNode;
  /** Si está cargando, muestra spinner en lugar del valor. */
  loading?: boolean;
};

/**
 * StatCard — tarjeta KPI reusable para dashboards.
 *
 * Usa las clases `dash__stat-*` definidas en `DashboardVisualPage.css`
 * (estilo tokens nativos). Pensada para dashboards verticales (medical,
 * workshops) que quieran el mismo look del dashboard principal.
 *
 * @example
 *   <StatCard label="Pacientes" value="124" sub="este mes" tone="blue" />
 */
export function StatCard({ label, value, sub, tone = 'blue', icon, loading }: StatCardProps) {
  return (
    <div className="dash__stat-card">
      <div className={`dash__stat-icon dash__stat-icon--${tone}`}>
        {loading ? '…' : icon ?? label.charAt(0)}
      </div>
      <div className="dash__stat-info">
        <div className="dash__stat-value">{loading ? <span className="spinner" /> : value}</div>
        <div className="dash__stat-label">{label}</div>
        {sub ? <div className="dash__stat-trend dash__stat-trend--muted">{sub}</div> : null}
      </div>
    </div>
  );
}
