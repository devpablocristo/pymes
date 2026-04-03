/**
 * Ajustes — solo configuración del producto (preferencias, apariencia, integraciones, etc.).
 * El trabajo operativo del negocio vive en el menú lateral / módulos, no acá.
 */
import { useSearch } from '@devpablocristo/modules-search';
import type { CSSProperties } from 'react';
import { lazy, Suspense, useCallback, useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  SectionHubPage,
  parseSectionHubSelection,
  type SectionHubSection,
} from '@devpablocristo/modules-ui-section-hub';
import '@devpablocristo/modules-ui-section-hub/styles.css';
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
import { usePageSearch } from '../components/PageSearch';
import { LanguageSelector } from '../components/LanguageSelector';
import { themeHubColorSwatches } from '../lib/productPalette';
import './SettingsHubPage.css';

function SettingsDeepLink({ to, title, desc }: { to: string; title: string; desc: string }) {
  return (
    <Link to={to} className="card stg__deep-link">
      <strong>{title}</strong>
      <p className="text-secondary stg__deep-link-desc">{desc}</p>
    </Link>
  );
}

const AdminPage = lazy(() => import('./AdminPage').then((m) => ({ default: m.AdminPage })));
const BillingSection = lazy(() => import('./SettingsPage').then((m) => ({ default: m.BillingSettingsSection })));
const ProfilePage = lazy(() => import('./SettingsPage').then((m) => ({ default: m.SettingsPage })));
const NotificationPreferencesPage = lazy(() =>
  import('./NotificationPreferencesPage').then((m) => ({ default: m.NotificationPreferencesPage })),
);

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

type SectionCard = SectionHubSection<Exclude<Section, null>>;

const SETTING_SECTIONS: SectionCard[] = [
  { id: 'profile', label: 'Perfil', desc: 'Datos personales y cuenta', icon: <IconUsers /> },
  { id: 'workspace', label: 'Negocio', desc: 'Razón social, monedas, IVA, prefijos', icon: <IconBuilding /> },
  { id: 'appearance', label: 'Apariencia', desc: 'Tema, skin, logos y colores', icon: <IconPalette /> },
  { id: 'language', label: 'Idioma', desc: 'Idioma de la plataforma', icon: <IconGlobe /> },
  {
    id: 'notifications',
    label: 'Notificaciones',
    desc: 'Preferencias de correo y canales de alerta',
    icon: <IconBell />,
  },
  { id: 'automation', label: 'Automatización', desc: 'Reglas del asistente y tareas proactivas', icon: <IconAlert /> },
  { id: 'gateway', label: 'Pagos y facturación', desc: 'Plan, pasarelas y métodos de cobro', icon: <IconCreditCard /> },
  { id: 'currencies', label: 'Monedas', desc: 'Monedas habilitadas', icon: <IconDollar /> },
  { id: 'company', label: 'Empresa', desc: 'Datos de contacto y dirección', icon: <IconBuilding /> },
  { id: 'firebaseNotif', label: 'Firebase', desc: 'Configuración push notifications', icon: <IconSettings /> },
];

// ─── Automatización (solo pantallas de configuración del asistente) ───

