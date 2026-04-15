/**
 * Lazy imports de páginas — un solo lugar para code-splitting (App + ShellRoutes).
 */
import { lazy } from 'react';

export const Shell = lazy(() => import('../components/Shell').then((mod) => ({ default: mod.Shell })));
export const ConfiguredCrudSectionPage = lazy(() =>
  import('../pages/ConfiguredCrudSectionPage').then((mod) => ({ default: mod.ConfiguredCrudSectionPage })),
);
export const WorkOrdersEditorPage = lazy(() =>
  import('../pages/WorkOrdersEditorPage').then((mod) => ({ default: mod.WorkOrdersEditorPage })),
);
export const ConfiguredCrudIndexRedirect = lazy(() =>
  import('../crud/configuredCrudViews').then((mod) => ({ default: mod.ConfiguredCrudIndexRedirect })),
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
export const ConfiguredCrudRouteModePage = lazy(() =>
  import('../crud/configuredCrudViews').then((mod) => ({ default: mod.ConfiguredCrudRouteModePage })),
);
export const ConfiguredCrudNestedRouteModePage = lazy(() =>
  import('../crud/configuredCrudViews').then((mod) => ({ default: mod.ConfiguredCrudNestedRouteModePage })),
);
export const CrudUiConfigurePage = lazy(() =>
  import('../pages/CrudUiConfigurePage').then((mod) => ({ default: mod.CrudUiConfigurePage })),
);
export const DashboardVisualPage = lazy(() =>
  import('../pages/DashboardVisualPage').then((mod) => ({ default: mod.DashboardVisualPage })),
);
export const SettingsHubPage = lazy(() =>
  import('../pages/SettingsHubPage').then((mod) => ({ default: mod.SettingsHubPage })),
);
