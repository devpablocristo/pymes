/**
 * Lazy imports de páginas — un solo lugar para code-splitting (App + ShellRoutes).
 */
import { lazy } from 'react';

export const Shell = lazy(() => import('../components/Shell').then((mod) => ({ default: mod.Shell })));
export const AutoRepairWorkOrdersPage = lazy(() =>
  import('../pages/AutoRepairWorkOrdersPage').then((mod) => ({ default: mod.AutoRepairWorkOrdersPage })),
);
export const BikeShopWorkOrdersPage = lazy(() =>
  import('../pages/BikeShopWorkOrdersPage').then((mod) => ({ default: mod.BikeShopWorkOrdersPage })),
);
export const BikeShopWorkOrdersBoard = lazy(() => import('../pages/BikeShopWorkOrdersBoard'));
export const BikeShopWorkOrdersSection = lazy(() => import('../pages/BikeShopWorkOrdersSection'));
export const WorkOrdersModuleSection = lazy(() =>
  import('../pages/WorkOrdersModuleSection').then((mod) => ({ default: mod.WorkOrdersModuleSection })),
);
export const WorkOrdersKanbanPanel = lazy(() =>
  import('../pages/WorkOrdersKanbanPanel').then((mod) => ({ default: mod.WorkOrdersKanbanPanel })),
);
export const WorkOrdersEditorPage = lazy(() =>
  import('../pages/WorkOrdersEditorPage').then((mod) => ({ default: mod.WorkOrdersEditorPage })),
);
export const ProductsModuleSection = lazy(() =>
  import('../pages/ProductsModuleSection').then((mod) => ({ default: mod.ProductsModuleSection })),
);
export const ProductsListPage = lazy(() =>
  import('../pages/ProductsListPage').then((mod) => ({ default: mod.ProductsListPage })),
);
export const ProductsGalleryPage = lazy(() =>
  import('../pages/ProductsGalleryPage').then((mod) => ({ default: mod.ProductsGalleryPage })),
);
export const UnifiedChatPage = lazy(() =>
  import('../pages/UnifiedChatPage').then((mod) => ({ default: mod.UnifiedChatPage })),
);
export const NotificationsCenterPage = lazy(() =>
  import('../pages/NotificationsCenterPage').then((mod) => ({ default: mod.NotificationsCenterPage })),
);
export const LoginPage = lazy(() => import('../pages/LoginPage').then((mod) => ({ default: mod.LoginPage })));
export const ModulePage = lazy(() => import('../pages/ModulePage').then((mod) => ({ default: mod.ModulePage })));
export const OnboardingPage = lazy(() =>
  import('../pages/OnboardingPage').then((mod) => ({ default: mod.OnboardingPage })),
);
export const RestaurantTableSessionsPage = lazy(() =>
  import('../pages/RestaurantTableSessionsPage').then((mod) => ({ default: mod.RestaurantTableSessionsPage })),
);
export const SignupPage = lazy(() => import('../pages/SignupPage').then((mod) => ({ default: mod.SignupPage })));
export const AutomationRulesPage = lazy(() => import('../pages/AutomationRulesPage'));
export const WhatsAppCampaignsPage = lazy(() =>
  import('../pages/WhatsAppCampaignsPage').then((mod) => ({ default: mod.WhatsAppCampaignsPage })),
);
export const WhatsAppInboxPage = lazy(() =>
  import('../pages/WhatsAppInboxPage').then((mod) => ({ default: mod.WhatsAppInboxPage })),
);
export const WatcherConfigPage = lazy(() => import('../pages/WatcherConfigPage'));
export const CalendarPage = lazy(() => import('../pages/CalendarPage').then((mod) => ({ default: mod.CalendarPage })));
export const StockPage = lazy(() => import('../pages/StockPage').then((mod) => ({ default: mod.StockPage })));
export const DashboardVisualPage = lazy(() =>
  import('../pages/DashboardVisualPage').then((mod) => ({ default: mod.DashboardVisualPage })),
);
export const InvoicesPage = lazy(() => import('../pages/InvoicesPage').then((mod) => ({ default: mod.InvoicesPage })));
export const SettingsHubPage = lazy(() =>
  import('../pages/SettingsHubPage').then((mod) => ({ default: mod.SettingsHubPage })),
);
