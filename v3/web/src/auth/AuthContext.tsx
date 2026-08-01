import {
  ClerkProvider,
  OrganizationSwitcher,
  Show,
  SignIn,
  UserButton,
  useAuth,
  useOrganization,
} from "@clerk/react";
import { useQuery } from "@tanstack/react-query";
import {
  lazy,
  type ReactNode,
  useCallback,
  useMemo,
  Suspense,
} from "react";
import { getCurrentSession } from "../api/currentSession";
import type { WebConfig } from "../config";
import {
  SessionContext,
  type Session,
  useSession,
} from "./sessionContext";

export { useSession };

const LocalSession = lazy(async () => {
  if (!import.meta.env.DEV && import.meta.env.MODE !== "e2e") {
    throw new Error("La sesión local no forma parte del runtime productivo.");
  }
  return import("./LocalSession");
});

function ClerkSession({
  config,
  children,
}: {
  config: WebConfig;
  children: ReactNode;
}) {
  const { getToken } = useAuth();
  const { organization } = useOrganization();
  const clerkOrganizationId = organization?.id;
  const currentSession = useQuery({
    queryKey: ["current-session", clerkOrganizationId],
    queryFn: () =>
      getCurrentSession(config.apiBaseUrl, () =>
        getToken({ skipCache: true }),
      ),
    enabled: Boolean(clerkOrganizationId),
    retry: false,
  });
  const schedulingToken = useCallback(() => getToken(), [getToken]);
  const session = useMemo<Session | null>(() => {
    if (!currentSession.data) {
      return null;
    }
    return {
      identity: {
        organizationId: currentSession.data.organization.id,
        getToken: schedulingToken,
      },
      organizationName: currentSession.data.organization.name,
      organizationSlug: currentSession.data.organization.slug,
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
  }, [currentSession.data, schedulingToken]);

  if (!clerkOrganizationId) {
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
  if (currentSession.isPending) {
    return (
      <main className="auth-stage" id="main-content">
        <div className="auth-card">
          <p className="eyebrow">Sesión segura</p>
          <h1>Abriendo tu organización</h1>
          <p>Estamos comprobando la membresía local de Pymes.</p>
        </div>
      </main>
    );
  }
  if (currentSession.isError || !session) {
    return (
      <main className="auth-stage" id="main-content">
        <div className="auth-card auth-card--error">
          <p className="eyebrow">Acceso no disponible</p>
          <h1>No pudimos abrir esta organización</h1>
          <p>
            {currentSession.error instanceof Error
              ? currentSession.error.message
              : "Revisá que la organización esté sincronizada con Pymes."}
          </p>
          <OrganizationSwitcher
            hidePersonal
            afterCreateOrganizationUrl="/app/agenda"
            afterSelectOrganizationUrl="/app/agenda"
          />
        </div>
      </main>
    );
  }
  if (currentSession.data.organization.status !== "ready") {
    return (
      <main className="auth-stage" id="main-content">
        <div className="auth-card">
          <p className="eyebrow">Organización en preparación</p>
          <h1>{currentSession.data.organization.name}</h1>
          <p>
            El estado actual es {currentSession.data.organization.status}. La
            agenda se habilitará cuando termine el provisionamiento.
          </p>
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
          <ClerkSession config={config}>{children}</ClerkSession>
        </Show>
      </ClerkProvider>
    );
  }
  if (config.allowInsecureLocalAuth) {
    return (
      <Suspense fallback={<main className="public-loading">Iniciando sesión local…</main>}>
        <LocalSession config={config}>{children}</LocalSession>
      </Suspense>
    );
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
