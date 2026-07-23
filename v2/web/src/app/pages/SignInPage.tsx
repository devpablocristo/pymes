import { SignIn } from "@clerk/react";
import { Navigate } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";
import {
  AuthLoadError,
  AuthNotConfigured,
  FullPageState,
} from "../guards/AuthGuards";
import { LoadingState } from "../states/ContentStates";

export function SignInPage({ invitation = false }: { invitation?: boolean }) {
  const auth = useProductAuth();

  if (auth.status === "loading") {
    return (
      <FullPageState>
        <LoadingState label="Cargando acceso seguro" />
      </FullPageState>
    );
  }
  if (auth.status === "unconfigured") {
    return <AuthNotConfigured />;
  }
  if (auth.status === "error") {
    return <AuthLoadError />;
  }
  if (auth.status === "signed-in") {
    return (
      <Navigate
        replace
        to={auth.activeOrganizationId ? "/dashboard" : "/select-organization"}
      />
    );
  }

  return (
    <main className="auth-page">
      <section className="auth-page__identity" aria-label="Pymes">
        <img className="auth-page__wordmark" src="/assets/logo-dark.svg" alt="Pymes" />
        <div className="auth-page__message">
          <span>{invitation ? "Invitación de equipo" : "Acceso privado"}</span>
          <h1>
            {invitation
              ? "Tu lugar de trabajo ya está preparado."
              : "La gestión diaria empieza con una identidad segura."}
          </h1>
          <p>
            {invitation
              ? "Confirmá el email invitado para entrar a la organización."
              : "Ingresá con el código enviado a tu email. No hay registro público."}
          </p>
        </div>
        <img className="auth-page__watermark" src="/assets/iso.svg" alt="" />
      </section>
      <section className="auth-page__form" aria-label="Autenticación">
        <SignIn
          routing="path"
          path={invitation ? "/accept-invitation" : "/sign-in"}
          fallbackRedirectUrl="/select-organization"
          signUpUrl={invitation ? "/accept-invitation" : undefined}
        />
      </section>
    </main>
  );
}
