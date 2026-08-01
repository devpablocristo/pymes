import { AdminAuthBoundary } from "../auth/AuthContext";
import type { WebConfig } from "../config";
import { AdminSchedulingPage } from "../pages/AdminSchedulingPage";

export default function AdminRoute({ config }: { config: WebConfig }) {
  return (
    <AdminAuthBoundary config={config}>
      <AdminSchedulingPage publicSlug={config.publicOrganizationSlug} />
    </AdminAuthBoundary>
  );
}
