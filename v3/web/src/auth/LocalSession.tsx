import type { ReactNode } from "react";
import type { WebConfig } from "../config";
import { SessionContext, type Session } from "./sessionContext";

export default function LocalSession({
  config,
  children,
}: {
  config: WebConfig;
  children: ReactNode;
}) {
  if (!config.localOrganizationId) {
    return (
      <main className="auth-stage" id="main-content">
        <div className="auth-card auth-card--error">
          <p className="eyebrow">Configuración local incompleta</p>
          <h1>Falta una organización de desarrollo</h1>
          <p>Definí VITE_PYMES_ORGANIZATION_ID para iniciar el entorno local.</p>
        </div>
      </main>
    );
  }
  const session: Session = {
    identity: {
      organizationId: config.localOrganizationId,
      getToken: async () => "local-e2e-token",
    },
    organizationName: "Centro Norte",
    organizationSlug: config.publicOrganizationSlug,
    accountControls: <span className="local-session-badge">Sesión local</span>,
    local: true,
  };
  return <SessionContext.Provider value={session}>{children}</SessionContext.Provider>;
}
