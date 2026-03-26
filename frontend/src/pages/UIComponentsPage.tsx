/**
 * Componentes UI — showcase de 19 categorías de componentes reutilizables.
 */
import { useState } from 'react';
import { IconStar } from '../components/Icons';
import './UIComponentsPage.css';

type Tab = 'alerts' | 'avatars' | 'badges' | 'buttons' | 'cards' | 'carousel' | 'colors' | 'dropdowns' | 'pagination' | 'progress' | 'radio' | 'rating' | 'switches' | 'tabs' | 'tags' | 'tooltips' | 'typography' | 'lists' | 'videos';

const TABS: { id: Tab; label: string }[] = [
  { id: 'alerts', label: 'Alertas' }, { id: 'avatars', label: 'Avatares' }, { id: 'badges', label: 'Badges' },
  { id: 'buttons', label: 'Botones' }, { id: 'cards', label: 'Cards' }, { id: 'carousel', label: 'Carrusel' },
  { id: 'colors', label: 'Colores' }, { id: 'dropdowns', label: 'Dropdowns' }, { id: 'pagination', label: 'Paginación' },
  { id: 'progress', label: 'Progreso' }, { id: 'radio', label: 'Radio' }, { id: 'rating', label: 'Estrellas' },
  { id: 'switches', label: 'Switches' }, { id: 'tabs', label: 'Tabs' }, { id: 'tags', label: 'Tags' },
  { id: 'tooltips', label: 'Tooltips' }, { id: 'typography', label: 'Tipografía' }, { id: 'lists', label: 'Listas' },
  { id: 'videos', label: 'Videos' },
];

const COLORS = [
  { name: 'Primary', hex: '#3b82f6' }, { name: 'Success', hex: '#10b981' }, { name: 'Warning', hex: '#f59e0b' },
  { name: 'Danger', hex: '#ef4444' }, { name: 'Purple', hex: '#8b5cf6' }, { name: 'Pink', hex: '#ec4899' },
  { name: 'Cyan', hex: '#06b6d4' }, { name: 'Neutral', hex: '#64748b' },
];

function AlertsSection() {
  return (
    <div className="card">      <div className="ui-catalog__alert ui-catalog__alert--info">Información — Este es un mensaje informativo.</div>
      <div className="ui-catalog__alert ui-catalog__alert--success">Éxito — La operación se completó correctamente.</div>
      <div className="ui-catalog__alert ui-catalog__alert--warning">Advertencia — Revisá los datos antes de continuar.</div>
      <div className="ui-catalog__alert ui-catalog__alert--danger">Error — No se pudo completar la acción.</div>
    </div>
  );
}

function AvatarsSection() {
  const sizes = [
    { cls: 'sm', label: 'SM' }, { cls: 'md', label: 'MD' }, { cls: 'lg', label: 'LG' }, { cls: 'xl', label: 'XL' },
  ];
  return (
    <div className="card">      <div className="ui-catalog__row">
        {sizes.map((s) => (
          <div key={s.cls} className={`ui-catalog__avatar ui-catalog__avatar--${s.cls}`} style={{ background: '#3b82f6' }}>{s.label}</div>
        ))}
      </div>      <div className="ui-catalog__row">
        {COLORS.slice(0, 6).map((c) => (
          <div key={c.name} className="ui-catalog__avatar ui-catalog__avatar--md" style={{ background: c.hex }}>{c.name.slice(0, 2)}</div>
        ))}
      </div>
    </div>
  );
}

function BadgesSection() {
  return (
    <div className="card">      <div className="ui-catalog__row">
        <span className="badge badge-success">Activo</span>
        <span className="badge badge-warning">Pendiente</span>
        <span className="badge badge-danger">Vencido</span>
        <span className="badge badge-neutral">Borrador</span>
      </div>      <div className="ui-catalog__row">
        {COLORS.map((c) => <span key={c.name} style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 999, fontSize: '0.75rem', fontWeight: 600, background: c.hex + '20', color: c.hex, border: `1px solid ${c.hex}40` }}>{c.name}</span>)}
      </div>
    </div>
  );
}

