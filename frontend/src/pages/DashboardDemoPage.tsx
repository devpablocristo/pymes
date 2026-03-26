/**
 * Dashboard demo — unifica lo mejor de los 11 dashboards Wowdash.
 * Charts CSS puro (sin ApexCharts) para evitar problemas CJS/ESM.
 */
import './DashboardDemoPage.css';

// ─── Stat Cards ───

const STATS = [
  { label: 'Clientes', value: '1,240', trend: '+120', up: true, icon: '👥', tone: 'blue' as const },
  { label: 'Ventas del mes', value: '$842K', trend: '+18%', up: true, icon: '💰', tone: 'green' as const },
  { label: 'Turnos hoy', value: '24', trend: '+5', up: true, icon: '📅', tone: 'purple' as const },
  { label: 'Facturas pendientes', value: '18', trend: '+3', up: false, icon: '📄', tone: 'amber' as const },
  { label: 'Gastos', value: '$215K', trend: '+8%', up: false, icon: '📊', tone: 'red' as const },
];

function StatCards() {
  return (
    <div className="dash__stats">
      {STATS.map((s) => (
        <div key={s.label} className="dash__stat-card">
          <div className={`dash__stat-icon dash__stat-icon--${s.tone}`}>{s.icon}</div>
          <div className="dash__stat-info">
            <div className="dash__stat-value">{s.value}</div>
            <div className="dash__stat-label">{s.label}</div>
            <div className={`dash__stat-trend ${s.up ? 'dash__stat-trend--up' : 'dash__stat-trend--down'}`}>
              {s.up ? '↑' : '↓'} {s.trend}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Sales Chart (CSS bars) ───

const SALES_DATA = [
  { month: 'Ene', value: 42 }, { month: 'Feb', value: 58 }, { month: 'Mar', value: 35 },
  { month: 'Abr', value: 72 }, { month: 'May', value: 65 }, { month: 'Jun', value: 88 },
  { month: 'Jul', value: 52 }, { month: 'Ago', value: 78 }, { month: 'Sep', value: 92 },
  { month: 'Oct', value: 68 }, { month: 'Nov', value: 85 }, { month: 'Dic', value: 95 },
];

function SalesChart() {
  const max = Math.max(...SALES_DATA.map((d) => d.value));
  return (
    <div className="card">
      <div className="dash__chart-header">
        <div>
          <h3 className="dash__chart-title">Ventas mensuales</h3>
          <div className="dash__chart-metric">$842,200</div>
          <span className="dash__stat-trend dash__stat-trend--up">↑ 18% vs año anterior</span>
        </div>
      </div>
      <div className="dash__bars">
        {SALES_DATA.map((d) => (
          <div key={d.month} className="dash__bar-col">
            <div
              className="dash__bar dash__bar--primary"
              style={{ height: `${(d.value / max) * 100}%` }}
            />
            <span className="dash__bar-label">{d.month}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Revenue vs Expenses ───

const REV_EXP = [
  { month: 'Ene', rev: 65, exp: 40 }, { month: 'Feb', rev: 72, exp: 45 },
  { month: 'Mar', rev: 55, exp: 38 }, { month: 'Abr', rev: 80, exp: 50 },
  { month: 'May', rev: 78, exp: 55 }, { month: 'Jun', rev: 92, exp: 48 },
];

function RevenueExpenseChart() {
  const max = Math.max(...REV_EXP.flatMap((d) => [d.rev, d.exp]));
  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Ingresos vs Gastos</h3>
        <div style={{ display: 'flex', gap: '0.75rem', fontSize: '0.75rem' }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
            <span style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--color-primary, #3b82f6)', display: 'inline-block' }} />
            Ingresos
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
            <span style={{ width: 10, height: 10, borderRadius: 2, background: '#f59e0b', display: 'inline-block' }} />
            Gastos
          </span>
        </div>
      </div>
      <div className="dash__bars">
        {REV_EXP.map((d) => (
          <div key={d.month} className="dash__bar-col" style={{ flexDirection: 'column', gap: 2, alignItems: 'center' }}>
            <div style={{ display: 'flex', gap: 3, alignItems: 'flex-end', height: '100%' }}>
              <div className="dash__bar dash__bar--primary" style={{ height: `${(d.rev / max) * 100}%`, width: 14 }} />
              <div className="dash__bar dash__bar--amber" style={{ height: `${(d.exp / max) * 100}%`, width: 14 }} />
            </div>
            <span className="dash__bar-label">{d.month}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Users Overview (CSS Donut) ───

function UsersOverview() {
  const segments = [
    { label: 'Nuevos', value: 500, color: '#3b82f6' },
    { label: 'Activos', value: 820, color: '#10b981' },
    { label: 'Inactivos', value: 180, color: '#f59e0b' },
  ];
  const total = segments.reduce((s, seg) => s + seg.value, 0);
  let accum = 0;
  const gradientParts = segments.map((seg) => {
    const start = (accum / total) * 360;
    accum += seg.value;
    const end = (accum / total) * 360;
    return `${seg.color} ${start}deg ${end}deg`;
  });

  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Usuarios</h3>
      </div>
      <div className="dash__donut-wrap">
        <div className="dash__donut" style={{ background: `conic-gradient(${gradientParts.join(', ')})` }}>
          <div className="dash__donut-center">{total.toLocaleString()}</div>
        </div>
        <div className="dash__donut-legend">
          {segments.map((seg) => (
            <div key={seg.label} className="dash__donut-legend-item">
              <span className="dash__donut-legend-dot" style={{ background: seg.color }} />
              <span>{seg.label}: <strong>{seg.value}</strong></span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ─── Subscribers Sparkline ───

function SubscribersCard() {
  const data = [6, 12, 8, 18, 10, 15, 20];
  const days = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];
  const max = Math.max(...data);
  return (
    <div className="card">
      <div className="dash__chart-header">
        <div>
          <h3 className="dash__chart-title">Suscriptores</h3>
          <div className="dash__chart-metric">5,240</div>
          <span className="dash__stat-trend dash__stat-trend--up">↑ 12% esta semana</span>
        </div>
      </div>
      <div className="dash__bars" style={{ height: 80 }}>
        {data.map((v, i) => (
          <div key={i} className="dash__bar-col">
            <div className="dash__bar dash__bar--success" style={{ height: `${(v / max) * 100}%` }} />
            <span className="dash__bar-label">{days[i]}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Latest Registered Table ───

const USERS = [
  { name: 'María García', email: 'maria@ejemplo.com', initials: 'MG', bg: '#eff6ff', color: '#3b82f6', date: '25 Mar 2026', plan: 'Premium', status: 'Activo' },
  { name: 'Juan Pérez', email: 'juan@ejemplo.com', initials: 'JP', bg: '#ecfdf5', color: '#10b981', date: '24 Mar 2026', plan: 'Básico', status: 'Activo' },
  { name: 'Ana López', email: 'ana@ejemplo.com', initials: 'AL', bg: '#f5f3ff', color: '#8b5cf6', date: '23 Mar 2026', plan: 'Estándar', status: 'Activo' },
  { name: 'Carlos Ruiz', email: 'carlos@ejemplo.com', initials: 'CR', bg: '#fffbeb', color: '#f59e0b', date: '22 Mar 2026', plan: 'Premium', status: 'Pendiente' },
  { name: 'Laura Díaz', email: 'laura@ejemplo.com', initials: 'LD', bg: '#fef2f2', color: '#ef4444', date: '21 Mar 2026', plan: 'Gratis', status: 'Activo' },
];

function LatestRegistered() {
  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Últimos registrados</h3>
      </div>
      <table className="dash__table">
        <thead>
          <tr><th>Usuario</th><th>Fecha</th><th>Plan</th><th>Estado</th></tr>
        </thead>
        <tbody>
          {USERS.map((u) => (
            <tr key={u.name}>
              <td>
                <div className="dash__user">
                  <div className="dash__user-avatar" style={{ background: u.bg, color: u.color }}>{u.initials}</div>
                  <div>
                    <div className="dash__user-name">{u.name}</div>
                    <div className="dash__user-email">{u.email}</div>
                  </div>
                </div>
              </td>
              <td>{u.date}</td>
              <td>{u.plan}</td>
              <td><span className={`badge ${u.status === 'Activo' ? 'badge-success' : 'badge-warning'}`}>{u.status}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Top Performers ───

const PERFORMERS = [
  { name: 'Sofía Torres', role: 'Vendedora', value: '$48,200', initials: 'ST', bg: '#eff6ff', color: '#3b82f6' },
  { name: 'Pedro Sánchez', role: 'Vendedor', value: '$42,100', initials: 'PS', bg: '#ecfdf5', color: '#10b981' },
  { name: 'Elena Martín', role: 'Vendedora', value: '$38,900', initials: 'EM', bg: '#f5f3ff', color: '#8b5cf6' },
  { name: 'Diego Flores', role: 'Vendedor', value: '$35,400', initials: 'DF', bg: '#fffbeb', color: '#f59e0b' },
  { name: 'Lucía Romero', role: 'Vendedora', value: '$31,800', initials: 'LR', bg: '#fef2f2', color: '#ef4444' },
];

function TopPerformers() {
  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Top vendedores</h3>
      </div>
      {PERFORMERS.map((p) => (
        <div key={p.name} className="dash__performer">
          <div className="dash__user-avatar" style={{ background: p.bg, color: p.color }}>{p.initials}</div>
          <div className="dash__performer-info">
            <div className="dash__performer-name">{p.name}</div>
            <div className="dash__performer-role">{p.role}</div>
          </div>
          <div className="dash__performer-value">{p.value}</div>
        </div>
      ))}
    </div>
  );
}

// ─── Top Products / Countries ───

const PRODUCTS = [
  { name: 'Plan Premium', sales: 342, pct: 85, color: 'var(--color-primary, #3b82f6)' },
  { name: 'Plan Estándar', sales: 256, pct: 64, color: 'var(--color-success, #10b981)' },
  { name: 'Plan Básico', sales: 198, pct: 50, color: '#f59e0b' },
  { name: 'Consultoría', sales: 145, pct: 36, color: '#8b5cf6' },
  { name: 'Soporte Premium', sales: 89, pct: 22, color: '#ef4444' },
];

function TopProducts() {
  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Productos más vendidos</h3>
      </div>
      {PRODUCTS.map((p) => (
        <div key={p.name} style={{ marginBottom: '0.85rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem', fontSize: '0.82rem' }}>
            <span style={{ fontWeight: 600 }}>{p.name}</span>
            <span style={{ color: 'var(--color-text-muted)', fontSize: '0.75rem' }}>{p.sales} ventas</span>
          </div>
          <div className="dash__progress-wrap">
            <div className="dash__progress">
              <div className="dash__progress-fill" style={{ width: `${p.pct}%`, background: p.color }} />
            </div>
            <span className="dash__progress-pct">{p.pct}%</span>
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Recent Transactions ───

const TRANSACTIONS = [
  { id: 'TXN-4521', customer: 'María García', date: '25 Mar', amount: '$15,000', type: 'Ingreso' as const },
  { id: 'TXN-4520', customer: 'Juan Pérez', date: '24 Mar', amount: '$8,500', type: 'Ingreso' as const },
  { id: 'TXN-4519', customer: 'Proveedor ABC', date: '24 Mar', amount: '-$3,200', type: 'Gasto' as const },
  { id: 'TXN-4518', customer: 'Ana López', date: '23 Mar', amount: '$25,000', type: 'Ingreso' as const },
  { id: 'TXN-4517', customer: 'Hosting Cloud', date: '22 Mar', amount: '-$4,800', type: 'Gasto' as const },
];

function RecentTransactions() {
  return (
    <div className="card">
      <div className="dash__chart-header">
        <h3 className="dash__chart-title">Últimas transacciones</h3>
      </div>
      <table className="dash__table">
        <thead>
          <tr><th>ID</th><th>Concepto</th><th>Fecha</th><th>Monto</th></tr>
        </thead>
        <tbody>
          {TRANSACTIONS.map((t) => (
            <tr key={t.id}>
              <td style={{ fontWeight: 600, fontSize: '0.78rem' }}>{t.id}</td>
              <td>{t.customer}</td>
              <td>{t.date}</td>
              <td style={{ fontWeight: 600, color: t.type === 'Ingreso' ? 'var(--color-success, #10b981)' : 'var(--color-danger, #ef4444)' }}>
                {t.amount}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Página ───

export function DashboardDemoPage() {
  return (
    <div className="dash">
      <div className="page-header">
        <h1>Dashboard</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>
          Resumen general del negocio — métricas, ventas, usuarios y operaciones
        </p>
      </div>

      <StatCards />

      <div className="dash__grid--3">
        <SalesChart />
        <UsersOverview />
        <SubscribersCard />
      </div>

      <div className="dash__grid">
        <RevenueExpenseChart />
        <TopProducts />
      </div>

      <div className="dash__grid--3">
        <LatestRegistered />
        <TopPerformers />
        <RecentTransactions />
      </div>
    </div>
  );
}

export default DashboardDemoPage;
