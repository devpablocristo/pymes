/**
 * Lazy imports de páginas — un solo lugar para code-splitting (App + ShellRoutes).
 */
import { lazy } from 'react';

export const Shell = lazy(() => import('../components/Shell').then((mod) => ({ default: mod.Shell })));
export const AdminPage = lazy(() => import('../pages/AdminPage').then((mod) => ({ default: mod.AdminPage })));
export const AutoRepairServicesPage = lazy(() =>
  import('../pages/AutoRepairServicesPage').then((mod) => ({ default: mod.AutoRepairServicesPage })),
);
export const AutoRepairVehiclesPage = lazy(() =>
  import('../pages/AutoRepairVehiclesPage').then((mod) => ({ default: mod.AutoRepairVehiclesPage })),
);
export const AutoRepairWorkOrdersPage = lazy(() =>
  import('../pages/AutoRepairWorkOrdersPage').then((mod) => ({ default: mod.AutoRepairWorkOrdersPage })),
);
export const BikeShopBicyclesPage = lazy(() =>
  import('../pages/BikeShopBicyclesPage').then((mod) => ({ default: mod.BikeShopBicyclesPage })),
);
export const BikeShopServicesPage = lazy(() =>
  import('../pages/BikeShopServicesPage').then((mod) => ({ default: mod.BikeShopServicesPage })),
);
export const BikeShopWorkOrdersPage = lazy(() =>
  import('../pages/BikeShopWorkOrdersPage').then((mod) => ({ default: mod.BikeShopWorkOrdersPage })),
);
export const WorkOrdersModuleSection = lazy(() =>
  import('../pages/WorkOrdersModuleSection').then((mod) => ({ default: mod.WorkOrdersModuleSection })),
);
export const WorkOrdersKanbanPanel = lazy(() =>
  import('../pages/WorkOrdersKanbanPanel').then((mod) => ({ default: mod.WorkOrdersKanbanPanel })),
);
export const WorkOrdersEditorPage = lazy(() =>
  import('../pages/WorkOrdersEditorPage').then((mod) => ({ default: mod.WorkOrdersEditorPage })),
);
export const BeautySalonServicesPage = lazy(() =>
  import('../pages/BeautySalonServicesPage').then((mod) => ({ default: mod.BeautySalonServicesPage })),
);
export const BeautyStaffPage = lazy(() => import('../pages/BeautyStaffPage').then((mod) => ({ default: mod.BeautyStaffPage })));
export const UnifiedChatPage = lazy(() =>
  import('../pages/UnifiedChatPage').then((mod) => ({ default: mod.UnifiedChatPage })),
);
export const NotificationsCenterPage = lazy(() =>
  import('../pages/NotificationsCenterPage').then((mod) => ({ default: mod.NotificationsCenterPage })),
);
export const CustomersPage = lazy(() => import('../pages/CustomersPage').then((mod) => ({ default: mod.CustomersPage })));
export const IntakesPage = lazy(() => import('../pages/IntakesPage').then((mod) => ({ default: mod.IntakesPage })));
export const LoginPage = lazy(() => import('../pages/LoginPage').then((mod) => ({ default: mod.LoginPage })));
export const ModulePage = lazy(() => import('../pages/ModulePage').then((mod) => ({ default: mod.ModulePage })));
export const OnboardingPage = lazy(() => import('../pages/OnboardingPage').then((mod) => ({ default: mod.OnboardingPage })));
export const PublicPreviewPage = lazy(() =>
  import('../pages/PublicPreviewPage').then((mod) => ({ default: mod.PublicPreviewPage })),
);
export const PurchasesPage = lazy(() => import('../pages/PurchasesPage').then((mod) => ({ default: mod.PurchasesPage })));
export const RestaurantDiningAreasPage = lazy(() =>
  import('../pages/RestaurantDiningAreasPage').then((mod) => ({ default: mod.RestaurantDiningAreasPage })),
);
export const RestaurantDiningTablesPage = lazy(() =>
  import('../pages/RestaurantDiningTablesPage').then((mod) => ({ default: mod.RestaurantDiningTablesPage })),
);
export const RestaurantTableSessionsPage = lazy(() =>
  import('../pages/RestaurantTableSessionsPage').then((mod) => ({ default: mod.RestaurantTableSessionsPage })),
);
export const SessionsPage = lazy(() => import('../pages/SessionsPage').then((mod) => ({ default: mod.SessionsPage })));
export const SignupPage = lazy(() => import('../pages/SignupPage').then((mod) => ({ default: mod.SignupPage })));
export const SpecialtiesPage = lazy(() => import('../pages/SpecialtiesPage').then((mod) => ({ default: mod.SpecialtiesPage })));
export const TeachersPage = lazy(() => import('../pages/TeachersPage').then((mod) => ({ default: mod.TeachersPage })));
export const AutomationRulesPage = lazy(() => import('../pages/AutomationRulesPage'));
export const WhatsAppCampaignsPage = lazy(() =>
  import('../pages/WhatsAppCampaignsPage').then((mod) => ({ default: mod.WhatsAppCampaignsPage })),
);
export const WhatsAppInboxPage = lazy(() =>
  import('../pages/WhatsAppInboxPage').then((mod) => ({ default: mod.WhatsAppInboxPage })),
);
export const WatcherConfigPage = lazy(() => import('../pages/WatcherConfigPage'));
export const CalendarPage = lazy(() => import('../pages/CalendarPage').then((mod) => ({ default: mod.CalendarPage })));
export const DashboardVisualPage = lazy(() =>
  import('../pages/DashboardVisualPage').then((mod) => ({ default: mod.DashboardVisualPage })),
);
export const DashboardPage = lazy(() => import('../pages/DashboardPage').then((mod) => ({ default: mod.DashboardPage })));
export const InvoicesPage = lazy(() => import('../pages/InvoicesPage').then((mod) => ({ default: mod.InvoicesPage })));
export const SettingsHubPage = lazy(() =>
  import('../pages/SettingsHubPage').then((mod) => ({ default: mod.SettingsHubPage })),
);
export const UIComponentsPage = lazy(() =>
  import('../pages/UIComponentsPage').then((mod) => ({ default: mod.UIComponentsPage })),
);
export const CryptoPage = lazy(() => import('../pages/CryptoPage').then((m) => ({ default: m.CryptoPage })));
