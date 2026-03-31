/**
 * Ajustes — solo configuración del producto (preferencias, apariencia, integraciones, etc.).
 * El trabajo operativo del negocio vive en el menú lateral / módulos, no acá.
 */
import { lazy, Suspense, useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  IconAlert,
  IconBell,
  IconBuilding,
  IconCreditCard,
  IconDollar,
  IconEdit,
  IconGlobe,
  IconPalette,
  IconSettings,
  IconTrash,
  IconUsers,
} from '@devpablocristo/modules-ui-data-display/icons';
import { AdminSkinSelector } from '../components/AdminSkinSelector';
import { LanguageSelector } from '../components/LanguageSelector';
import './SettingsHubPage.css';

const deepLinkCardStyle: CSSProperties = {
  margin: 0,
  padding: '1rem',
  textDecoration: 'none',
  color: 'inherit',
  display: 'block',
  border: '1px solid var(--color-border, #e5e7eb)',
};

function SettingsDeepLink({ to, title, desc }: { to: string; title: string; desc: string }) {
  return (
    <Link to={to} className="card" style={deepLinkCardStyle}>
      <strong>{title}</strong>
      <p className="text-secondary" style={{ margin: '0.35rem 0 0', fontSize: '0.88rem' }}>
        {desc}
      </p>
    </Link>
  );
}

const AdminPage = lazy(() => import('./AdminPage').then((m) => ({ default: m.AdminPage })));
const BillingSection = lazy(() => import('./SettingsPage').then((m) => ({ default: m.BillingSettingsSection })));
const ProfilePage = lazy(() => import('./SettingsPage').then((m) => ({ default: m.SettingsPage })));
const NotificationPreferencesPage = lazy(() => import('./NotificationPreferencesPage').then((m) => ({ default: m.NotificationPreferencesPage })));

type Section =
  | null
  | 'profile'
  | 'notifications'
  | 'automation'
  | 'company'
  | 'firebaseNotif'
  | 'currencies'
  | 'gateway'
  | 'appearance'
  | 'language'
  | 'workspace';

type SectionCard = { id: Exclude<Section, null>; label: string; desc: string; icon: ReactNode };

const SETTING_SECTIONS: SectionCard[] = [
  { id: 'profile', label: 'Perfil', desc: 'Datos personales y cuenta', icon: <IconUsers /> },
  { id: 'workspace', label: 'Negocio', desc: 'Razón social, monedas, IVA, prefijos', icon: <IconBuilding /> },
  { id: 'appearance', label: 'Apariencia', desc: 'Tema, skin, logos y colores', icon: <IconPalette /> },
  { id: 'language', label: 'Idioma', desc: 'Idioma de la plataforma', icon: <IconGlobe /> },
  { id: 'notifications', label: 'Notificaciones', desc: 'Preferencias de correo y canales de alerta', icon: <IconBell /> },
  { id: 'automation', label: 'Automatización', desc: 'Reglas del asistente y tareas proactivas', icon: <IconAlert /> },
  { id: 'gateway', label: 'Pagos y facturación', desc: 'Plan, pasarelas y métodos de cobro', icon: <IconCreditCard /> },
  { id: 'currencies', label: 'Monedas', desc: 'Monedas habilitadas', icon: <IconDollar /> },
  { id: 'company', label: 'Empresa', desc: 'Datos de contacto y dirección', icon: <IconBuilding /> },
  { id: 'firebaseNotif', label: 'Firebase', desc: 'Configuración push notifications', icon: <IconSettings /> },
];

// ─── Automatización (solo pantallas de configuración del asistente) ───

