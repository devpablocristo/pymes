# Plan de recuperación del frontend Pymes

Documento de auditoría, arquitectura objetivo, fases de migración, tradeoffs explícitos y estado de ejecución.  
**Basado en el código real** del directorio `frontend/` (React 18, Vite 6, TypeScript).  
**Última actualización:** 2026-04-02.

---

## Tradeoffs explícitos (leer primero)

| Decisión | Qué se eligió | Qué **no** se hizo y por qué |
|----------|----------------|------------------------------|
| **Estilos** | Mantener **design tokens en CSS global** (`styles.css`, `:root`, `[data-theme="dark"]`) + hojas `.css` por página + reducción gradual de estilos inline. | **No** introducir Tailwind, CSS-in-JS (Emotion/styled-components), ni PostCSS “de cero” en esta fase: ya hay ~3300 líneas y variables en `styles.css`; añadir otro sistema aumentaría paradigmas mezclados y costo de migración sin eliminar el legado. |
| **CSS Modules** | Opcional **futuro**, archivo por archivo cuando una pantalla se refactorice. | **No** migración masiva a `.module.css` ahora: alto riesgo de regresiones visuales y bajo retorno inmediato frente a ordenar tokens y convenciones. |
| **Datos** | Dirección: **React Query** como estándar para datos remotos (ya está `QueryClient` en `main.tsx`). | **No** reescribir todas las pantallas de golpe: hoy gran parte usa `api.ts` + efectos o el motor CRUD; migración incremental por feature. |
| **Monorepo modules** | Seguir consumiendo `@devpablocristo/modules-*` (shell, CRUD, kanban, calendar, etc.) como capa de UI compartida. | **No** duplicar esos componentes en el repo salvo divergencia de producto justificada. |
| **Big-bang** | Recuperación **incremental** (tokens, `lang`, páginas calientes, tests). | **No** reescritura completa del frontend: el costo y el riesgo de negocio superan el beneficio con el estado actual auditado. |

**Resumen en una línea:** se prioriza **coherencia y mantenibilidad** con el stack existente (CSS global con tokens + módulos npm) y **refactors seguros por capas**, en lugar de adoptar herramientas de moda que multiplicarían superficies de estilo sin borrar la deuda acumulada.

---

## 1. Executive Summary

**Condición del codebase:** consola React 18 + Vite 6 con rutas mayormente `lazy`, shell y CRUD apoyados en `@devpablocristo/modules-*`, y un `styles.css` global muy grande que concentra tokens, utilidades y reglas de producto. No hay Tailwind, CSS Modules ni styled-components en este paquete (verificación por búsqueda en el repo).

**Problemas principales:** (1) Mezcla de paradigmas de datos — React Query presente pero poco usado fuera de dashboard / dashboard visual. (2) Estilos: global monolítico + CSS por página + muchos `style={{}}` inline — no es un design system completo propio. (3) Límites de features difusos: `pages/` mezcla producto y demos (`CryptoPage`, `UIComponentsPage`). (4) Tests: pocos archivos; cobertura acotada (Vitest en verde tras fixes de búsqueda CRUD y asincronía en tests Clerk).

**Riesgos:** acoplamiento a clases globales; fetch/cache aún heterogéneo fuera de pantallas migradas a React Query; a11y no auditada de punta a punta (mejoras incrementales: `lang="es"`, skip link, landmark principal).

---

## 2. Current Architecture (evidencia en repo)

