# V2 interna

## Camino canonico

- CRUD real en frontend: `frontend/src/components/CrudPage.tsx` + `frontend/src/crud/*` + `frontend/src/crud/crudModuleCatalog.ts`
- Shell autenticado y rutas: `frontend/src/app/App.tsx` + `frontend/src/app/ShellRoutes.tsx`
- Explorer / superficies no CRUD: `frontend/src/pages/ModulePage.tsx` + `frontend/src/lib/moduleCatalog.ts`
- Backend owner por bounded context: `docs/ARCHITECTURE.md`

## Desvios detectados

- `frontend/src/app/ShellRoutes.tsx` todavia expone varias rutas manuales que solo abren wrappers CRUD.
- `frontend/src/app/lazyRoutes.tsx` importa varias paginas que no agregan logica y solo retornan `LazyConfiguredCrudPage`.
- Hay mezcla de rutas canonicas `/modules/:moduleId` con rutas legacy o verticales especificas para el mismo recurso.
- El shell ya esta centralizado, pero no termino de converger al catalogo modular.

## Wrappers CRUD candidatos a eliminar

- `frontend/src/pages/CustomersPage.tsx`
- `frontend/src/pages/PurchasesPage.tsx`
- `frontend/src/pages/TeachersPage.tsx`
- `frontend/src/pages/SpecialtiesPage.tsx`
- `frontend/src/pages/IntakesPage.tsx`
- `frontend/src/pages/SessionsPage.tsx`
- `frontend/src/pages/AutoRepairVehiclesPage.tsx`
- `frontend/src/pages/AutoRepairServicesPage.tsx`
- `frontend/src/pages/BikeShopBicyclesPage.tsx`
- `frontend/src/pages/BikeShopServicesPage.tsx`
- `frontend/src/pages/BeautyStaffPage.tsx`
- `frontend/src/pages/BeautySalonServicesPage.tsx`
- `frontend/src/pages/RestaurantDiningAreasPage.tsx`
- `frontend/src/pages/RestaurantDiningTablesPage.tsx`

## Flujos que siguen bespoke por ahora

| Flujo | Motivo | Owner frontend |
|-------|--------|----------------|
| onboarding | wizard multi-step + sync con org/auth + persistencia de tenant profile | `frontend/src/pages/OnboardingPage.tsx` |
| scheduling / calendar | calendario, drag and drop, vistas operativas y board de cola | `frontend/src/pages/CalendarPage.tsx` |
| work orders | mezcla de board, editor, lista y transiciones operativas; ya converge sobre `frontend/src/components/GenericWorkOrdersBoard.tsx` para reducir duplicación entre verticales | `frontend/src/pages/WorkOrdersKanbanPanel.tsx` / `frontend/src/pages/WorkOrdersEditorPage.tsx` |
| dashboard | superficie analitica fija, no CRUD puro | `frontend/src/pages/DashboardVisualPage.tsx` |
| chat | estado conversacional, routing AI, handoff y bloques enriquecidos | `frontend/src/pages/UnifiedChatPage.tsx` |
| settings avanzados | hubs y formularios compuestos, no un recurso CRUD unico | `frontend/src/pages/SettingsHubPage.tsx` |
| admin / RBAC | secciones compuestas y vistas de control | `frontend/src/pages/AdminPage.tsx` |
| restaurant table sessions | flujo operativo de apertura/cierre de mesa, no CRUD base | `frontend/src/pages/RestaurantTableSessionsPage.tsx` |

## Regla operativa

- Si una pagina solo hace `return <LazyConfiguredCrudPage resourceId=\"...\" />`, se migra a `/modules/:moduleId`.
- Si un flujo necesita nesting, tabs, wizard, realtime, drag and drop o contratos compuestos, queda bespoke hasta definir su frontera.
- Cada convergencia debe borrar wrappers, imports lazy y redirects obsoletos en el mismo slice.