function AutomationHubTab() {
  return (
    <div className="card" style={{ marginBottom: 'var(--space-4)' }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        <SettingsDeepLink
          to="/automation-rules"
          title="Atención automática"
          desc="Políticas y reglas de revisión del asistente."
        />
        <SettingsDeepLink
          to="/watcher-config"
          title="Asistente proactivo"
          desc="Monitores y tareas programadas sugeridas por el sistema."
        />
      </div>
    </div>
  );
}

/** Valores válidos de `?section=` para deep link dentro de Ajustes. */
const ALL_KNOWN_SECTION_IDS: readonly Exclude<Section, null>[] = SETTING_SECTIONS.map((s) => s.id);

function sectionFromSearchParam(raw: string | null): Section {
  if (!raw) return null;
  return (ALL_KNOWN_SECTION_IDS as readonly string[]).includes(raw) ? (raw as Exclude<Section, null>) : null;
}

// ─── Toggle ───

function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="stg__switch">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="stg__switch-slider" />
    </label>
  );
}

// ─── Company ───

function CompanyTab() {
  const [form, setForm] = useState({
    name: '', email: '', phone: '', website: '', country: '', city: '', state: '', zip: '', address: '',
  });
  const set = (k: string) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((p) => ({ ...p, [k]: e.target.value }));

  return (
    <div className="card">
      <div className="stg__form-grid">
        <div className="form-group"><label>Nombre completo</label><input value={form.name} onChange={set('name')} placeholder="Mi Empresa S.R.L." /></div>
        <div className="form-group"><label>Email</label><input type="email" value={form.email} onChange={set('email')} placeholder="info@empresa.com" /></div>
        <div className="form-group"><label>Teléfono</label><input value={form.phone} onChange={set('phone')} placeholder="+54 11 1234-5678" /></div>
        <div className="form-group"><label>Sitio web</label><input value={form.website} onChange={set('website')} placeholder="https://empresa.com" /></div>
        <div className="form-group"><label>País</label>
          <select value={form.country} onChange={set('country')}><option value="">Seleccionar</option><option>Argentina</option><option>Chile</option><option>Colombia</option><option>México</option><option>Perú</option><option>Uruguay</option></select>
        </div>
        <div className="form-group"><label>Ciudad</label><input value={form.city} onChange={set('city')} placeholder="Buenos Aires" /></div>
        <div className="form-group"><label>Provincia</label><input value={form.state} onChange={set('state')} placeholder="CABA" /></div>
        <div className="form-group"><label>Código postal</label><input value={form.zip} onChange={set('zip')} placeholder="C1000" /></div>
        <div className="form-group stg__form-full"><label>Dirección</label><input value={form.address} onChange={set('address')} placeholder="Av. Corrientes 1234" /></div>
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">Resetear</button>
        <button type="button" className="btn-primary btn-sm">Guardar</button>
      </div>
    </div>
  );
}

// ─── Firebase Notifications ───

function FirebaseNotifTab() {
  const fields = [
    { label: 'Secret Key', placeholder: 'AAAA...' },
    { label: 'VAPID Key', placeholder: 'BKagO...' },
    { label: 'API Key', placeholder: 'AIzaS...' },
    { label: 'Auth Domain', placeholder: 'proyecto.firebaseapp.com' },
    { label: 'Project ID', placeholder: 'mi-proyecto' },
    { label: 'Storage Bucket', placeholder: 'proyecto.appspot.com' },
    { label: 'Sender ID', placeholder: '123456789' },
    { label: 'App ID', placeholder: '1:123:web:abc' },
  ];
  return (
    <div className="card">
      <div className="stg__form-grid">
        {fields.map((f) => (
          <div key={f.label} className="form-group"><label>{f.label}</label><input placeholder={f.placeholder} /></div>
        ))}
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">Resetear</button>
        <button type="button" className="btn-primary btn-sm">Guardar</button>
      </div>
    </div>
  );
}

// ─── Alert Channels ───