| Área | Ubicación |
|------|-----------|
| Entrada | `src/main.tsx` — ErrorBoundary, QueryClient, LanguageProvider, BrowserRouter, App; Clerk opcional; `applyTheme` / `applyAdminSkin`; `import './styles.css'`. |
| Rutas | `src/app/App.tsx` — lazy masivo; `Shell` con rutas anidadas; `RequireOnboarding`, `ProtectedRoute`. |
| Shell | `src/components/Shell.tsx`, `src/shared/frontendShell.tsx` — `@devpablocristo/modules-shell-sidebar`. |
| CRUD | `src/components/CrudPage.tsx` — adaptador a `modules-crud-ui`; `src/crud/lazyCrudPage.tsx`, `resourceConfigs.*.tsx`. |
| API | `src/lib/api.ts` — `@devpablocristo/core-authn/http/fetch`. |
| Tema | `src/lib/theme.ts` — `data-theme` en `documentElement`. |
| Build | `vite.config.ts` — `manualChunks` (clerk, calendar, kanban, crud, etc.). |

**Estilo implícito:** hoja global + CSS por ruta + inline; módulos externos traen su CSS.

---

## 3. Styling System Analysis

- **Global:** `src/styles.css` — fuentes Google, `:root` / dark, `--text-*`, `--font-weight-*`, `--space-*`, `--color-*`, utilidades (`.btn-*`, `.card`, …).
- **Por página:** ~16 `*.css` bajo `src/` importados desde TSX.
- **Inline:** uso extendido en páginas como `UIComponentsPage.tsx`, `DashboardVisualPage.tsx`, `SettingsHubPage.tsx` (riesgo de inconsistencia).
- **Sin** CSS Modules / Tailwind en este frontend.

**Problemas:** monolito CSS difícil de navegar; inline dificulta tema y consistencia; tema por variables **sí es viable** y ya está parcialmente implementado.

---

## 4. Main Structural Problems

1. `App.tsx` como registro muy largo de `lazy()`.
2. React Query infrautilizado respecto al `QueryClient` global.
3. CRUD partido en muchos `resourceConfigs` con code-split manual (escalable pero cognitivamente pesado).
4. Rutas demo/producto mezcladas.

---

## 5. Main Maintainability Problems

- CSS global grande + especificidad.
- Inline styles dispersos.
- Cobertura de tests baja; regresiones se detectan con Vitest (25 tests al día de la última verificación).

---

## 6. UI/UX Consistency

- Tokens existen; adopción heterogénea en páginas con inline.
- Estados vacío/carga varían por pantalla.

---

## 7. Performance

- Code-splitting en `App.tsx` y Vite — positivo.
- Dependencias pesadas (Clerk, FullCalendar) — mitigadas por chunks.

---

## 8. Accessibility

- `lang` del documento alineado con contenido en español (`index.html` → `lang="es"`).
- Falta auditoría sistemática de roles/foco/contraste en componentes custom.

---

## 9. Dependencies / Tooling

- Node ≥20, Vite 6, TS 5.7 — razonable.
- **ESLint 9** (`eslint.config.js`): `typescript-eslint` + `react-hooks` + `react-refresh`; `react-hooks/exhaustive-deps` desactivado en legado (revisar al tocar efectos). `npm run lint`; Docker: `make lint-docker-frontend` (tras `docker compose build frontend` si cambian devDeps).
- Vitest + Playwright (e2e excluido del run unitario por config).

---

# Target Architecture

## 10. Recommended Frontend Architecture

Mantener Vite + React + RR6 + adaptadores a `modules-*`. Evolucionar hacia:

- `app/`: bootstrap y rutas (ya existe `src/app/`).
- `features/<name>/`: opcional cuando una pantalla crezca (no obligatorio desde día 1).
- **Un enfoque dominante de datos:** React Query para servidor; `api.ts` como capa fina.

## 11. Recommended Styling Architecture (este repo)

**Una estrategia:** tokens en `:root` + CSS por página + menos inline; **no** añadir Tailwind/CSS-in-JS en esta fase (ver tabla de tradeoffs arriba).

## 12. Recommended Folder Structure (evolutivo)

```
src/
  app/           # App, rutas
  components/    # UI compartida
  pages/         # rutas (→ features/ gradual si aplica)
  crud/          # resourceConfigs, lazyCrudPage
  lib/           # api, i18n, theme, auth
  shared/        # frontendShell, frontendAuth
  styles.css     # tokens + globales
```