function AutomationHubTab() {
  return (
    <div className="card stg__card-mb">
      <div className="stg__stack">
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
function sectionFromSearchParam(raw: string | null): Section {
  return parseSectionHubSelection(SETTING_SECTIONS, raw);
}

// ─── Toggle ───

function Toggle({
  checked,
  onChange,
  ariaLabel,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  ariaLabel?: string;
}) {
  return (
    <label className="stg__switch">
      <input type="checkbox" checked={checked} aria-label={ariaLabel} onChange={(e) => onChange(e.target.checked)} />
      <span className="stg__switch-slider" />
    </label>
  );
}

// ─── Company ───

function CompanyTab() {
  const [form, setForm] = useState({
    name: '',
    email: '',
    phone: '',
    website: '',
    country: '',
    city: '',
    state: '',
    zip: '',
    address: '',
  });
  const set = (k: string) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((p) => ({ ...p, [k]: e.target.value }));

  return (
    <div className="card">
      <div className="stg__form-grid">
        <div className="form-group">
          <label>Nombre completo</label>
          <input
            aria-label="Nombre completo"
            value={form.name}
            onChange={set('name')}
            placeholder="Mi Empresa S.R.L."
          />
        </div>
        <div className="form-group">
          <label>Email</label>
          <input
            aria-label="Email"
            type="email"
            value={form.email}
            onChange={set('email')}
            placeholder="info@empresa.com"
          />
        </div>
        <div className="form-group">
          <label>Teléfono</label>
          <input aria-label="Teléfono" value={form.phone} onChange={set('phone')} placeholder="+54 11 1234-5678" />
        </div>
        <div className="form-group">
          <label>Sitio web</label>
          <input
            aria-label="Sitio web"
            value={form.website}
            onChange={set('website')}
            placeholder="https://empresa.com"
          />
        </div>
        <div className="form-group">
          <label>País</label>
          <select aria-label="País" value={form.country} onChange={set('country')}>
            <option value="">Seleccionar</option>
            <option>Argentina</option>
            <option>Chile</option>
            <option>Colombia</option>
            <option>México</option>
            <option>Perú</option>
            <option>Uruguay</option>
          </select>
        </div>
        <div className="form-group">
          <label>Ciudad</label>
          <input aria-label="Ciudad" value={form.city} onChange={set('city')} placeholder="Buenos Aires" />
        </div>
        <div className="form-group">
          <label>Provincia</label>
          <input aria-label="Provincia" value={form.state} onChange={set('state')} placeholder="CABA" />
        </div>
        <div className="form-group">
          <label>Código postal</label>
          <input aria-label="Código postal" value={form.zip} onChange={set('zip')} placeholder="C1000" />
        </div>
        <div className="form-group stg__form-full">
          <label>Dirección</label>
          <input
            aria-label="Dirección"
            value={form.address}
            onChange={set('address')}
            placeholder="Av. Corrientes 1234"
          />
        </div>
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">
          Resetear
        </button>
        <button type="button" className="btn-primary btn-sm">
          Guardar
        </button>
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
          <div key={f.label} className="form-group">
            <label>{f.label}</label>
            <input aria-label={f.label} placeholder={f.placeholder} />
          </div>
        ))}
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">
          Resetear
        </button>
        <button type="button" className="btn-primary btn-sm">
          Guardar
        </button>
      </div>
    </div>
  );
}

// ─── Alert Channels ───

function AlertChannelsTab() {
  const [channels, setChannels] = useState([
    {
      id: 'mail',
      label: 'Email',
      desc: 'Notificaciones por correo electrónico',
      enabled: true,
      text: 'Se envió un nuevo mensaje a tu bandeja.',
    },
    { id: 'sms', label: 'SMS', desc: 'Notificaciones por mensaje de texto', enabled: false, text: '' },
    { id: 'push', label: 'Push', desc: 'Notificaciones push en el navegador', enabled: false, text: '' },
  ]);

  const toggle = (id: string) =>
    setChannels((prev) => prev.map((c) => (c.id === id ? { ...c, enabled: !c.enabled } : c)));
  const setText = (id: string, text: string) =>
    setChannels((prev) => prev.map((c) => (c.id === id ? { ...c, text } : c)));

  return (
    <div className="card">
      {channels.map((ch) => (
        <div key={ch.id} className="stg__channel-block">
          <div className="stg__toggle">
            <div>
              <div className="stg__toggle-label">{ch.label}</div>
              <div className="stg__toggle-desc">{ch.desc}</div>
            </div>
            <Toggle checked={ch.enabled} ariaLabel={`Activar ${ch.label}`} onChange={() => toggle(ch.id)} />
          </div>
          {ch.enabled && (
            <div className="form-group stg__form-group-mt">
              <textarea
                aria-label={`Texto del mensaje para ${ch.label}`}
                rows={2}
                value={ch.text}
                onChange={(e) => setText(ch.id, e.target.value)}
                placeholder="Texto del mensaje…"
              />
            </div>
          )}
        </div>
      ))}
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">
          Resetear
        </button>
        <button type="button" className="btn-primary btn-sm">
          Guardar
        </button>
      </div>
    </div>
  );
}

// ─── Theme ───

/** Swatches de “color principal”: derivados de `productPalette` (ids estables: primary, success, …). */
const THEME_COLORS = themeHubColorSwatches();