function AlertChannelsTab() {
  const [channels, setChannels] = useState([
    { id: 'mail', label: 'Email', desc: 'Notificaciones por correo electrónico', enabled: true, text: 'Se envió un nuevo mensaje a tu bandeja.' },
    { id: 'sms', label: 'SMS', desc: 'Notificaciones por mensaje de texto', enabled: false, text: '' },
    { id: 'push', label: 'Push', desc: 'Notificaciones push en el navegador', enabled: false, text: '' },
  ]);

  const toggle = (id: string) => setChannels((prev) => prev.map((c) => c.id === id ? { ...c, enabled: !c.enabled } : c));
  const setText = (id: string, text: string) => setChannels((prev) => prev.map((c) => c.id === id ? { ...c, text } : c));

  return (
    <div className="card">
      {channels.map((ch) => (
        <div key={ch.id} style={{ marginBottom: 'var(--space-4)' }}>
          <div className="stg__toggle">
            <div>
              <div className="stg__toggle-label">{ch.label}</div>
              <div className="stg__toggle-desc">{ch.desc}</div>
            </div>
            <Toggle checked={ch.enabled} onChange={() => toggle(ch.id)} />
          </div>
          {ch.enabled && (
            <div className="form-group" style={{ marginTop: 'var(--space-2)' }}>
              <textarea rows={2} value={ch.text} onChange={(e) => setText(ch.id, e.target.value)} placeholder="Texto del mensaje…" />
            </div>
          )}
        </div>
      ))}
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">Resetear</button>
        <button type="button" className="btn-primary btn-sm">Guardar</button>
      </div>
    </div>
  );
}

// ─── Theme ───

const THEME_COLORS = [
  { id: 'blue', color: '#3b82f6', label: 'Azul' },
  { id: 'green', color: '#10b981', label: 'Verde' },
  { id: 'orange', color: '#f59e0b', label: 'Naranja' },
  { id: 'red', color: '#ef4444', label: 'Rojo' },
  { id: 'purple', color: '#8b5cf6', label: 'Violeta' },
  { id: 'pink', color: '#ec4899', label: 'Rosa' },
];

function ThemeTab() {
  const [selected, setSelected] = useState('blue');
  return (
    <div className="card">
      <div className="form-group" style={{ marginBottom: 'var(--space-4)' }}>
        <label>Logo (subir imagen)</label>
        <input type="file" accept="image/*" />
      </div>
      <div className="form-group" style={{ marginBottom: 'var(--space-4)' }}>
        <label>Logo oscuro (subir imagen)</label>
        <input type="file" accept="image/*" />
      </div>
      <div className="form-group">
        <label style={{ marginBottom: 'var(--space-2)' }}>Color principal</label>
        <div className="stg__colors">
          {THEME_COLORS.map((c) => (
            <button
              key={c.id}
              type="button"
              className={`stg__color ${selected === c.id ? 'stg__color--active' : ''}`}
              style={{ background: c.color }}
              onClick={() => setSelected(c.id)}
              title={c.label}
            />
          ))}
        </div>
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">Resetear</button>
        <button type="button" className="btn-primary btn-sm">Guardar</button>
      </div>
    </div>
  );
}

// ─── Currencies ───

type Currency = { id: string; name: string; symbol: string; code: string; crypto: boolean; active: boolean };

const DEFAULT_CURRENCIES: Currency[] = [
  { id: '1', name: 'Peso Argentino', symbol: '$', code: 'ARS', crypto: false, active: true },
  { id: '2', name: 'Dólar', symbol: 'US$', code: 'USD', crypto: false, active: true },
  { id: '3', name: 'Euro', symbol: '€', code: 'EUR', crypto: false, active: false },
  { id: '4', name: 'Real Brasileño', symbol: 'R$', code: 'BRL', crypto: false, active: false },
  { id: '5', name: 'Bitcoin', symbol: '₿', code: 'BTC', crypto: true, active: false },
];

