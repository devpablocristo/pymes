/**
 * Componentes UI — showcase de 19 categorías de componentes reutilizables.
 */
import type { CSSProperties } from 'react';
import { useState } from 'react';
import { IconStar } from '@devpablocristo/modules-ui-data-display/icons';
import { PRODUCT_PALETTE } from '../lib/productPalette';
import { PageLayout } from '../components/PageLayout';
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

function AlertsSection() {
  return (
    <div className="card">
      <div className="ui-catalog__alert ui-catalog__alert--info">Información — Este es un mensaje informativo.</div>
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
    <div className="card">
      <div className="ui-catalog__row">
        {sizes.map((s) => (
          <div key={s.cls} className={`ui-catalog__avatar ui-catalog__avatar--${s.cls} ui-catalog__avatar--brand`}>
            {s.label}
          </div>
        ))}
      </div>
      <div className="ui-catalog__row">
        {PRODUCT_PALETTE.slice(0, 6).map((c) => (
          <div
            key={c.id}
            className="ui-catalog__avatar ui-catalog__avatar--md ui-catalog__avatar--token"
            style={{ '--ui-token': c.token } as CSSProperties}
          >
            {c.label.slice(0, 2)}
          </div>
        ))}
      </div>
    </div>
  );
}

function BadgesSection() {
  return (
    <div className="card">
      <div className="ui-catalog__row">
        <span className="badge badge-success">Activo</span>
        <span className="badge badge-warning">Pendiente</span>
        <span className="badge badge-danger">Vencido</span>
        <span className="badge badge-neutral">Borrador</span>
      </div>
      <div className="ui-catalog__row">
        {PRODUCT_PALETTE.map((c) => (
          <span
            key={c.id}
            className="ui-catalog__pill-palette"
            style={{ '--ui-palette-color': c.token } as CSSProperties}
          >
            {c.label}
          </span>
        ))}
      </div>
    </div>
  );
}

function ButtonsSection() {
  return (
    <div className="card">
      <div className="ui-catalog__row">
        <button type="button" className="btn-primary">Primary</button>
        <button type="button" className="btn-secondary">Secondary</button>
        <button type="button" className="btn-success">Success</button>
        <button type="button" className="btn-danger">Danger</button>
      </div>
      <div className="ui-catalog__row">
        <button type="button" className="btn-primary btn-sm">Pequeño</button>
        <button type="button" className="btn-primary">Normal</button>
      </div>
      <div className="ui-catalog__row">
        <button type="button" className="btn-primary" disabled>Deshabilitado</button>
        <button type="button" className="btn-secondary" disabled>Deshabilitado</button>
      </div>
    </div>
  );
}

function CardsSection() {
  return (
    <div className="ui-catalog__grid ui-catalog__grid--cards-wide">
      <div className="card">
        <p className="ui-catalog__lead">Contenido simple dentro de una card con padding estándar.</p>
      </div>
      <div className="card ui-catalog__card--flush">
        <div className="ui-catalog__card-hero-strip" />
        <div className="ui-catalog__card-body-pad">
          <p className="ui-catalog__lead">Header visual con contenido debajo.</p>
        </div>
      </div>
      <div className="stat-card">
        <div className="ui-catalog__stat-big">$42K</div>
        <div className="ui-catalog__stat-caption">Ingresos del mes</div>
      </div>
    </div>
  );
}

function CarouselSection() {
  const [idx, setIdx] = useState(0);
  const slides = PRODUCT_PALETTE.slice(0, 4).map((p) => p.token);
  return (
    <div className="card">
      <div className="ui-catalog__carousel-slide" style={{ '--carousel-bg': slides[idx] } as CSSProperties}>
        Slide {idx + 1}
      </div>
      <div className="ui-catalog__row ui-catalog__row--carousel-dots">
        <button type="button" className="btn-secondary btn-sm" onClick={() => setIdx((idx - 1 + slides.length) % slides.length)} aria-label="Slide anterior">&larr;</button>
        {slides.map((_, i) => (
          <button
            key={i}
            type="button"
            className={`ui-catalog__carousel-dot ${i === idx ? 'ui-catalog__carousel-dot--active' : ''}`}
            onClick={() => setIdx(i)}
            aria-label={`Ir a slide ${i + 1}`}
            aria-current={i === idx ? 'true' : undefined}
          />
        ))}
        <button type="button" className="btn-secondary btn-sm" onClick={() => setIdx((idx + 1) % slides.length)} aria-label="Slide siguiente">&rarr;</button>
      </div>
    </div>
  );
}

