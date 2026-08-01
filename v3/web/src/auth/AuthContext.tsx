import {
  ClerkProvider,
  OrganizationSwitcher,
  Show,
  SignIn,
  UserButton,
  useAuth,
  useOrganization,
} from "@clerk/react";
import { createContext, type ReactNode, useContext, useMemo } from "react";
import type { RequestIdentity } from "../api/SchedulingGateway";
import type { WebConfig } from "../config";

type Session = {
  identity: RequestIdentity;
  organizationName: string;
  accountControls: ReactNode;
  local: boolean;
};

const SessionContext = createContext<Session | null>(null);

export function useSession(): Session {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession debe utilizarse dentro de AdminAuthBoundary");
  }
  return value;
}

function ClerkSession({ children }: { children: ReactNode }) {
  const { getToken } = useAuth();
  const { organization } = useOrganization();
  const organizationId = organization?.id;
  const session = useMemo<Session | null>(() => {
    if (!organizationId) {
      return null;
    }
    return {
      identity: { organizationId, getToken },
      organizationName: organization?.name ?? "Mi organización",
      accountControls: (
        <div className="account-controls">
          <OrganizationSwitcher
            hidePersonal
            afterCreateOrganizationUrl="/app/agenda"
            afterSelectOrganizationUrl="/app/agenda"
          />
          <UserButton />
        </div>
      ),
      local: false,
    };
  }, [getToken, organization?.name, organizationId]);

  if (!session) {
    return (
      <main className="auth-stage" id="main-content">
        <div className="auth-card">
          <p className="eyebrow">Organización requerida</p>
          <h1>Elegí dónde vas a trabajar</h1>
          <p>La agenda siempre opera dentro de una organización activa.</p>
          <OrganizationSwitcher
            hidePersonal
            afterCreateOrganizationUrl="/app/agenda"
            afterSelectOrganizationUrl="/app/agenda"
          />
        </div>
      </main>
    );
  }

  return <SessionContext.Provider value={session}>{children}</SessionContext.Provider>;
}

function LocalSession({ config, children }: { config: WebConfig; children: ReactNode }) {
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
    accountControls: <span className="local-session-badge">Sesión local</span>,
    local: true,
  };
  return <SessionContext.Provider value={session}>{children}</SessionContext.Provider>;
}

export function AdminAuthBoundary({ config, children }: { config: WebConfig; children: ReactNode }) {
  if (config.clerkPublishableKey) {
    return (
      <ClerkProvider publishableKey={config.clerkPublishableKey}>
        <Show when="signed-out">
          <main className="auth-stage" id="main-content">
            <div className="auth-card auth-card--signin">
              <p className="eyebrow">Pymes · Agenda</p>
              <SignIn routing="hash" />
            </div>
          </main>
        </Show>
        <Show when="signed-in">
          <ClerkSession>{children}</ClerkSession>
        </Show>
      </ClerkProvider>
    );
  }
  if (config.allowInsecureLocalAuth) {
    return <LocalSession config={config}>{children}</LocalSession>;
  }
  return (
    <main className="auth-stage" id="main-content">
      <div className="auth-card auth-card--error">
        <p className="eyebrow">Configuración requerida</p>
        <h1>Clerk todavía no está conectado</h1>
        <p>Definí VITE_CLERK_PUBLISHABLE_KEY para abrir la aplicación.</p>
      </div>
    </main>
  );
}
