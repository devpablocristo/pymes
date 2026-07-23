import { useEffect, useRef, useState } from "react";
import { Navigate } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";

export function SelectOrganizationPage() {
  const auth = useProductAuth();
  const autoSelected = useRef(false);
  const [pending, setPending] = useState<string>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (
      !auth.activeOrganizationId &&
      auth.organizations.length === 1 &&
      !autoSelected.current
    ) {
      autoSelected.current = true;
      const organizationId = auth.organizations[0].id;
      setPending(organizationId);
      auth
        .setActiveOrganization(organizationId)
        .catch(() => setError("No pudimos activar la organización. Intentá nuevamente."))
        .finally(() => setPending(undefined));
    }
  }, [auth]);

  if (auth.activeOrganizationId) {
    return <Navigate replace to="/dashboard" />;
  }
  if (auth.organizations.length === 0) {
    return <Navigate replace to="/no-access" />;
  }

  return (
    <main className="organization-picker">
      <header>
        <span className="organization-picker__wordmark">
          <img className="organization-picker__logo-light" src="/assets/logo.svg" alt="Pymes" />
          <img className="organization-picker__logo-dark" src="/assets/logo-dark.svg" alt="Pymes" />
        </span>
        <span>Organización activa</span>
        <h1>¿Dónde vas a trabajar?</h1>
        <p>La organización elegida quedará incluida en el próximo token de sesión.</p>
      </header>
      {error ? <p className="form-error" role="alert">{error}</p> : null}
      <div className="organization-picker__list">
        {auth.organizations.map((organization) => (
          <button
            key={organization.id}
            type="button"
            disabled={Boolean(pending)}
            aria-busy={pending === organization.id}
            onClick={async () => {
              setError(undefined);
              setPending(organization.id);
              try {
                await auth.setActiveOrganization(organization.id);
              } catch {
                setError("No pudimos activar la organización. Intentá nuevamente.");
              } finally {
                setPending(undefined);
              }
            }}
          >
            <span className="organization-picker__monogram" aria-hidden="true">
              {organization.name.slice(0, 2).toUpperCase()}
            </span>
            <span>
              <strong>{organization.name}</strong>
              <small>{organization.slug || "Organización Pymes"}</small>
            </span>
            <span aria-hidden="true">{pending === organization.id ? "…" : "→"}</span>
          </button>
        ))}
      </div>
    </main>
  );
}
