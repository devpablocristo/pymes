# UI System — Pymes

Documento de referencia del design system que vive en `ui/src/`. Resultado de la migración Wooko → Pymes (rama `ui-enano`, ver `.claude/plans/tengo-un-bug-en-melodic-river.md` para detalle por fase).

## TL;DR

- **Tokens**: CSS variables en `ui/src/styles/tokens.css`. Light + dark, namespace `pymes-ui`, storage key `pymes:theme`.
- **Fuentes**: Plus Jakarta Sans (body) + JetBrains Mono (mono), cargadas via `@fontsource/*`.
- **Iconos**: `@tabler/icons-react` (npm, tree-shakeable). El sidebar mantiene su set de SVG inline custom (`ShellIcons.tsx`) por decisión de producto previo.
- **Layout**: shell card flotante (sidebar margenes 16/radius 18/shadow z1). Topbar azul + barra blanca de acciones.
- **Componentes shared**: `<ThemeToggle />`, `<EmptyState />`, `<Skeleton />`, `<NotificationsDropdown />`.
- **Reglas**: usar tokens (`var(--color-*)`, `var(--text-*)`, `var(--space-*)`, `var(--radius-*)`, `var(--shadow-*)`); no hardcodear; no introducir Tailwind ni CSS Modules.

## Tokens

Definidos en `ui/src/styles/tokens.css`. Para usar en componentes nuevos: referenciar siempre la variable, nunca el literal.

### Colores

| Token | Light | Rol |
|---|---|---|
| `--color-bg` | `#f0f5f9` | Body background |
| `--color-surface` | `#ffffff` | Cards, modals, drawers |
| `--color-surface-hover` | `#f8fafc` | Hover state de surface |
| `--color-border` | `#e6ecf2` | Borders default |
| `--color-border-subtle` | `#eef2f7` | Borders secundarios |
| `--color-sidebar` | `#ffffff` | Sidebar bg |
| `--color-sidebar-text` | `#5a6a85` | Sidebar texto inactive |
| `--color-sidebar-text-active` | `#0085db` | Sidebar texto activo |
| `--color-sidebar-active` | `#e6f4ff` | Sidebar item activo bg |
| `--color-text` | `#2a3547` | Texto principal |
| `--color-text-secondary` | `#5a6a85` | Texto secundario |
| `--color-text-muted` | `#8898aa` | Texto deshabilitado / placeholders |
| `--color-primary` | `#0085db` | Primary action |
| `--color-primary-hover` | `#006abf` | Primary hover |
| `--color-primary-subtle` | `#e6f4ff` | Primary fondo (badge, estado) |
| `--color-success` | `#4bd08b` | Success |
| `--color-warning` | `#f8c076` | Warning |
| `--color-danger` | `#fb977d` | Danger |
| `--color-on-primary` | `#f8fafc` | Texto sobre primary (botones) |

Los valores dark están en el bloque `[data-theme="dark"]` del mismo archivo y se aplican automáticamente.

### Tipografía

- `--font-body`: `'Plus Jakarta Sans', system-ui, sans-serif`
- `--font-mono`: `'JetBrains Mono', 'Fira Code', monospace`
- Pesos: `--font-weight-{light|normal|medium|semibold|bold}` (300–700).
- Escala: `--text-{2xs|xs|sm|base|md|lg|xl|2xl|3xl}` (0.65rem → 2rem).
- Roles semánticos (preferir sobre escala cruda):
  - `--text-role-page-title` → `--text-xl`
  - `--text-role-section` → `--text-md`
  - `--text-role-body` → `--text-base`
  - `--text-role-caption` → `--text-xs`
  - `--text-role-micro` → `--text-2xs`

### Spacing y radius

- Spacing scale 8px: `--space-{1..12}` (4px → 96px).
- Radius: `--radius-sm` 8 / `--radius-md` 14 / `--radius-lg` 18 / `--radius-btn` 30 / `--radius-badge` 50.

### Sombras y transiciones

- Elevación: `--shadow-{z1|z4|z8|z12|z16|z20|z24}`. En light tienen tinte azul (`rgba(37, 83, 185, *)`); en dark son neutras y más profundas.
- Coloreadas: `--shadow-{primary|success|warning|danger}` para hover/focus de botones de color.
- Transición default: `--transition` (`150ms cubic-bezier(0.4, 0, 0.2, 1)`).

## Fuentes

Cargadas en `main.tsx` via `@fontsource/*`:

```ts
import '@fontsource/plus-jakarta-sans/300.css';
import '@fontsource/plus-jakarta-sans/400.css';
import '@fontsource/plus-jakarta-sans/400-italic.css';
import '@fontsource/plus-jakarta-sans/500.css';
import '@fontsource/plus-jakarta-sans/600.css';
import '@fontsource/plus-jakarta-sans/700.css';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/500.css';
```

Bundler procesa los archivos como CSS separado — no hay render-blocking ni dependencia de CDN.

## Dark mode

- Activación: clic en el botón con icono ☾/☀ del footer del sidebar (`<ThemeToggle />`).
- Persistencia: `localStorage('pymes-ui:pymes:theme')`.
- Fallback inicial: `prefers-color-scheme`.
- Implementación: el theme manager (`@devpablocristo/core-browser/theme`, instanciado en `ui/src/lib/theme.ts`) setea `data-theme="light|dark"` en `<html>`.
- El sidebar respeta los tokens dark via override explícito en `shell-sidebar.css` (`background: var(--color-sidebar) !important`) — la lib externa hardcodeaba blanco.

Para forzar el tema desde DevTools:

```js
localStorage.setItem('pymes-ui:pymes:theme','dark'); location.reload();
localStorage.setItem('pymes-ui:pymes:theme','light'); location.reload();
```

## Componentes shared nuevos

### `<ThemeToggle />`

Botón compacto que alterna light ↔ dark. Montado por defecto en el footer del sidebar (`Shell.tsx`). Estilo neutro 36x36 cuando está en sidebar; adopta estilo blanco semi-transparente cuando se monta en `.page-layout__header-top-row` (topbar de página).

```tsx
import { ThemeToggle } from '../components/ThemeToggle';
<ThemeToggle />
```

### `<EmptyState />`

Placeholder para listas/vistas sin datos. Usa `.empty-state` CSS.

```tsx
import { EmptyState } from '../components/EmptyState';

<EmptyState
  title="Sin clientes todavía"
  description="Creá el primero o importá un CSV."
  icon={<IconUsersOff size={48} stroke={1.5} />}
  action={<button className="btn-primary">Nuevo cliente</button>}
/>
```

### `<Skeleton />`

Shimmer loading. Variants `text` (con `lines`), `rect`, `circle`. Width/height configurable.

```tsx
import { Skeleton } from '../components/Skeleton';

<Skeleton variant="text" lines={3} />
<Skeleton variant="circle" width={48} height={48} />
<Skeleton variant="rect" width="100%" height={180} />
```

### `<StatCard />`

KPI card reusable para dashboards. Estilo unificado con `.dash__stat-*`. Tonos: `blue | green | purple | red | amber`.

```tsx
import { StatCard } from '../components/StatCard';

<StatCard label="Pacientes" value="124" sub="este mes" tone="blue" />
<StatCard label="Ventas" value={total} tone="green" loading={isLoading} />
```

A11y: `role="group"` + `aria-label={label}`, icono decorativo `aria-hidden`.

### `<NotificationsDropdown />`

Campana con panel desplegable. Embebe `NotificationsCenterPage` en modo `embedded`. Conteo unread con badge, refetch 60s, cierra con click fuera o Escape.

```tsx
import { NotificationsDropdown } from '../components/NotificationsDropdown';
<NotificationsDropdown />
```

**No está montado por default** — disponible para superficies como topbar de página (`.page-layout__header-top-row`) o cualquier slot custom. Reemplaza al ítem `/notifications` del sidebar nav cuando el equipo decida la migración.

## Estructura de archivos CSS

```
ui/src/
  styles.css                       # entry point: importa los demás
  styles/
    tokens.css                     # variables CSS (light + dark)
    base.css                       # reset + tipografía base
    utilities.css                  # .u-mb-md, .text-muted, etc.
    sections.css                   # layouts de secciones
    shell-sidebar.css              # override del sidebar de la lib externa
    shell-topbar.css               # override del topbar de crud-page-shell
    components.css                 # botones, inputs, cards, modals, tabs, etc.
    auth.css                       # login / signup
    onboarding.css                 # wizard 4-step (gradient + dots)
    profile.css                    # settings / perfil
    admin-crud-theme.css           # tema CRUD admin/tenant/API keys
    module-page.css                # page templates
    weekly-calendar.css            # weekly calendar workspace
    calendar-workspace.css         # calendar shared
    viewModeSegmentedSwitch.css    # switch de vista list/board/gallery
```