## 13. Conventions

- HTTP vía `lib/api` / patrones existentes.
- Estilos nuevos: variables primero.
- Estado servidor: preferir React Query en código nuevo.
- Páginas **custom** (no CRUD): usar `PageLayout` (`src/components/PageLayout.tsx`) para `page-stack` + cabecera (`title`, `lead`, `actions` opcional); el buscador del shell sigue con `usePageSearch()` en la página.

---

# Migration Plan

## 14. Refactor Phases

| Fase | Objetivo | Riesgo | Incremental |
|------|----------|--------|-------------|
| 1 | Documentar tokens y reglas en CSS | Bajo | Sí |
| 2 | `lang`, a11y básicos | Bajo | Sí |
| 3 | Inline tipográfico → tokens en páginas calientes | Bajo | Sí |
| 4 | Unificar fetch con React Query | Medio | Sí |
| 5 | Partir `styles.css` (p. ej. `tokens.css`) | Medio | Sí |
| 6 | Arreglar test CrudPage / contrato PageSearch | Medio | Sí |

## 15. Quick Wins

- Comentario de arquitectura en cabecera de `styles.css`.
- `lang="es"` en `index.html`.
- Pesos con `var(--font-weight-*)` en `DashboardVisualPage.tsx` y otras páginas ya tocadas.
- Seguir migración inline → tokens en Settings/Invoices/UI demo.

## 16. High-Risk Areas

- Cambios masivos en `styles.css`.
- Rutas anidadas en `App.tsx`.
- Integración CrudPage + PageSearch + tests.

---

# Execution (registro)

## 17. Cambios ya aplicados (sesión de recuperación)