function CurrenciesTab() {
  const [items, setItems] = useState(DEFAULT_CURRENCIES);
  const [search, setSearch] = useState('');
  const filtered = items.filter((c) => !search || c.name.toLowerCase().includes(search.toLowerCase()) || c.code.toLowerCase().includes(search.toLowerCase()));

  const toggleActive = (id: string) => setItems((p) => p.map((c) => c.id === id ? { ...c, active: !c.active } : c));
  const remove = (id: string) => setItems((p) => p.filter((c) => c.id !== id));

  return (
    <div className="card">
      <div className="stg__crud-toolbar">
        <input type="search" placeholder="Buscar moneda…" value={search} onChange={(e) => setSearch(e.target.value)} style={{ maxWidth: 250 }} />
        <button type="button" className="btn-primary btn-sm">+ Agregar moneda</button>
      </div>
      <table className="stg__crud-table">
        <thead><tr><th>Nombre</th><th>Símbolo</th><th>Código</th><th>Cripto</th><th>Estado</th><th>Acciones</th></tr></thead>
        <tbody>
          {filtered.map((c) => (
            <tr key={c.id}>
              <td style={{ fontWeight: 600 }}>{c.name}</td>
              <td>{c.symbol}</td>
              <td>{c.code}</td>
              <td>{c.crypto ? <span className="badge badge-neutral">Sí</span> : '—'}</td>
              <td><Toggle checked={c.active} onChange={() => toggleActive(c.id)} /></td>
              <td>
                <div className="stg__crud-actions">
                  <button type="button" className="stg__crud-action stg__crud-action--edit" title="Editar"><IconEdit /></button>
                  <button type="button" className="stg__crud-action stg__crud-action--delete" title="Eliminar" onClick={() => remove(c.id)}><IconTrash /></button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Languages ───

type Language = { id: string; name: string; active: boolean };

const DEFAULT_LANGUAGES: Language[] = [
  { id: '1', name: 'Español', active: true },
  { id: '2', name: 'Inglés', active: true },
  { id: '3', name: 'Portugués', active: false },
  { id: '4', name: 'Francés', active: false },
];

function LanguagesTab() {
  const [items, setItems] = useState(DEFAULT_LANGUAGES);
  const toggle = (id: string) => setItems((p) => p.map((l) => l.id === id ? { ...l, active: !l.active } : l));
  const remove = (id: string) => setItems((p) => p.filter((l) => l.id !== id));

  return (
    <div className="card">
      <div className="stg__crud-toolbar">
        <button type="button" className="btn-primary btn-sm">+ Agregar idioma</button>
      </div>
      <table className="stg__crud-table">
        <thead><tr><th>Idioma</th><th>Estado</th><th>Acciones</th></tr></thead>
        <tbody>
          {items.map((l) => (
            <tr key={l.id}>
              <td style={{ fontWeight: 600 }}>{l.name}</td>
              <td><Toggle checked={l.active} onChange={() => toggle(l.id)} /></td>
              <td>
                <div className="stg__crud-actions">
                  <button type="button" className="stg__crud-action stg__crud-action--edit" title="Editar"><IconEdit /></button>
                  <button type="button" className="stg__crud-action stg__crud-action--delete" title="Eliminar" onClick={() => remove(l.id)}><IconTrash /></button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ─── Payment Gateway ───

function GatewayTab() {
  const gateways = [
    { name: 'Mercado Pago', desc: 'Pagos con tarjeta, transferencia y QR', enabled: true, fields: ['Access Token', 'Public Key'] },
    { name: 'Stripe', desc: 'Pagos internacionales con tarjeta', enabled: false, fields: ['Secret Key', 'Publishable Key', 'Webhook Secret'] },
    { name: 'PayPal', desc: 'Pagos vía PayPal', enabled: false, fields: ['Client ID', 'Client Secret'] },
  ];
  const [states, setStates] = useState(gateways.map((g) => g.enabled));
  const toggleGw = (i: number) => setStates((p) => p.map((v, j) => j === i ? !v : v));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {gateways.map((gw, i) => (
        <div key={gw.name} className="card">
          <div className="stg__toggle" style={{ borderBottom: states[i] ? undefined : 'none' }}>
            <div>
              <div className="stg__toggle-label">{gw.name}</div>
              <div className="stg__toggle-desc">{gw.desc}</div>
            </div>
            <Toggle checked={states[i]} onChange={() => toggleGw(i)} />
          </div>
          {states[i] && (
            <>
              <div className="stg__form-grid" style={{ marginTop: 'var(--space-3)' }}>
                {gw.fields.map((f) => (
                  <div key={f} className="form-group"><label>{f}</label><input placeholder={`Ingresá tu ${f}…`} /></div>
                ))}
              </div>
              <div className="stg__form-actions">
                <button type="button" className="btn-primary btn-sm">Guardar</button>
              </div>
            </>
          )}
        </div>
      ))}
    </div>
  );
}

// ─── Página principal ───

export function SettingsHubPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [section, setSection] = useState<Section>(() =>
    sectionFromSearchParam(searchParams.get('section')),
  );

  useEffect(() => {
    setSection(sectionFromSearchParam(searchParams.get('section')));
  }, [searchParams]);

  function openSection(id: Exclude<Section, null>): void {
    setSection(id);
    setSearchParams({ section: id }, { replace: true });
  }

  function goBackToGrid(): void {
    setSection(null);
    if (searchParams.get('section')) {
      setSearchParams({}, { replace: true });
    }
  }

  return (
    <div className="stg">
      {section === null ? (
        <div className="stg__nav-grid">
          {SETTING_SECTIONS.map((s) => (
            <button
              key={s.id}
              type="button"
              className="stg__nav-card"
              onClick={() => openSection(s.id)}
            >
              <div className="stg__nav-icon">{s.icon}</div>
              <div className="stg__nav-info">
                <div className="stg__nav-title">{s.label}</div>
                <div className="stg__nav-desc">{s.desc}</div>
              </div>
            </button>
          ))}
        </div>
      ) : (
        <>
          <button type="button" className="stg__back" onClick={goBackToGrid}>
            ← Volver a Ajustes
          </button>

          {section === 'profile' && <Suspense fallback={<div className="spinner" />}><ProfilePage /></Suspense>}
          {section === 'notifications' && (
            <>
              <div className="card" style={{ marginBottom: 'var(--space-4)' }}>
                <p className="text-secondary" style={{ margin: 0, fontSize: '0.88rem' }}>
                  La bandeja de avisos y aprobaciones está en el menú <strong>Base → Notificaciones</strong> (
                  <Link to="/notifications">abrir centro</Link>
                  ).
                </p>
              </div>
              <Suspense fallback={<div className="spinner" />}>
                <NotificationPreferencesPage embedded />
              </Suspense>
              <AlertChannelsTab />
            </>
          )}
          {section === 'automation' && <AutomationHubTab />}
          {section === 'company' && <CompanyTab />}
          {section === 'firebaseNotif' && <FirebaseNotifTab />}
          {section === 'currencies' && <CurrenciesTab />}
          {section === 'gateway' && (
            <>
              <Suspense fallback={<div className="spinner" />}><BillingSection /></Suspense>
              <GatewayTab />
            </>
          )}
          {section === 'appearance' && (
            <>
              <Suspense fallback={<div className="spinner" />}><AdminPage section="appearance" /></Suspense>
              <div className="card"><AdminSkinSelector /></div>
              <ThemeTab />
            </>
          )}
          {section === 'language' && (
            <>
              <div className="card"><LanguageSelector /></div>
              <LanguagesTab />
            </>
          )}
          {section === 'workspace' && <Suspense fallback={<div className="spinner" />}><AdminPage section="workspace" /></Suspense>}
        </>
      )}
    </div>
  );
}

export default SettingsHubPage;