Pages y modules tienen su propio CSS adyacente al TSX cuando se necesita styling específico (e.g. `pages/InventoryPage.css`, `modules/crud/CrudEntityEditorModal.css`).

## Reglas para CSS nuevo

1. **Siempre usar tokens.** Nada de literales hex/rgba sueltos. Si falta un valor, agregarlo a `tokens.css` antes de usarlo.
2. **No hardcodear tipografía.** Usar `var(--text-role-*)` o la escala `var(--text-*)`.
3. **No introducir Tailwind ni CSS Modules.** El sistema es CSS variables + custom properties.
4. **Override de librerías externas con `!important` controlado.** Mantener overrides en archivos dedicados (`shell-*.css`) para fácil revert.
5. **Dark mode primero.** Cualquier color/sombra/border debe tener su versión en el bloque `[data-theme="dark"]`. Si solo agregás light, romp dark.
6. **Animaciones via `--transition`** (no inventar duraciones nuevas salvo justificación).
7. **Componente nuevo > clase suelta.** Si una pieza visual se va a usar en >2 lugares, hacer un componente React reusable.

## Iconografía

- **Tabler React** (`@tabler/icons-react`) para iconos nuevos. Importar puntual: `import { IconBell, IconX } from '@tabler/icons-react'`. Tree-shake automático.
- **`ShellIcons.tsx`** (SVG inline custom) — sidebar y wrappers que ya existían. Decisión de producto: todos los items del sidebar usan `dotIcon` (círculo simple); no diferenciar por icono.
- **NO Tabler webfont CDN.** Wooko original cargaba `https://cdn.jsdelivr.net/.../tabler-icons-webfont` — descartado por dependency-on-runtime.

## Estado de la migración

Ver `.claude/plans/tengo-un-bug-en-melodic-river.md` (sección 5 / 6 / 11) para el roadmap completo. Por phase:

| Phase | Estado |
|---|---|
| 0 — Setup (deps, gitignore, wooko-diff.sh) | ✅ |
| 1 — Foundations (tokens + fuentes + dark mode + ThemeToggle) | ✅ |
| 2 — Shell (sidebar card + topbar split) | ✅ |
| 3 — UI kit + reskin masivo CSS (12 archivos) | ✅ |
| 4 — Iconos (no-op: ShellIcons custom + Tabler para nuevos) | ✅ |
| 5/6/8 — Auth/Onboarding/Settings CSS | ✅ (cubierto en 3) |
| 7 — Dashboard StatCard + montaje ThemeToggle | ✅ (StatCard extraído a `components/StatCard.tsx`) |
| 9 — EmptyState + Skeleton components | ✅ |
| 10 — Módulos verticales (CSS swap) | ✅ (CSS); TSX-side review pendiente |
| 11 — Cherry-pick (NotificationsDropdown + Reports + SettingsMenu) | ✅ |
| 12 — Animaciones + microinteracciones | ✅ (tokens + transitions ya en CSS) |
| 13 — A11y baseline | ✅ baseline (StatCard semantics, focus contrast en topbar). Lighthouse run + axe-core en Playwright pendientes para CI. |
| 14 — Cleanup + este doc | ✅ |

## Convenciones de bug-fixing visual

Si encontrás un componente roto post-migración:

1. Confirmá que el problema es **CSS** y no lógica (chequeá Network: la data debe llegar OK).
2. Usá `scripts/wooko-diff.sh` para ver qué cambió respecto al fork de Wooko:
   ```bash
   scripts/wooko-diff.sh src/pages/InventoryPage.css
   ```
3. Si el TSX divergió y el CSS de Wooko apunta a un DOM que el actual no tiene, revertí solo ese CSS al estado pre-swap:
   ```bash
   git show <commit-anterior>:ui/src/pages/X.css > ui/src/pages/X.css
   ```
4. NO copiar TSX de Wooko sin verificar imports — la mayoría depende de hooks/types que ya no existen.

## Referencias

- Plan canónico: `.claude/plans/tengo-un-bug-en-melodic-river.md`
- Helper diff: `scripts/wooko-diff.sh`
- Theme manager: `@devpablocristo/core-browser/theme` (instancia en `ui/src/lib/theme.ts`)
- I18n keys del toggle: `shell.theme.light` / `shell.theme.dark` en `ui/src/lib/i18n/messages/shell.ts`