function ThemeTab() {
  const [selected, setSelected] = useState(THEME_COLORS[0]?.id ?? 'primary');
  return (
    <div className="card">
      <div className="form-group stg__form-group-mb">
        <label>Logo (subir imagen)</label>
        <input aria-label="Logo (subir imagen)" type="file" accept="image/*" />
      </div>
      <div className="form-group stg__form-group-mb">
        <label>Logo oscuro (subir imagen)</label>
        <input aria-label="Logo oscuro (subir imagen)" type="file" accept="image/*" />
      </div>
      <div className="form-group">
        <label className="stg__label-mb">Color principal</label>
        <div className="stg__colors">
          {THEME_COLORS.map((c) => (
            <button
              key={c.id}
              type="button"
              className={`stg__color ${selected === c.id ? 'stg__color--active' : ''}`}
              style={{ '--stg-swatch-bg': c.bg } as CSSProperties}
              onClick={() => setSelected(c.id)}
              aria-label={`Seleccionar color ${c.label}`}
              aria-pressed={selected === c.id}
              title={c.label}
            />
          ))}
        </div>
      </div>
      <div className="stg__form-actions">
        <button type="button" className="btn-secondary btn-sm">
          Resetear
        </button>
        <button type="button" className="btn-primary btn-sm">
          Guardar
        </button>
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
  const search = usePageSearch();
  const currencyText = useCallback((c: Currency) => `${c.name} ${c.code}`, []);
  const filtered = useSearch(items, currencyText, search);

  const toggleActive = (id: string) => setItems((p) => p.map((c) => (c.id === id ? { ...c, active: !c.active } : c)));
  const remove = (id: string) => setItems((p) => p.filter((c) => c.id !== id));

  return (
    <div className="card">
      <div className="stg__crud-toolbar">
        <div className="stg__toolbar-spacer" />
        <button type="button" className="btn-primary btn-sm">
          + Agregar moneda
        </button>
      </div>
      <table className="stg__crud-table">
        <thead>
          <tr>
            <th>Nombre</th>
            <th>Símbolo</th>
            <th>Código</th>
            <th>Cripto</th>
            <th>Estado</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {filtered.map((c) => (
            <tr key={c.id}>
              <td className="stg__crud-cell-name">{c.name}</td>
              <td>{c.symbol}</td>
              <td>{c.code}</td>
              <td>{c.crypto ? <span className="badge badge-neutral">Sí</span> : '—'}</td>
              <td>
                <Toggle checked={c.active} ariaLabel={`Activar moneda ${c.name}`} onChange={() => toggleActive(c.id)} />
              </td>
              <td>
                <div className="stg__crud-actions">
                  <button
                    type="button"
                    className="stg__crud-action stg__crud-action--edit"
                    aria-label={`Editar ${c.name}`}
                    title="Editar"
                  >
                    <IconEdit />
                  </button>
                  <button
                    type="button"
                    className="stg__crud-action stg__crud-action--delete"
                    aria-label={`Eliminar ${c.name}`}
                    title="Eliminar"
                    onClick={() => remove(c.id)}
                  >
                    <IconTrash />
                  </button>
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
  const toggle = (id: string) => setItems((p) => p.map((l) => (l.id === id ? { ...l, active: !l.active } : l)));
  const remove = (id: string) => setItems((p) => p.filter((l) => l.id !== id));

  return (
    <div className="card">
      <div className="stg__crud-toolbar">
        <button type="button" className="btn-primary btn-sm">
          + Agregar idioma
        </button>
      </div>
      <table className="stg__crud-table">
        <thead>
          <tr>
            <th>Idioma</th>
            <th>Estado</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {items.map((l) => (
            <tr key={l.id}>
              <td className="stg__crud-cell-name">{l.name}</td>
              <td>
                <Toggle checked={l.active} ariaLabel={`Activar idioma ${l.name}`} onChange={() => toggle(l.id)} />
              </td>
              <td>
                <div className="stg__crud-actions">
                  <button
                    type="button"
                    className="stg__crud-action stg__crud-action--edit"
                    aria-label={`Editar ${l.name}`}
                    title="Editar"
                  >
                    <IconEdit />
                  </button>
                  <button
                    type="button"
                    className="stg__crud-action stg__crud-action--delete"
                    aria-label={`Eliminar ${l.name}`}
                    title="Eliminar"
                    onClick={() => remove(l.id)}
                  >
                    <IconTrash />
                  </button>
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
    {
      name: 'Mercado Pago',
      desc: 'Pagos con tarjeta, transferencia y QR',
      enabled: true,
      fields: ['Access Token', 'Public Key'],
    },
    {
      name: 'Stripe',
      desc: 'Pagos internacionales con tarjeta',
      enabled: false,
      fields: ['Secret Key', 'Publishable Key', 'Webhook Secret'],
    },
    { name: 'PayPal', desc: 'Pagos vía PayPal', enabled: false, fields: ['Client ID', 'Client Secret'] },
  ];
  const [states, setStates] = useState(gateways.map((g) => g.enabled));
  const toggleGw = (i: number) => setStates((p) => p.map((v, j) => (j === i ? !v : v)));

  return (
    <div className="stg__gateway-stack">
      {gateways.map((gw, i) => (
        <div key={gw.name} className="card">
          <div className={`stg__toggle ${!states[i] ? 'stg__toggle--collapsed' : ''}`}>
            <div>
              <div className="stg__toggle-label">{gw.name}</div>
              <div className="stg__toggle-desc">{gw.desc}</div>
            </div>
            <Toggle checked={states[i]} ariaLabel={`Activar pasarela ${gw.name}`} onChange={() => toggleGw(i)} />
          </div>
          {states[i] && (
            <>
              <div className="stg__form-grid stg__form-grid--after-toggle">
                {gw.fields.map((f) => (
                  <div key={f} className="form-group">
                    <label>{f}</label>
                    <input aria-label={`${gw.name}: ${f}`} placeholder={`Ingresá tu ${f}…`} />
                  </div>
                ))}
              </div>
              <div className="stg__form-actions">
                <button type="button" className="btn-primary btn-sm">
                  Guardar
                </button>
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
  const settingsSearch = usePageSearch();
  const sectionTextFn = useCallback((s: SectionCard) => `${s.label} ${s.desc}`, []);
  const filteredSections = useSearch(SETTING_SECTIONS, sectionTextFn, settingsSearch);
  const [searchParams, setSearchParams] = useSearchParams();
  const [section, setSection] = useState<Section>(() => sectionFromSearchParam(searchParams.get('section')));

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

  const activeSectionCard = SETTING_SECTIONS.find((item) => item.id === section) ?? null;

  return (
    <SectionHubPage
      className="stg"
      pageTitle="Ajustes"
      pageLead="Elegí un área para configurar tu cuenta y tu espacio de trabajo."
      sections={SETTING_SECTIONS}
      visibleSections={filteredSections}
      emptyState={
        <div className="card">
          <p className="text-secondary u-m-0">No hay secciones de ajustes que coincidan con la búsqueda actual.</p>
        </div>
      }
      activeSectionId={section}
      onOpenSection={openSection}
      onBack={goBackToGrid}
      backLabel={activeSectionCard ? '← Volver a Ajustes' : 'Volver'}
    >
      {section === 'profile' && (
        <Suspense fallback={<div className="spinner" />}>
          <ProfilePage embedded />
        </Suspense>
      )}
      {section === 'notifications' && (
        <>
          <div className="card stg__card-mb">
            <p className="text-secondary u-m-0 u-text-base">
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
          <Suspense fallback={<div className="spinner" />}>
            <BillingSection />
          </Suspense>
          <GatewayTab />
        </>
      )}
      {section === 'appearance' && (
        <>
          <Suspense fallback={<div className="spinner" />}>
            <AdminPage section="appearance" embedded />
          </Suspense>
          <div className="card">
            <AdminSkinSelector />
          </div>
          <ThemeTab />
        </>
      )}
      {section === 'language' && (
        <>
          <div className="card">
            <LanguageSelector />
          </div>
          <LanguagesTab />
        </>
      )}
      {section === 'workspace' && (
        <Suspense fallback={<div className="spinner" />}>
          <AdminPage section="workspace" embedded />
        </Suspense>
      )}
    </SectionHubPage>
  );
}

export default SettingsHubPage;