- `index.html`: `lang="es"`.
- `src/styles.css`: comentario de arquitectura al inicio del archivo.
- `src/pages/DashboardVisualPage.tsx`: `fontWeight` numérico → `var(--font-weight-semibold)` / `var(--font-weight-bold)`.
- (Sesión anterior relacionada) tokens tipográficos en `styles.css`, `PageSearch.css`, `UnifiedChatPage.css`, `SettingsHubPage`, `InvoicesPage`, `main`, `CryptoPage`, `UIComponentsPage`.
- **`CrudPage` + `PageSearch` (fase “comenzar migración”):** el motor `@devpablocristo/modules-crud-ui` solo muestra el input de búsqueda **inline** cuando `externalSearch == null`. El wrapper `CrudPage` pasaba siempre `externalSearch={pageSearch}` (string), así que en tests **sin** `PageSearchProvider` no había ningún `<input>` de búsqueda. Se añadió `PageSearchShellContext` (true solo dentro de `<PageSearchProvider>` en `Shell`) y `externalSearch={pageSearchInShell ? pageSearch : undefined}` para que en consola real se use el buscador del shell y fuera de él (tests, Storybook futuro) el placeholder i18n del CRUD (`Search items...`, etc.).
- **`src/styles/tokens.css`:** tokens (`:root`, `[data-theme="dark"]`, fuentes) extraídos; `styles.css` importa `@import './styles/tokens.css'` y conserva reset + utilidades + reglas de producto.
- **`SettingsHubPage.tsx`:** deep links y automation hub usan `var(--space-*)`, `var(--color-border)` y `var(--font-weight-semibold)` en tablas; menos literales sueltos.
- **Ronda siguiente:** `AdminPage` (cabecera auditoría), `UnifiedChatPage` (TSX + `UnifiedChatPage.css` pesos), `AdminRbacSection` (márgenes / `minHeight` con `calc(var(--space-4) * 6)`), `InvoicesPage` (padding, gaps, pesos en tablas y formulario).
- **Pesos en CSS global y por página:** `font-weight: 600` / `700` reemplazados por `var(--font-weight-semibold)` / `var(--font-weight-bold)` en `styles.css`, `InvoicesPage.css`, `CalendarPage.css`, `DashboardVisualPage.css`, `CryptoPage.css`, `SettingsHubPage.css`, `UIComponentsPage.css`, `WorkOrdersModuleSection.css`, `WorkOrderKanbanDetailModal.css`, `ApprovalInboxPage.css`, `WatcherConfigPage.css`, `AutomationRulesPage.css`, `WorkOrdersKanbanPanel.css`.
- **`DashboardVisualPage.tsx`:** `marginBottom` residual `0.25rem` → `var(--space-1)`.
- **`UIComponentsPage.tsx`:** gaps/márgenes/padding inline del catálogo → `var(--space-*)`; `code` demo con `fontSize: var(--text-sm)`.
- **`SettingsPage.clerk.test.tsx`:** `waitFor` hasta que aparezca el nombre de Clerk (evita falso rojo si la UI sigue en “Cargando...” un tick después de resolver `getMe`).
- **`SettingsPage.test.tsx` (modo API key):** mismos criterios — esperar `getMe` y contenido (`Fábrica Norte`, heading “Cuenta”) con `waitFor` para no competir con el estado “Cargando...”.
- **`font-weight: 500` → `var(--font-weight-medium)`** en `styles.css` y CSS de `InvoicesPage`, `UIComponentsPage`, `ApprovalInboxPage`, `WatcherConfigPage`, `AutomationRulesPage`.
- **`CryptoPage.tsx`:** `gap` en barra de asignación → `var(--space-2)`.
- **`UnifiedChatPage.css`:** divisor de conversaciones — `margin-top` y padding lateral/superior con tokens (`0.35rem` bottom se mantiene por ajuste fino).
- **`AutomationRulesPage.css`:** padding en `select` e inputs de umbral → `var(--space-*)`.
- **`tokens.css`:** `--space-12: 3rem` para bloques tipo empty/loading (~48px).
- **`styles.css`:** `.endpoint-path` usa `var(--font-weight-normal)` en lugar de `400`.
- **`WatcherConfigPage.css` / `ApprovalInboxPage.css`:** espaciado `px` → `var(--space-*)` y `var(--space-12)` en estados vacíos/carga.
- **`UnifiedChatPage.css`:** badges, submeta, acciones de bloque, highlights KPI, celdas de tabla → tokens de espacio.
- **`UIComponentsPage.tsx`:** padding de chips/tags y `code` demo con `var(--space-*)`.
- **Ronda espaciado en CSS de producto:** `InvoicesPage.css`, `WorkOrdersKanbanPanel.css`, `SettingsHubPage.css` (nav/toggle desc + tabla CRUD), `CryptoPage.css`, `WorkOrdersModuleSection.css` (switch tablero/lista), `DashboardVisualPage.css` (KPI, barras, tablas, performers) — `rem`/`px` sueltos → `var(--space-*)` donde encaja la escala.
- **`WorkOrderKanbanDetailModal.css`:** backdrop, header/body/footer, grid, campos, inputs y botones — `rem` sueltos → `var(--space-*)`.
- **`DashboardVisualPage.css`:** sparkline CSS-only — `gap` entre barras → `var(--space-1)`.
- **`PageSearch.css`:** márgenes/padding del input y del botón clear → `var(--space-*)` (se mantiene `2rem` a la derecha del input para el área del icono).
- **`WorkOrderEditor.css`:** padding inferior del contenedor y margen del toolbar → tokens.
- **`UIComponentsPage.css`:** gap entre estrellas del rating → `var(--space-1)`.
- **`styles.css` (bloque shell / cabecera CRUD):** gaps y padding del selector de idioma y cabeceras `crud-page-shell` → `var(--space-*)`.
- **`styles.css` (bloque producto base):** stat cards, tablas globales, formularios, botones, badges, `pre`/`code`, perfil (`.profile-*`), empty state, auth card, plans grid — sustitución de `rem` repetidos por `var(--space-*)` y `calc(var(--space-4) + var(--space-1))` donde hacía falta **1.25rem**; avatares `4rem` y anchos `28rem`/`minmax` se mantienen como layout fijo.
- **`styles.css` (ronda 2026-04-02):** se eliminó el **segundo** bloque duplicado `.empty-state` y se consolidó en el primero (`padding` con tokens; `p` con `margin-bottom` y color secundario). Onboarding (compact, chips, summary, footer, botones, media query), calendario semanal (toolbar, stats, cabeceras de día, eventos, footer), bloque **CRUD** (toolbar, búsqueda, formularios, acciones de fila, confirm delete), **admin** (monedas, settings, textarea, actividad, chat log) y **API keys** (fieldset, filas de scopes, acciones) — espaciados y radios alineados a `var(--space-*)`, `var(--radius-sm|md)` y `calc` donde aplica.
- **CSS por página / componente (misma línea):** `PageSearch.css` (padding derecho `var(--space-6)`, `margin-left` del clear con `calc`, `border-radius` con `var(--radius-sm)`), `WorkOrderEditor.css` y `WorkOrderKanbanDetailModal.css` (dimensiones y `min-height` con `calc` sobre tokens), `WorkOrdersModuleSection.css`, `DashboardVisualPage.css` (`.dash__progress-pct`), `AutomationRulesPage.css` (anchos/mínimos con `calc(var(--space-4) * n)` o `var(--space-5) * 7`), `UnifiedChatPage.css` (padding inferior del divisor con `calc(var(--space-2) * 0.7)` ≈ 0.35rem), `styles.css` (avatar perfil y columnas admin monedas).
- **`styles.css` sin literales `*rem` en tamaños:** última pasada — selector de idioma, búsqueda en cabecera CRUD, filas de perfil, card auth, badge swatch, `clamp` del hero dashboard, grillas onboarding (incl. compact y media query), `--weekly-cal-header-height`, columna acciones CRUD, log comercial; expresiones `calc(var(--space-4) * n)`, `calc(var(--space-5) * k)` y `calc(var(--space-12) * 0.45)` donde aplica. `AutomationRulesPage.css`: `select.rule-card` alineado con `calc(var(--space-5) * 7)`.
- **Inline TSX → tokens (2026-04-02):** `DashboardVisualPage.tsx` (spinner, altura de barras cashflow, barra de color en citas), `UIComponentsPage.tsx` (alturas de demo, dots del carousel, `minmax` en grillas, dropdown y select), `CryptoPage.tsx` (barra de asignación y `minWidth` del `%`), `InvoicesPage.tsx` (`maxWidth` de columnas del ítem). Convención: `calc(var(--space-4) * N / 15)` para equivalente a **N px** con `html { font-size: 15px }`; `calc(var(--space-4) * 8)` para 120px (8×15).
- **Seguimiento inline:** `UIComponentsPage.tsx` — carousel con `height: calc(var(--space-4) * 12)` (180px), chips/tags `borderRadius: '999px'`, `code` y `blockquote` con `var(--radius-sm)` y borde izquierdo vía `calc(var(--space-4) * 4 / 15)`. `main.tsx` (ErrorBoundary DEV): `<pre>` del stack con `var(--space-4)`, `var(--radius-md)` y `maxHeight: calc(var(--space-4) * 250 / 15)`.
- **Tokens + CSS para errores DEV / acento chat:** `tokens.css`: `--color-accent-indigo`, `--color-dev-error-pre-bg|fg` (claro; en oscuro variantes más legibles). `styles.css`: clase `.error-boundary-fallback__dev-pre` (incl. `font-mono`). `main.tsx`: sin inline en el `<pre>`. `CryptoPage.tsx`: sparkline y estrella con `var(--color-success|danger|warning)`. `UnifiedChatPage.tsx` y `resourceConfigs.beauty.tsx`: índigo vía `var(--color-accent-indigo)`.
- **Showcase UI + chat (paleta):** `tokens.css`: `--color-accent-pink`, `--color-accent-cyan` (tema oscuro con tonos afinados). `UIComponentsPage.tsx`: `PRODUCT_PALETTE` (`token` / `label`); gradientes, carousel, avatares, badges/tags con `color-mix`, barras de progreso y texto sobre slides con `var(--color-on-primary)`. `UnifiedChatPage.tsx`: contactos demo humanos con colores por token.
- **`SettingsHubPage.tsx` (ThemeTab):** `themeHubColorSwatches()` desde `productPalette` (ids `primary`, `success`, … + `PRODUCT_PALETTE_LABELS_ES`); sin persistencia API en este bloque.
- **Ronda “todo” (calendario + utilidades + CRUD beauty):** `CalendarPage.tsx`: `CALENDAR_COLOR_OPTIONS` con `hex` (API/FullCalendar) + `swatch` (tokens en puntitos); `DEFAULT_APPOINTMENT_COLOR_HEX` centralizado. `styles.css`: utilidades `.u-m-0`, `.u-pre-wrap`, `.u-text-base`. `NotificationsCenterPage.tsx` / `SettingsHubPage.tsx`: párrafos sin inline de margen/tipo. `resourceConfigs.beauty.tsx`: placeholder del color de agenda alineado a tokens. `CryptoPage.tsx`: comentario de archivo sobre colores de marca en demo.
- **Paleta DRY:** `src/lib/productPalette.ts` — `PRODUCT_PALETTE` (id, label, hex, token), `DEFAULT_APPOINTMENT_COLOR_HEX`, `CALENDAR_APPOINTMENT_COLOR_OPTIONS`. Consumen `CalendarPage` y `UIComponentsPage` (showcase); una sola fuente para hex + tokens.
- **CSS utilidades:** `src/styles/utilities.css` (`.u-*`) importado desde `styles.css` tras `tokens.css`.
- **ESLint + hooks:** corrección de hooks condicionales en `AutomationRulesPage` y `WatcherConfigPage` (búsqueda siempre tras hooks); regex `downloadAPIFile` en `api.ts`; `CryptoPage` toggle sin expresión suelta; imports limpios (`App.tsx` sin lazy `DashboardPage` duplicado, `csvToolbar` / `commercial` / talleres / `aiApi`). `AccountPlanSection`: directiva eslint obsoleta eliminada.
- **Tests:** `src/lib/productPalette.test.ts` (Vitest).
- **React Query + claves:** `src/lib/queryKeys.ts`; `CalendarPage` usa `useQuery` para `/v1/appointments` e invalidación tras crear/editar/archivar; `AdminPage` usa `useQuery` para tenant settings, auditoría y sesión (`staleTime` acotado), `setQueryData` tras guardar.
- **Rutas:** `App.tsx` aligerado — `lazyRoutes.tsx` (todos los `lazy()`), `ShellRoutes.tsx` (rutas anidadas del shell), `suspended.tsx`; comentario en `ShellRoutes` para rutas demo (`/crypto`, `/ui`).
- **a11y (incremental):** enlace **“Saltar al contenido principal”** (`skip-link` en `base.css`) + `<main id="main-content">` en `Shell`; modal de calendario con `role="dialog"`, `aria-modal`, `aria-labelledby`; indicador de guardado con `role="status"` / `aria-live="polite"`; `ErrorBoundary` en español con `role="alert"`.
- **Pulido previo:** `WorkOrdersModuleSection` (thumb), `resourceConfigs.beauty` (`--badge-swatch-bg`), `transversalWidgets` (`--widget-bar-width`), `WidgetFrame` (grid por CSS vars).

