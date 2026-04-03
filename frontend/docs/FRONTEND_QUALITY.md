# Frontend — Quality Checklist

Auditoría de calidad del frontend. Fecha de última actualización: 2026-04-02.

## Estado actual

| Área | Estado | Notas |
|---|---|---|
| Organización de código | Excelente | Separación clara pages → components → lib → crud |
| Capa API | Excelente | Centralizada, tipada, sin fetch sueltos |
| TypeScript | Bueno | strict: true, sin @ts-ignore, sin as any casts |
| Seguridad | Bueno | Sin dangerouslySetInnerHTML, sin eval, auth abstraído |
| Performance / code splitting | Bueno | Chunks separados por vertical, React Query |
| CSS / diseño | Bueno | Tokens + global CSS, sin CSS-in-JS |
| i18n | Excelente | Shell 100% i18n, es + en completos |
| Dependencias | Bueno | React 18, Vite 6, TS 5.7 |
| State management | Aceptable | React Query + Context, sin global store |
| Error handling | Aceptable | ErrorBoundary root + React Query onError |
| Testing | Bueno | 37 test files, 227 tests (~26% archivos, utilidades core 100%) |
| ESLint / hooks | Excelente | exhaustive-deps en warn, 0 warnings |
| Accesibilidad | Excelente | aria-hidden, aria-label, alt, htmlFor, role=dialog |

## Checklist de mantenimiento

### Accesibilidad
- [x] SVGs decorativos con `aria-hidden="true"`
- [x] Botones con icono tienen `aria-label`
- [x] Imágenes (avatars) con `alt` descriptivo
- [x] Inputs vinculados a labels via `id` + `htmlFor`
- [x] Modales con `role="dialog"` y `aria-labelledby`
- [x] Skip link en Shell
- [x] `aria-current="page"` en paginación activa

### i18n
- [x] Shell.tsx — todos los labels via `t()`
- [x] Bike shop nav — via `t()`
- [x] Skip link y search placeholder — via `t()`
- [x] Keys `common.error.*` para ErrorBoundary (disponibles, no usadas por ser class component)
- [x] OnboardingPage — 100% i18n (es + en, ~90 keys)
- [x] ErrorBoundary main.tsx — i18n via lectura directa de storage + commonMessages (sin contexto React)

### ESLint
- [x] `react-hooks/exhaustive-deps`: `warn` (no `off`)
- [x] 0 warnings activos
- [x] Suppressions documentadas con justificación (`// eslint-disable-next-line ... -- razón`)
- [x] `@typescript-eslint/no-explicit-any`: `warn`, 0 warnings (1 suppress justificado en CrudResourceConfigMap — TS no soporta existenciales)
- [x] `react-refresh/only-export-components`: `warn` con `allowConstantExport`, 0 warnings

### Testing
- [x] 37 test files, 227 tests pasando
- [x] Vitest + Testing Library configurados
- [x] Playwright con 15 E2E tests (onboarding, navigation, accessibility)
- [x] Cobertura ~26% (37/143 archivos), utilidades core al 100%
- [x] Páginas con tests: OnboardingPage, CalendarPage, DashboardVisualPage, NotificationsCenterPage, PublicPreviewPage

## Deuda técnica pendiente

### Alta prioridad
- [x] Subir test coverage — 37 test files / 227 tests (~26% por archivos, utilidades core 100%)
- [x] Escribir E2E con Playwright — 3 suites: onboarding (5 tests), navigation (6 tests), accessibility (4 tests)

### Media prioridad
- [x] Consolidar libs de drag-and-drop: `@hello-pangea/dnd` eliminado, solo `@dnd-kit`
- [x] `npm audit fix` — 0 vulnerabilidades (picomatch + esbuild/vitest resueltos)
- [x] Agregar `.prettierrc` para estandarizar formateo

### Baja prioridad
- [x] Habilitar `@typescript-eslint/no-explicit-any` como warn, 0 warnings
- [x] Extraer inline SVG icons de Shell a `ShellIcons.tsx` (16 iconos, helper `Icon` con props compartidos)
- [x] Agregar Sentry / error tracking para producción (`@sentry/react`, activado via `VITE_SENTRY_DSN`)
- [ ] Bundle analysis automatizado en CI
