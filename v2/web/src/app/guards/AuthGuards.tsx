import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";
import { FatalErrorState, LoadingState } from "../states/ContentStates";

export function SignedInGuard() {
  const auth = useProductAuth();
  const location = useLocation();

  if (auth.status === "loading") {
    return <FullPageState><LoadingState label="Cargando acceso seguro" /></FullPageState>;
  }
  if (auth.status === "unconfigured") {
    return <AuthNotConfigured />;
  }
  if (auth.status === "error") {
    return <AuthLoadError code={auth.errorCode} />;
  }
  if (auth.status === "signed-out") {
    return <Navigate replace to="/sign-in" state={{ from: location.pathname }} />;
  }
  return <Outlet />;
}

export function ActiveOrganizationGuard() {
  const auth = useProductAuth();
  if (!auth.activeOrganizationId) {
    return <Navigate replace to="/select-organization" />;
  }
  return <Outlet />;
}

export function AuthLoadError({
  code,
}: {
  code?: import("../../auth/AuthContext").AuthErrorCode;
}) {
  const body =
    code === "AUTH_SESSION_TOKEN_UNAVAILABLE"
      ? "Clerk no pudo emitir una sesión válida. Volvé a iniciar sesión."
      : code === "AUTH_SESSION_REJECTED"
        ? "La sesión fue rechazada por la API. Volvé a iniciar sesión."
        : code === "AUTH_DIRECTORY_UNAVAILABLE"
          ? "No pudimos cargar tus organizaciones. Intentá nuevamente."
          : "Revisá que la API esté disponible e intentá nuevamente.";
  return (
    <FatalErrorState
      title="Pymes no pudo cargar la autenticación"
      body={body}
      reloadLabel="Intentar de nuevo"
    />
  );
}

export function AuthNotConfigured() {
  return (
    <main className="auth-system-state">
      <img src="/assets/iso.svg" alt="" />
      <p className="auth-system-state__code">AUTH_NOT_CONFIGURED</p>
      <h1>Falta conectar Clerk</h1>
      <p>
        El stack está operativo, pero el acceso permanece bloqueado hasta configurar
        las claves de la instancia Pymes v2.
      </p>
      <code>PYMES_CLERK_PUBLISHABLE_KEY · PYMES_CLERK_SECRET_KEY</code>
    </main>
  );
}

export function FullPageState({ children }: { children: React.ReactNode }) {
  return <main className="auth-system-state">{children}</main>;
}