## 18. Archivos tocados en esa línea de trabajo

Ver historial git; incluyen entre otros: `frontend/index.html`, `frontend/src/styles.css`, `frontend/src/styles/tokens.css`, `frontend/src/pages/DashboardVisualPage.tsx`, `frontend/src/pages/SettingsHubPage.tsx`, `frontend/src/components/PageSearch.tsx`, `frontend/src/components/CrudPage.tsx`, y los listados en commits de normalización tipográfica.

## 19. Verificación

- Con stack Docker (`make up`): `make test-docker-frontend`, `make build-docker-frontend`, `make lint-docker-frontend` — OK (Vitest incl. `productPalette.test.ts`, `tsc && vite build`, `eslint .` sin errores). Tras añadir devDeps: `docker compose build frontend`.
- En host (respaldo): `npm run build`, `npm run test`, `npm run lint` en `frontend/`.

---

# Final Assessment

## 20. What Improved

- Documentación **en el propio** `styles.css` sobre el modelo de estilos.
- Mejor coherencia **documento** ↔ idioma de UI.
- Menos números mágicos de peso en dashboard visual.
- **Búsqueda CRUD:** tests y entornos sin Shell vuelven a ver el input inline del módulo; con Shell se mantiene el buscador global. CI verde (Vitest en `frontend/`).
- **Tokens en archivo dedicado:** edición de color/tipo/tema sin abrir el monolito completo de reglas de componentes.
- **Ajustes (hub):** más uso de variables de espacio y peso en deep links y tablas de ejemplo.

