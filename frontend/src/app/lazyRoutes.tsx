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
export const ConfiguredCrudIndexRedirect = lazy(() =>
  import('../crud/configuredCrudViews').then((mod) => ({ default: mod.ConfiguredCrudIndexRedirect })),
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
export const CustomerMessagingCampaignsPage = lazy(() =>
  import('../pages/CustomerMessagingCampaignsPage').then((mod) => ({ default: mod.CustomerMessagingCampaignsPage })),
);
export const CustomerMessagingInboxPage = lazy(() =>
  import('../pages/CustomerMessagingInboxPage').then((mod) => ({ default: mod.CustomerMessagingInboxPage })),
);
export const WatcherConfigPage = lazy(() => import('../pages/WatcherConfigPage'));
export const CalendarPage = lazy(() => import('../pages/CalendarPage').then((mod) => ({ default: mod.CalendarPage })));
export const ConfiguredCrudModePage = lazy(() =>
  import('../crud/configuredCrudViews').then((mod) => ({ default: mod.ConfiguredCrudModePage })),
);
export const StockModuleSection = lazy(() =>
  import('../pages/StockModuleSection').then((mod) => ({ default: mod.StockModuleSection })),
);
export const StockListPage = lazy(() => import('../pages/StockListPage').then((mod) => ({ default: mod.StockListPage })));
export const StockCrudUiConfigurePage = lazy(() =>
  import('../pages/StockCrudUiConfigurePage').then((mod) => ({ default: mod.StockCrudUiConfigurePage })),
);
export const DashboardVisualPage = lazy(() =>
  import('../pages/DashboardVisualPage').then((mod) => ({ default: mod.DashboardVisualPage })),
);
export const InvoicesPage = lazy(() => import('../pages/InvoicesPage').then((mod) => ({ default: mod.InvoicesPage })));
export const SettingsHubPage = lazy(() =>
  import('../pages/SettingsHubPage').then((mod) => ({ default: mod.SettingsHubPage })),
);