function ColorsSection() {
  return (
    <div className="card">
      <div className="ui-catalog__grid ui-catalog__grid--colors-tight">
        {PRODUCT_PALETTE.map((c) => (
          <div key={c.id} className="ui-catalog__swatch">
            <div
              className="ui-catalog__swatch-color ui-catalog__swatch-color--token"
              style={{ '--ui-token': c.token } as CSSProperties}
            />
            <div className="ui-catalog__swatch-info"><strong>{c.label}</strong><br />{c.hex}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DropdownsSection() {
  const [open, setOpen] = useState(false);
  return (
    <div className="card">
      <div className="ui-catalog__row">
        <div className="ui-catalog__dropdown-wrap">
          <button type="button" className="btn-primary btn-sm" onClick={() => setOpen(!open)}>Dropdown ▾</button>
          {open && (
            <div className="ui-catalog__dropdown-panel">
              {['Acción 1', 'Acción 2', 'Acción 3'].map((a) => (
                <button key={a} type="button" className="ui-catalog__dropdown-item" onClick={() => setOpen(false)}>
                  {a}
                </button>
              ))}
            </div>
          )}
        </div>
        <select className="ui-catalog__select-demo">
          <option>Select dropdown</option>
          <option>Opción A</option>
          <option>Opción B</option>
        </select>
      </div>
    </div>
  );
}

function PaginationSection() {
  const [page, setPage] = useState(1);
  return (
    <div className="card">
      <div className="ui-catalog__row">
        <button type="button" className="inv__page-btn" onClick={() => setPage(Math.max(1, page - 1))}>&larr;</button>
        {[1, 2, 3, 4, 5].map((p) => (
          <button key={p} type="button" className={`inv__page-btn ${page === p ? 'inv__page-btn--active' : ''}`} onClick={() => setPage(p)}>{p}</button>
        ))}
        <button type="button" className="inv__page-btn" onClick={() => setPage(Math.min(5, page + 1))}>&rarr;</button>
      </div>
    </div>
  );
}

/** Porcentajes demo alineados 1:1 con los cuatro primeros swatches de `PRODUCT_PALETTE`. */
const PROGRESS_DEMO_PCTS = [75, 50, 35, 90] as const;

function ProgressSection() {
  const bars = PRODUCT_PALETTE.slice(0, 4).map((p, i) => ({
    id: p.id,
    label: p.label,
    pct: PROGRESS_DEMO_PCTS[i],
    color: p.token,
  }));
  return (
    <div className="card">
      {bars.map((b) => (
        <div key={b.id}>
          <div className="ui-catalog__progress-head">
            <span>{b.label}</span>
            <span>{b.pct}%</span>
          </div>
          <div className="ui-catalog__progress">
            <div
              className="ui-catalog__progress-fill ui-catalog__progress-fill--token"
              style={{ width: `${b.pct}%`, '--ui-progress-color': b.color } as CSSProperties}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

function RadioSection() {
  const [v, setV] = useState('a');
  return (
    <div className="card">
      <div className="ui-catalog__stack">
        {[{ id: 'a', label: 'Opción A' }, { id: 'b', label: 'Opción B' }, { id: 'c', label: 'Opción C (deshabilitada)' }].map((o) => (
          <label
            key={o.id}
            className={`ui-catalog__radio-label ${o.id === 'c' ? 'ui-catalog__radio-label--disabled' : ''}`}
          >
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
    <div className="card">
      <div className="ui-catalog__stars">
        {[1, 2, 3, 4, 5].map((i) => (
          <button key={i} type="button" className={`ui-catalog__star ${i <= rating ? 'ui-catalog__star--filled' : ''}`} onClick={() => setRating(i)} aria-label={`Puntuar ${i} de 5`}><IconStar filled={i <= rating} /></button>
        ))}
      </div>
      <p className="ui-catalog__rating-caption">Seleccionado: {rating}/5</p>
    </div>
  );
}

function SwitchesSection() {
  const [s1, setS1] = useState(true);
  const [s2, setS2] = useState(false);
  return (
    <div className="card">
      <div className="ui-catalog__stack ui-catalog__stack--loose">
        <label className="ui-catalog__switch-row">
          <span className="ui-catalog__switch">
            <input type="checkbox" checked={s1} onChange={() => setS1(!s1)} />
            <span className="ui-catalog__switch-track" />
          </span>
          Activo ({s1 ? 'ON' : 'OFF'})
        </label>
        <label className="ui-catalog__switch-row">
          <span className="ui-catalog__switch">
            <input type="checkbox" checked={s2} onChange={() => setS2(!s2)} />
            <span className="ui-catalog__switch-track" />
          </span>
          Notificaciones ({s2 ? 'ON' : 'OFF'})
        </label>
      </div>
    </div>
  );
}

function TabsSection() {
  const [t, setT] = useState(0);
  return (
    <div className="card">
      <div className="ui-catalog__preview-tabs">
        {['General', 'Perfil', 'Seguridad'].map((label, i) => (
          <button key={label} type="button" className={`ui-catalog__preview-tab ${t === i ? 'ui-catalog__preview-tab--active' : ''}`} onClick={() => setT(i)}>{label}</button>
        ))}
      </div>
      <p className="ui-catalog__tab-lead">
        Contenido de la pestaña "{['General', 'Perfil', 'Seguridad'][t]}".
      </p>
    </div>
  );
}

function TagsSection() {
  const tags = ['React', 'TypeScript', 'CSS', 'Node.js', 'PostgreSQL', 'Docker'];
  return (
    <div className="card">
      <div className="ui-catalog__row">
        {tags.map((tag, i) => {
          const c = PRODUCT_PALETTE[i % PRODUCT_PALETTE.length];
          return (
            <span
              key={tag}
              className="ui-catalog__tag-palette"
              style={{ '--ui-palette-color': c.token } as CSSProperties}
            >
              {tag}
            </span>
          );
        })}
      </div>
    </div>
  );
}

function TooltipsSection() {
  return (
    <div className="card">
      <div className="ui-catalog__row">
        {['Arriba', 'Abajo', 'Izquierda', 'Derecha'].map((dir) => (
          <button key={dir} type="button" className="btn-secondary btn-sm" title={`Tooltip ${dir.toLowerCase()}`}>Hover: {dir}</button>
        ))}
      </div>
      <p className="ui-catalog__hint-muted">Pasá el cursor sobre los botones para ver tooltips nativos del navegador.</p>
    </div>
  );
}

function TypographySection() {
  return (
    <div className="card ui-catalog__typography">
      <h1>Heading 1</h1>
      <h2>Heading 2</h2>
      <h3>Heading 3</h3>
      <h4>Heading 4</h4>
      <p className="ui-catalog__paragraph-spaced">
        Texto de párrafo normal. <strong>Negrita.</strong> <em>Itálica.</em>{' '}
        <a href="#" onClick={(e) => e.preventDefault()}>Enlace.</a>{' '}
        <code className="ui-catalog__code-inline">código inline</code>
      </p>
      <blockquote className="ui-catalog__blockquote">
        Cita de ejemplo — "La simplicidad es la máxima sofisticación."
      </blockquote>
    </div>
  );
}

function ListsSection() {
  const items = ['Elemento activo', 'Segundo elemento', 'Tercer elemento', 'Cuarto elemento'];
  return (
    <div className="card">
      <div className="ui-catalog__list-box">
        {items.map((item, i) => (
          <div key={item} className={`ui-catalog__list-row ${i === 0 ? 'ui-catalog__list-row--active' : ''}`}>
            {item}
          </div>
        ))}
      </div>
    </div>
  );
}

function VideosSection() {
  return (
    <div className="card">
      <div className="ui-catalog__video-wrap">
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
    <PageLayout
      className="ui-catalog"
      title="Catálogo de componentes"
      lead="Patrones de UI y tokens del producto para desarrollo y QA."
    >
      <div className="ui-catalog__tabs">
        {TABS.map((t) => (
          <button key={t.id} type="button" className={`ui-catalog__tab ${tab === t.id ? 'ui-catalog__tab--active' : ''}`} onClick={() => setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>

      <Content />
    </PageLayout>
  );
}

export default UIComponentsPage;