## 21. Remaining Debt (actualizado 2026-04-02)

- **CSS global:** `styles.css` es solo **barrel de `@import`** (tokens, utilities, sections, base, shell, perfil, onboarding, weekly-calendar, auth, dashboard, admin-crud-theme, components, module-page). El “monolito” de reglas está repartido en esos archivos; seguir extrayendo si crece una sección concreta.
- **React Query:** hay **patrón estable** (`src/lib/queryKeys.ts`) y uso en **Calendar** (lista de turnos) y **Admin** (tenant, auditoría, sesión). Otras pantallas con `useEffect` + fetch pueden migrarse por feature.
- **Inline dinámico:** en páginas calientes quedan sobre todo **variables CSS / %** (dashboard, showcase UI, crypto, etc.), no deuda de “márgenes sueltos”.
- **ESLint:** está en el paquete (`eslint.config.js`, `npm run lint`, `make lint-docker-frontend`).

Pendiente de **alcance mayor** (no cerrado en esta recuperación):

- **Auditoría a11y completa:** revisar *todo* el producto con criterio sistemático (WCAG, teclado, lector de pantalla, contraste). Lo hecho hasta ahora son **mejoras puntuales** (skip link, `main`, modal calendario, etc.), no un certificado ni un checklist cerrado.
- **React Query en el resto de pantallas:** migrar cada `useEffect` + `fetch` que quede a `useQuery` / mutaciones donde aplique; Calendar y Admin ya usan el patrón con `queryKeys`.
- **E2E Playwright en CI:** tener pruebas de navegador automáticas en el pipeline (build → tests unitarios → e2e); el repo puede tener Playwright en config pero **no sustituye** definir flujos, mantenerlos y ejecutarlos en CI.

Separación estricta **demo vs producto** en rutas: hoy documentada en comentario en `ShellRoutes.tsx`; no implica mover código de dominio.

## 22. Next Steps (orden sugerido)

1. Extender **useQuery** + `queryKeys` a más páginas (priorizar listados que repiten fetch al montar).
2. Revisar **a11y** por flujo (navegación teclado, modales, tablas).
3. Opcional: tests e2e Playwright en CI (el paquete tiene Playwright en config pero no en el run unitario).

---

## Referencias de archivos clave

- `frontend/src/main.tsx`
- `frontend/src/app/App.tsx`
- `frontend/src/styles.css`
- `frontend/src/styles/tokens.css`
- `frontend/src/components/CrudPage.tsx`
- `frontend/src/crud/lazyCrudPage.tsx`
- `frontend/src/lib/api.ts`
- `frontend/src/lib/theme.ts`
- `frontend/vite.config.ts`
- `frontend/package.json`