function ButtonsSection() {
  return (
    <div className="card">      <div className="ui-catalog__row">
        <button type="button" className="btn-primary">Primary</button>
        <button type="button" className="btn-secondary">Secondary</button>
        <button type="button" className="btn-success">Success</button>
        <button type="button" className="btn-danger">Danger</button>
      </div>      <div className="ui-catalog__row">
        <button type="button" className="btn-primary btn-sm">Pequeño</button>
        <button type="button" className="btn-primary">Normal</button>
      </div>      <div className="ui-catalog__row">
        <button type="button" className="btn-primary" disabled>Deshabilitado</button>
        <button type="button" className="btn-secondary" disabled>Deshabilitado</button>
      </div>
    </div>
  );
}

function CardsSection() {
  return (
    <div className="ui-catalog__grid" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(250px, 1fr))' }}>
      <div className="card"><p style={{ fontSize: 'var(--text-base)', color: 'var(--color-text-secondary)' }}>Contenido simple dentro de una card con padding estándar.</p></div>
      <div className="card" style={{ padding: 0 }}>
        <div style={{ height: 120, background: 'linear-gradient(135deg, #3b82f6, #8b5cf6)', borderRadius: 'var(--radius-lg) var(--radius-lg) 0 0' }} />
        <div style={{ padding: 'var(--space-4)' }}><p style={{ fontSize: 'var(--text-base)', color: 'var(--color-text-secondary)' }}>Header visual con contenido debajo.</p></div>
      </div>
      <div className="stat-card">
        <div style={{ fontSize: '1.5rem', fontWeight: 700, color: 'var(--color-primary)' }}>$42K</div>
        <div style={{ fontSize: '0.82rem', color: 'var(--color-text-secondary)' }}>Ingresos del mes</div>
      </div>
    </div>
  );
}

function CarouselSection() {
  const [idx, setIdx] = useState(0);
  const slides = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6'];
  return (
    <div className="card">      <div style={{ borderRadius: 'var(--radius-md)', overflow: 'hidden', position: 'relative', height: 180, background: slides[idx], display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '1.5rem', fontWeight: 700, transition: 'background 0.3s ease' }}>
        Slide {idx + 1}
      </div>
      <div className="ui-catalog__row" style={{ justifyContent: 'center', marginTop: 'var(--space-3)' }}>
        <button type="button" className="btn-secondary btn-sm" onClick={() => setIdx((idx - 1 + slides.length) % slides.length)}>&larr;</button>
        {slides.map((_, i) => (
          <button key={i} type="button" onClick={() => setIdx(i)} style={{ width: 10, height: 10, borderRadius: '50%', border: 'none', background: i === idx ? 'var(--color-primary)' : 'var(--color-border)', cursor: 'pointer', padding: 0 }} />
        ))}
        <button type="button" className="btn-secondary btn-sm" onClick={() => setIdx((idx + 1) % slides.length)}>&rarr;</button>
      </div>
    </div>
  );
}

