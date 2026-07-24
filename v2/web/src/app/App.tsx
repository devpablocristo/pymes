import { Navigate, Route, Routes } from "react-router-dom";
import { ActiveOrganizationGuard, GlobalOwnerGuard, SignedInGuard } from "./guards/AuthGuards";
import { AccessManagementPage } from "./pages/AccessManagementPage";
import { DashboardPage } from "./pages/DashboardPage";
import { EmployeesPage } from "./pages/EmployeesPage";
import { NoAccessPage } from "./pages/NoAccessPage";
import { NotFoundPage } from "./pages/NotFoundPage";
import { OrganizationSettingsPage } from "./pages/OrganizationSettingsPage";
import { SelectOrganizationPage } from "./pages/SelectOrganizationPage";
import { SessionsPage } from "./pages/SessionsPage";
import { SignInPage } from "./pages/SignInPage";
import { TeamPage } from "./pages/TeamPage";
import { ProductShell } from "./shell/ProductShell";

export function App() {
  return (
    <Routes>
      <Route path="/sign-in/*" element={<SignInPage />} />
      <Route path="/accept-invitation/*" element={<SignInPage invitation />} />
      <Route element={<SignedInGuard />}>
        <Route path="/select-organization" element={<SelectOrganizationPage />} />
        <Route path="/no-access" element={<NoAccessPage />} />
        <Route element={<ProductShell />}>
          <Route path="/settings/sessions" element={<SessionsPage />} />
          <Route element={<GlobalOwnerGuard />}>
            <Route path="/admin/users" element={<AccessManagementPage />} />
            <Route path="/admin/tenants" element={<Navigate replace to="/admin/users" />} />
            <Route path="/admin/invitations" element={<Navigate replace to="/admin/users" />} />
          </Route>
          <Route element={<ActiveOrganizationGuard />}>
            <Route index element={<Navigate replace to="/dashboard" />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/employees" element={<EmployeesPage />} />
            <Route path="/settings/organization" element={<OrganizationSettingsPage />} />
            <Route path="/settings/team" element={<TeamPage />} />
          </Route>
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate replace to="/sign-in" />} />
    </Routes>
  );
}