function ColorsSection() {
  return (
    <div className="card">      <div className="ui-catalog__grid" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))' }}>
        {COLORS.map((c) => (
          <div key={c.name} className="ui-catalog__swatch">
            <div className="ui-catalog__swatch-color" style={{ background: c.hex }} />
            <div className="ui-catalog__swatch-info"><strong>{c.name}</strong><br />{c.hex}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DropdownsSection() {
  const [open, setOpen] = useState(false);
  return (
    <div className="card">      <div className="ui-catalog__row">
        <div style={{ position: 'relative' }}>
          <button type="button" className="btn-primary btn-sm" onClick={() => setOpen(!open)}>Dropdown ▾</button>
          {open && (
            <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: 4, background: 'var(--color-surface)', border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', boxShadow: 'var(--shadow-md)', zIndex: 10, minWidth: 160, padding: '0.25rem 0' }}>
              {['Acción 1', 'Acción 2', 'Acción 3'].map((a) => (
                <button key={a} type="button" onClick={() => setOpen(false)} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '0.45rem 0.85rem', border: 'none', background: 'transparent', cursor: 'pointer', fontSize: '0.82rem', color: 'var(--color-text)' }}>{a}</button>
              ))}
            </div>
          )}
        </div>
        <select style={{ maxWidth: 200 }}><option>Select dropdown</option><option>Opción A</option><option>Opción B</option></select>
      </div>
    </div>
  );
}

function PaginationSection() {
  const [page, setPage] = useState(1);
  return (
    <div className="card">      <div className="ui-catalog__row">
        <button type="button" className="inv__page-btn" onClick={() => setPage(Math.max(1, page - 1))}>&larr;</button>
        {[1, 2, 3, 4, 5].map((p) => (
          <button key={p} type="button" className={`inv__page-btn ${page === p ? 'inv__page-btn--active' : ''}`} onClick={() => setPage(p)}>{p}</button>
        ))}
        <button type="button" className="inv__page-btn" onClick={() => setPage(Math.min(5, page + 1))}>&rarr;</button>
      </div>
    </div>
  );
}

function ProgressSection() {
  const bars = [
    { label: 'Primary', pct: 75, color: '#3b82f6' }, { label: 'Success', pct: 50, color: '#10b981' },
    { label: 'Warning', pct: 35, color: '#f59e0b' }, { label: 'Danger', pct: 90, color: '#ef4444' },
  ];
  return (
    <div className="card">      {bars.map((b) => (
        <div key={b.label}>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.78rem', marginBottom: '0.2rem' }}>
            <span>{b.label}</span><span>{b.pct}%</span>
          </div>
          <div className="ui-catalog__progress"><div className="ui-catalog__progress-fill" style={{ width: `${b.pct}%`, background: b.color }} /></div>
        </div>
      ))}
    </div>
  );
}

function RadioSection() {
  const [v, setV] = useState('a');
  return (
    <div className="card">      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
        {[{ id: 'a', label: 'Opción A' }, { id: 'b', label: 'Opción B' }, { id: 'c', label: 'Opción C (deshabilitada)' }].map((o) => (
          <label key={o.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem', cursor: o.id === 'c' ? 'not-allowed' : 'pointer', opacity: o.id === 'c' ? 0.5 : 1 }}>
            <input type="radio" name="ui-radio" checked={v === o.id} onChange={() => setV(o.id)} disabled={o.id === 'c'} />
            {o.label}
          </label>
        ))}
      </div>
    </div>
  );
}

function RatingSection() {
  const [rating, setRating] = useState(3);
  return (
    <div className="card">      <div className="ui-catalog__stars">
        {[1, 2, 3, 4, 5].map((i) => (
          <span key={i} className={`ui-catalog__star ${i <= rating ? 'ui-catalog__star--filled' : ''}`} onClick={() => setRating(i)}><IconStar filled={i <= rating} /></span>
        ))}
      </div>
      <p style={{ fontSize: '0.82rem', color: 'var(--color-text-secondary)', marginTop: '0.5rem' }}>Seleccionado: {rating}/5</p>
    </div>
  );
}

function SwitchesSection() {
  const [s1, setS1] = useState(true);
  const [s2, setS2] = useState(false);
  return (
    <div className="card">      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', fontSize: '0.85rem' }}>
          <label className="ui-catalog__switch"><input type="checkbox" checked={s1} onChange={() => setS1(!s1)} /><span className="ui-catalog__switch-track" /></label>
          Activo ({s1 ? 'ON' : 'OFF'})
        </label>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', fontSize: '0.85rem' }}>
          <label className="ui-catalog__switch"><input type="checkbox" checked={s2} onChange={() => setS2(!s2)} /><span className="ui-catalog__switch-track" /></label>
          Notificaciones ({s2 ? 'ON' : 'OFF'})
        </label>
      </div>
    </div>
  );
}

function TabsSection() {
  const [t, setT] = useState(0);
  return (
    <div className="card">      <div className="ui-catalog__preview-tabs">
        {['General', 'Perfil', 'Seguridad'].map((label, i) => (
          <button key={label} type="button" className={`ui-catalog__preview-tab ${t === i ? 'ui-catalog__preview-tab--active' : ''}`} onClick={() => setT(i)}>{label}</button>
        ))}
      </div>
      <p style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
        Contenido de la pestaña "{['General', 'Perfil', 'Seguridad'][t]}".
      </p>
    </div>
  );
}

function TagsSection() {
  const tags = ['React', 'TypeScript', 'CSS', 'Node.js', 'PostgreSQL', 'Docker'];
  return (
    <div className="card">      <div className="ui-catalog__row">
        {tags.map((tag, i) => <span key={tag} style={{ display: 'inline-block', padding: '4px 12px', borderRadius: 999, fontSize: '0.78rem', fontWeight: 600, background: COLORS[i % COLORS.length].hex + '18', color: COLORS[i % COLORS.length].hex, border: `1px solid ${COLORS[i % COLORS.length].hex}30` }}>{tag}</span>)}
      </div>
    </div>
  );
}

function TooltipsSection() {
  return (
    <div className="card">      <div className="ui-catalog__row">
        {['Arriba', 'Abajo', 'Izquierda', 'Derecha'].map((dir) => (
          <button key={dir} type="button" className="btn-secondary btn-sm" title={`Tooltip ${dir.toLowerCase()}`}>Hover: {dir}</button>
        ))}
      </div>
      <p style={{ fontSize: '0.78rem', color: 'var(--color-text-muted)', marginTop: 'var(--space-2)' }}>Pasá el cursor sobre los botones para ver tooltips nativos del navegador.</p>
    </div>
  );
}

function TypographySection() {
  return (
    <div className="card">      <h1 style={{ margin: '0 0 0.5rem' }}>Heading 1</h1>
      <h2 style={{ margin: '0 0 0.5rem' }}>Heading 2</h2>
      <h3 style={{ margin: '0 0 0.5rem' }}>Heading 3</h3>
      <h4 style={{ margin: '0 0 0.5rem' }}>Heading 4</h4>
      <p style={{ marginTop: 'var(--space-3)' }}>
        Texto de párrafo normal. <strong>Negrita.</strong> <em>Itálica.</em>{' '}
        <a href="#" onClick={(e) => e.preventDefault()}>Enlace.</a>{' '}
        <code style={{ background: 'var(--color-surface-hover)', padding: '2px 6px', borderRadius: 4, fontSize: '0.85em' }}>código inline</code>
      </p>
      <blockquote style={{ borderLeft: '4px solid var(--color-primary)', paddingLeft: 'var(--space-4)', margin: 'var(--space-3) 0', color: 'var(--color-text-secondary)', fontStyle: 'italic' }}>
        Cita de ejemplo — "La simplicidad es la máxima sofisticación."
      </blockquote>
    </div>
  );
}

function ListsSection() {
  const items = ['Elemento activo', 'Segundo elemento', 'Tercer elemento', 'Cuarto elemento'];
  return (
    <div className="card">      <div style={{ border: '1px solid var(--color-border)', borderRadius: 'var(--radius-sm)', overflow: 'hidden' }}>
        {items.map((item, i) => (
          <div key={item} style={{ padding: '0.6rem 0.85rem', borderBottom: i < items.length - 1 ? '1px solid var(--color-border-subtle)' : undefined, background: i === 0 ? 'var(--color-primary-subtle)' : undefined, fontWeight: i === 0 ? 600 : 400, fontSize: '0.85rem' }}>
            {item}
          </div>
        ))}
      </div>
    </div>
  );
}

function VideosSection() {
  return (
    <div className="card">      <div className="ui-catalog__video-wrap">
        <iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" title="Video embebido" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowFullScreen />
      </div>
    </div>
  );
}

const TAB_COMPONENTS: Record<Tab, () => JSX.Element> = {
  alerts: AlertsSection, avatars: AvatarsSection, badges: BadgesSection, buttons: ButtonsSection,
  cards: CardsSection, carousel: CarouselSection, colors: ColorsSection, dropdowns: DropdownsSection,
  pagination: PaginationSection, progress: ProgressSection, radio: RadioSection, rating: RatingSection,
  switches: SwitchesSection, tabs: TabsSection, tags: TagsSection, tooltips: TooltipsSection,
  typography: TypographySection, lists: ListsSection, videos: VideosSection,
};

export function UIComponentsPage() {
  const [tab, setTab] = useState<Tab>('alerts');
  const Content = TAB_COMPONENTS[tab];

  return (
    <div className="ui-catalog">
      <div className="ui-catalog__tabs">
        {TABS.map((t) => (
          <button key={t.id} type="button" className={`ui-catalog__tab ${tab === t.id ? 'ui-catalog__tab--active' : ''}`} onClick={() => setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>

      <Content />
    </div>
  );
}

export default UIComponentsPage;
